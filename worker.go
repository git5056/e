package main

import (
	"bufio"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

type i struct {
	pick string
	conn net.Conn
}

var workid int = 1
var requestQ chan i
var requestP chan string
var logger *log.Logger
var connMap map[string]net.Conn

func init() {
	requestQ = make(chan i, 20)
	requestP = make(chan string, 20)
	// connMap = make(chan string, 20)
	logger = log.New(os.Stdout, "logger :", log.Lshortfile)
}

func run(port int) {
	for i := 1; i <= 5; i++ {
		go working()

	}
	go getp()
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		logger.Fatal(err)
	}
	logger.Printf("listen on port %d", port)
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Fatal(err)
			continue
		}
		logger.Printf("new coll from %s \n", conn.RemoteAddr())
		requestQ <- i{
			conn: conn,
			pick: "aaaaa",
		}
	}
}

func working() {
	id := workid
	workid++
	for {
		t := <-requestQ
		logger.Printf("working at {%d} : %s", id, t.pick)
		t.conn.Close()
		requestP <- hh
		// time.Sleep(time.Second * 3)
	}
}

var hh string = "https://www.baidu.com/"

// var hh string = "https://www.google.com"

// var hh string = "https://www.baidu.com"

// var hh string = "http://example.com"
var count int = 1

// 171.221.202.181	63000	四川	高匿
// 101.37.79.125:3128 浙江省杭州市 阿里云计算有限公司 阿里云
// 118.212.137.135	31288	江西	透明	HTTPS
// 112.115.57.20	3128	云南昆明	透明	HTTP
// 221.228.17.172	8181	江苏无锡	高匿
// 120.78.78.141:8888
// var pip string = "171.10.31.74:80"
// var pip string = "112.115.57.20:3128"

var pip string = "14.215.177.38:443"

func getp() {
	var conn *net.Conn
	DialHandle := func(network, addr string) (net.Conn, error) {
		return *conn, nil
	}
	a := 0
	pl, err := url.Parse("https://" + pip)
	// pl, err := url.Parse("https://101.37.79.125:3128")
	if err != nil && pl != nil && DialHandle != nil {
	}
	var DefaultTransport http.RoundTripper = &http.Transport{
		Proxy: http.ProxyURL(pl),
		// Proxy:   http.ProxyFromEnvironment, // 暂时发现，要使用transport必须要使用proxy
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
