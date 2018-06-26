package main

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var requestImg chan RequestOptions
var noticeChan chan os.Signal
var resultMap map[string]*bytes.Buffer
var lockForresultMap *sync.RWMutex
var errMap map[string]bool
var lockForerrMap *sync.RWMutex
var work2id int32 = 0

var task map[string]int32
var taskResult map[string]TaskResult
var tasklock sync.RWMutex

var diffsource map[string]DiffMap
var diffmaplock sync.RWMutex
var CachedNotice chan int

type TaskResult struct {
	RawUri string
	Buf    *bytes.Buffer
	Type0  string
}

type DiffMap struct {
	MD5  string
	Path string
}

type RequestOptions struct {
	ImgUri         string
	Type0          string
	Notice         chan int32
	NotCache       int32
	NeedRespHeader int32
	ISSSL          int32
}

const (
	TLOG     = 1
	TSUCCESS = 2
	TCACHED  = 3
	TFAIL    = 100
)

func init() {
	requestImg = make(chan RequestOptions, 20)
	noticeChan = make(chan os.Signal, 1)
	resultMap = make(map[string]*bytes.Buffer)
	errMap = make(map[string]bool)
	lockForresultMap = &sync.RWMutex{}
	lockForerrMap = &sync.RWMutex{}

	// task
	task = make(map[string]int32)
	taskResult = make(map[string]TaskResult)

	diffsource = make(map[string]DiffMap)
	CachedNotice = make(chan int, 20)
}

func run_2() {
	for i := 0; i < 20; i++ {
		go work_2()
	}
}

func work_2() {
	workid := atomic.AddInt32(&work2id, 1)
	for {
		itidle(FlagWorkType2, workid)
		imgUrlOptions := <-requestImg
		itbusy(FlagWorkType2, workid)
		imgUrl := imgUrlOptions.ImgUri

		// use cached
		numtag := getNumTag(imgUrlOptions.ImgUri)
		if numtag != "" && imgUrlOptions.Type0 != "" {
			diffmaplock.RLock()
			if _, ok := diffsource[imgUrlOptions.Type0+"_"+numtag]; ok {
				diffmaplock.RUnlock()
				imgUrlOptions.Notice <- SUCCESSFULCACHED
				continue
			}
			diffmaplock.RUnlock()
		}
		md5lock.Lock()
		md5s := getMD5(imgUrl)
		md5lock.Unlock()
		tasklock.Lock()
		if stat, ok := task[md5s]; ok {
			if stat == TLOG {
				tasklock.Unlock()
				// imgUrlOptions.Notice<-Wait
				continue
			}
			if stat == TSUCCESS || stat == TCACHED {
				tasklock.Unlock()
				imgUrlOptions.Notice <- SUCCESSFUL
				continue
			}
			// task[md5s] = TLOG
			// tasklock.Unlock()
		}
		// else{
		task[md5s] = TLOG
		tasklock.Unlock()
		// }
		count_retry := 0
		useProxy := 0
	retry:
		count_retry++
		url, _ := url.Parse(imgUrl)
		ips := getAddr(imgUrl)
		if ips == nil || len(ips) < 1 {
			continue
		}
		ip := ips[0] + ":" + getHttpOrHttpsPort(url)
		watch := time.Now()
		for {
			var cp *ConnProxy
			// useProxy++
			if useProxy < 1 {
				cp, _ = getConn(ip)
			} else {
				cp, _ = getProxy(ip, watch.Unix())
			}
			if cp != nil {
				// logger.Println("push ok: " + ip)
				var err error
				k := false
				if imgUrlOptions.ISSSL == 0 {

					err, k = fetchImg(cp, imgUrlOptions)
					if err != nil {
						ConnPool.mu.Lock()
						cp.EndTime = time.Now()
						if _, ok := ConnPool.conn[cp.Conn]; ok {
							ConnPool.conn[cp.Conn].IsInvalid = true
						}
						ConnPool.mu.Unlock()
						ConnPool.recycleChan <- 1
						logger.Printf("download {%s} ...err:{%s}\n", imgUrl, err.Error())
						if strings.HasSuffix(err.Error(), "EOF") {
							nop()
							nop()
							goto retry
						}
						if !cp.IsProxy {
							useProxy++
							goto retry
						}
						tasklock.Lock()
						task[md5s] = TFAIL
						tasklock.Unlock()
						imgUrlOptions.Notice <- FAILED
						break
					}
				} else {
					err = fetchImgWithHttpsProxy(cp, imgUrlOptions)
					nop()
					nop()
				}

				ConnPool.mu.Lock()
				ConnPool.conn[cp.Conn].IsBusy = false
				ConnPool.mu.Unlock()
				if !k {
					ConnPool.mu.Lock()
					cp.EndTime = time.Now()
					logger.Println(cp.Conn.RemoteAddr(), !k)
					if _, ok := ConnPool.conn[cp.Conn]; ok {
						ConnPool.conn[cp.Conn].IsInvalid = true
					}
					ConnPool.mu.Unlock()
					ConnPool.recycleChan <- 1
				}
				break
			}
			if time.Now().Sub(watch.Add(time.Second*5)) > 0 {
				tasklock.Lock()
				task[md5s] = TFAIL
				tasklock.Unlock()
				imgUrlOptions.Notice <- FAILED
				break
			}
			time.Sleep(time.Second * 1)
		}
	}
}

func fetchImg(cp *ConnProxy, options RequestOptions) (error, bool) {
	k := false
	defer func() {
		//signal.Notify(c, os.Interrupt)
		// noticeChan <- 1
	}()
	req, err := http.NewRequest("GET", options.ImgUri, nil)
	if err != nil {
		// logger.Printf(err.Error())
		return err, k
	}
	setHeader(req)
	if err := req.Write(cp.Conn); err != nil {
		// logger.Printf(err.Error())
		return err, k
	}
	bufreader := bufio.NewReader(cp.Conn)
	resp, err := http.ReadResponse(bufreader, req)
	if err != nil {
		return err, k
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err, k
	}
	if len(b) == 28 {
		Montior.Lock.Lock()
		Montior.serverNot++
		Montior.Lock.Unlock()
		options.Notice <- 9
		return nil, false
	}

	Montior.Lock.Lock()
	Montior.ok++
	Montior.Lock.Unlock()

	// 会不会造成栈溢出?
	var temp bytes.Buffer
	if options.NeedRespHeader == 1 {
		nop()
		nop()
		temp.Write([]byte("HTTP/1.1 200 ok\r\n"))
		headerbtyes := getBytes(resp.Header)
		temp.Write(headerbtyes)
		// var rw bufio.ReadWriter
		// resp.Write(&rw)
		// temp.ReadFrom(&rw)
		logger.Println(string(temp.Bytes()))
		nop()
		nop()
	}
	temp.Write(b)
	md5 := getMD5(options.ImgUri)

	// cache it
	tasklock.Lock()
	task[md5] = TSUCCESS
	taskResult[md5] = TaskResult{
		RawUri: options.ImgUri,
		Buf:    &temp,
		Type0:  options.Type0,
	}
	tasklock.Unlock()
	numtag := getNumTag(options.ImgUri)
	if options.Type0 != "" && numtag != "" {
		numtag = options.Type0 + "_" + numtag
		diffmaplock.Lock()
		if _, ok := diffsource[numtag]; !ok {
			diffsource[numtag] = DiffMap{
				MD5:  md5,
				Path: numtag + "_" + md5 + path.Ext(options.ImgUri),
			}
		}
		diffmaplock.Unlock()
	}
	// CachedNotice <- 3
	options.Notice <- SUCCESSFUL
	cp.c++
	logger.Printf("download ok: %d--%s", cp.c, options.ImgUri)
	// ioutil.WriteFile("./1.jpg", b, os.ModeAppend)
	return nil, !resp.Close
}

func Init() {
	dir_list, err := ioutil.ReadDir("cache")
	Montior.before = len(dir_list)
	if err != nil {
		log.Fatal(err)
	}
	regexp_md5, _ := regexp.Compile("_[^\\.]+\\.")
	regexp_num, _ := regexp.Compile("_[^_]+_")
	for _, v := range dir_list {
		md5 := strings.TrimRight(regexp_num.ReplaceAllString(regexp_md5.FindString(v.Name()), ""), ".")
		if _, ok := task[md5]; !ok {
			task[md5] = TCACHED
		}
		sp := strings.Split(v.Name(), "_")
		if len(sp) > 1 {
			if sp[0] != "unknow" && sp[1] != "unknow" {
				key := strings.Join(sp[:2], "_")
				if _, ok := diffsource[key]; !ok {
					diffsource[key] = DiffMap{
						MD5:  md5,
						Path: v.Name(),
					}
				}
			}
		}
	}
	nop()
}

func run_cached() {
	CachedNotice <- 1
	for {
		<-CachedNotice
		var todo []string
		tasklock.RLock()
		for md5, stat := range task {
			if stat == TSUCCESS {
				todo = append(todo, md5)
			}
		}
		tasklock.RUnlock()
		for _, md5 := range todo {
			tasklock.Lock()
			if temp, _ := task[md5]; temp == TSUCCESS {
				file, err := os.OpenFile("./cache/"+getFileName(taskResult[md5].RawUri, taskResult[md5].Type0), os.O_CREATE, 0666)
				if err != nil {
					logger.Panicln(err)
				}
				if _, err := taskResult[md5].Buf.WriteTo(file); err == nil {
					task[md5] = TCACHED
					delete(taskResult, md5)
				} else {
					logger.Println(err)
				}
				file.Close()
			}
			tasklock.Unlock()
		}

		// time.AfterFunc(time.Second*2, func() {
		// 	CachedNotice <- 2
		// })
		nop()
	}
}

func fetchImgWithHttpsProxy(cp *ConnProxy, options RequestOptions) error {
	req, _ := http.NewRequest("GET", options.ImgUri, nil)
	setHeader(req)
	var reader io.Reader
	DialHandle := func(network, addr string) (net.Conn, error) {
		return cp.Conn, nil
	}

	var DefaultTransport http.RoundTripper
	if cp.IsProxy {
		pl, _ := url.Parse("https://" + strings.TrimLeft(cp.Proxy, "https://"))
		DefaultTransport = &http.Transport{
			Proxy: http.ProxyURL(pl),
			// Proxy: http.ProxyFromEnvironment,
			DialTLS: DialHandle,
			// Dial: DialHandle,
			// Dial: (&net.Dialer{
			// 	Timeout:   30 * time.Second,
			// 	KeepAlive: 30 * time.Second,
			// }).Dial,
			DisableKeepAlives:   false,
			TLSHandshakeTimeout: 100 * time.Second,
		}

	} else {
		DefaultTransport = &http.Transport{
			DialTLS: DialHandle,
			// Dial: DialHandle,
			// Dial: (&net.Dialer{
			// 	Timeout:   30 * time.Second,
			// 	KeepAlive: 30 * time.Second,
			// }).Dial,
			DisableKeepAlives:   false,
			TLSHandshakeTimeout: 100 * time.Second,
		}
	}
	// if cp.IsProxy {
	// 	DefaultTransport.Proxy = http.ProxyURL(pl)
	// }

	resp, err := DefaultTransport.RoundTrip(req)
	//a--
	if err != nil {
		// logger.Fatalln(err)
		return err
	}
	reader = resp.Body
	body, err := ioutil.ReadAll(reader)
	if err != nil {
		req.Response.Body.Close()
		logger.Fatalln(err)
		// continue
	}
	logger.Println(string(body), 1)

	Montior.Lock.Lock()
	Montior.ok++
	Montior.Lock.Unlock()

	// 会不会造成栈溢出?
	var temp bytes.Buffer
	if options.NeedRespHeader == 1 {
		nop()
		nop()
		temp.Write([]byte("HTTP/1.1 200 ok\r\n"))
		headerbtyes := getBytes(resp.Header)
		temp.Write(headerbtyes)
		// var rw bufio.ReadWriter
		// resp.Write(&rw)
		// temp.ReadFrom(&rw)
		logger.Println(string(temp.Bytes()))
		nop()
		nop()
	}
	temp.Write(body)
	md5 := getMD5(options.ImgUri)

	// cache it
	tasklock.Lock()
	task[md5] = TSUCCESS
	taskResult[md5] = TaskResult{
		RawUri: options.ImgUri,
		Buf:    &temp,
		Type0:  options.Type0,
	}
	tasklock.Unlock()
	numtag := getNumTag(options.ImgUri)
	if options.Type0 != "" && numtag != "" {
		numtag = options.Type0 + "_" + numtag
		diffmaplock.Lock()
		if _, ok := diffsource[numtag]; !ok {
			diffsource[numtag] = DiffMap{
				MD5:  md5,
				Path: numtag + "_" + md5 + path.Ext(options.ImgUri),
			}
		}
		diffmaplock.Unlock()
	}
	// CachedNotice <- 3
	options.Notice <- SUCCESSFUL

	return nil
}

func getDialHandle(conn *net.Conn) func(network, addr string) (net.Conn, error) {
	return func(network, addr string) (net.Conn, error) {
		return *conn, nil
	}
}

func getp2(isHttp bool) {
	var conn *net.Conn
	DialHandle := getDialHandle(conn)
	a := 0
	pl, err := url.Parse("https://" + pip)
	// pl, err := url.Parse("https://101.37.79.125:3128")
	if err != nil && pl != nil && DialHandle != nil {
	}
	var DefaultTransport http.RoundTripper = &http.Transport{
		Proxy: http.ProxyURL(pl),
		// Proxy: http.ProxyFromEnvironment,
		DialTLS: DialHandle,
		// Dial: DialHandle,
		// Dial: (&net.Dialer{
		// 	Timeout:   30 * time.Second,
		// KeepAlive: 30 * time.Second,
		// }).Dial,
		DisableKeepAlives:   false,
		TLSHandshakeTimeout: 100 * time.Second,
	}
	logger.Println(DefaultTransport)

	for {
		rawurl := <-requestP
		var reader io.Reader
		req, err := http.NewRequest("GET", rawurl, nil)
		if err != nil {
			logger.Fatalln(err)
			continue
		}

		setHeader(req)
		// url, err := url.Parse(rawurl)
		if err != nil {
			logger.Fatalln(err)
			continue
		}
		addrs := getAddr(rawurl)
		if addrs == nil || len(addrs) == 0 {
			continue
		}
		index_addr := -1
	try:
		index_addr++
		if len(addrs) == index_addr {
			continue
		}
		for _, addr := range addrs {
			logger.Printf("parse dns : %s\n", addr)
		}
		if conn == nil {
			// 101.37.79.125:3128
			// 217.61.5.209:3128
			_conn, err := net.Dial("tcp", pip) //218.14.115.211:3128") //"101.37.79.125:3128"
			// _conn, err := net.Dial("tcp", addrs[index_addr]+":"+getHttpOrHttpsPort(url))
			if err != nil {
				logger.Fatalln(err)
				goto try
			}
			logger.Printf("Dial s %s \n", _conn.RemoteAddr())
			conn = &_conn
		}

		if a > -1 {
			resp, err := DefaultTransport.RoundTrip(req)
			//a--
			if err != nil {
				logger.Fatalln(err)
			}
			reader = resp.Body
		} else {
			if err := req.Write(*conn); err != nil {
				logger.Fatalln(err)
				conn = nil
				if len(addrs) == index_addr {
					continue
				}
				goto try
			}

			bufreader := bufio.NewReader(*conn)
			resp, err := http.ReadResponse(bufreader, req)
			if err != nil {
				logger.Fatalln(err)
				conn = nil
				if len(addrs) == index_addr {
					continue
				}
				goto try
			}
			reader = resp.Body
		}

		// reader = resp.Body
		// for a, b := range resp.Header {
		// 	logger.Println(a, " , ", b)
		// }

		body, err := ioutil.ReadAll(reader)
		if err != nil {
			req.Response.Body.Close()
			logger.Fatalln(err)
			continue
		}
		logger.Println(string(body), count)
		count++
		time.Sleep(time.Second * 3)
		requestP <- hh
	}
}
