package main

import (
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ConnProxy struct {
	Dst       string
	Proxy     string
	IsInit    bool
	IsProxy   bool
	IsInvalid bool
	IsBusy    bool
	Stat      Statistic
	Conn      net.Conn
	StartTime time.Time
	EndTime   time.Time
	c         int
}

type Statistic struct {
	F  int
	S  int
	AF int
	AS int
}

var ConnPool struct {
	connChan    chan *ConnProxy
	ditaling    map[string]time.Time
	conn        map[net.Conn]*ConnProxy
	mu          *sync.RWMutex
	recycleChan chan int
}

type ProxyItem struct {
	ip   string
	port int
}

var proxies map[string]ProxyItem
var proxiesditalmap map[int64]map[string]bool

const retryCount int = 3

var work1id int32 = 0

//var workid int = 1
// var requestQ chan i
// var requestP chan string
// var logger *log.Logger

func init() {
	//requestQ = make(chan i, 20)
	//requestP = make(chan string, 20)
	//logger = log.New(os.Stdout, "logger :", log.Lshortfile)
	ConnPool.connChan = make(chan *ConnProxy, 20)
	ConnPool.conn = make(map[net.Conn]*ConnProxy)
	ConnPool.mu = &sync.RWMutex{}
	ConnPool.recycleChan = make(chan int, 2)
	ConnPool.ditaling = make(map[string]time.Time)
	proxies = make(map[string]ProxyItem)
	proxiesditalmap = make(map[int64]map[string]bool)
}

func pushConn(dst string, useProxy bool, proxy string) {
	conn := ConnProxy{
		Dst:     dst,
		IsInit:  false,
		c:       0,
		IsProxy: useProxy,
		Proxy:   proxy,
	}
	ConnPool.connChan <- &conn
}

func getConn(dst string) (*ConnProxy, int) {
	ConnPool.mu.Lock()
	defer ConnPool.mu.Unlock()
	count := 0
	for _, cp := range ConnPool.conn {
		if cp.Dst == dst {
			count++
		}
		if cp.Dst == dst && cp.IsBusy == false {
			cp.IsBusy = true
			return cp, count
		}
	}
	if _, ok := ConnPool.ditaling[dst]; !ok {
		ConnPool.ditaling[dst] = time.Now()
		pushConn(dst, false, "")
	}
	return nil, count
}

func getProxy(dst string, id int64) (*ConnProxy, int) {
	ConnPool.mu.Lock()
	defer ConnPool.mu.Unlock()
	count := 0
	for _, cp := range ConnPool.conn {
		if cp.Dst == dst && cp.IsProxy {
			count++
		}
		if cp.Dst == dst && cp.IsBusy == false && cp.IsProxy {
			cp.IsBusy = true
			return cp, count
		}
	}
	resp, err := http.Get("http://127.0.0.1:8000/?types=0&count=15&country=%E5%9B%BD%E5%86%85")
	if err != nil {
		// handle error
		return nil, 0
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	content := string(body)
	re, _ := regexp.Compile("\\[|\\]|\\\"|\\s")
	rs := re.ReplaceAllString(content, "")
	arr := strings.Split(rs, ",")
	for i := 0; i < len(arr); i += 3 {
		item := ProxyItem{}
		item.ip = arr[i]
		item.port, _ = strconv.Atoi(arr[i+1])
		if _, ok := proxies[item.ip+":"+arr[i+1]]; !ok {
			proxies[item.ip+":"+arr[i+1]] = item
		}
	}

	if _, ok := proxiesditalmap[id]; ok {
		return nil, count
	}
	countProxy := 0
	var idMap = make(map[string]bool)
	for key, _ := range proxies {
		if _, ok := ConnPool.ditaling[dst+","+key]; !ok {
			ConnPool.ditaling[dst+","+key] = time.Now()
			idMap[dst+","+key] = true
			pushConn(dst, true, key)
			countProxy++
		}
		if countProxy >= 5 {
			break
		}
	}
	if countProxy > 0 {
		proxiesditalmap[id] = idMap
	}
	// if _, ok := ConnPool.ditaling[dst]; !ok {
	// 	ConnPool.ditaling[dst] = time.Now()
	// }
	return nil, count
}

func removeDitaling(dst string) {
	ConnPool.mu.Lock()
	if _, ok := ConnPool.ditaling[dst]; ok {
		delete(ConnPool.ditaling, dst)
	}
	ConnPool.mu.Unlock()
}

func recycle() {
	for {
		<-ConnPool.recycleChan
		ConnPool.mu.Lock()
		// delkeys := make([]net.Conn, len(ConnPool.conn))
		for c, cp := range ConnPool.conn {
			if cp.IsInvalid == true {
				cp.Conn.Close()
				delete(ConnPool.conn, c)
			} else {
				err := cp.Conn.SetDeadline(time.Now().Add(time.Minute * 1))
				if err != nil {
					cp.Conn.Close()
					delete(ConnPool.conn, c)
				}
			}
		}
		ConnPool.mu.Unlock()
	}
}

func work_1() {
	workid := atomic.AddInt32(&work1id, 1)
	for {
		itidle(FlagWorkType1, workid)
		cp := <-ConnPool.connChan
		itbusy(FlagWorkType1, workid)
		if cp.IsInit {
			time.Sleep(time.Second * 1)
			continue
		}
	retry:
		try_count := 0
		dst := cp.Dst
		if cp.IsProxy {
			dst = cp.Proxy
		}
		conn, err := net.Dial("tcp", dst)
		if err != nil {
			cp.Stat.F += 1
			if try_count < retryCount {
				logger.Printf("retry dial %s\n", dst)
				goto retry
			} else if try_count < 5 {
				logger.Printf("retry dial %s\n", dst)
				ConnPool.connChan <- cp
			}
			if !cp.IsProxy {
				removeDitaling(cp.Dst)
			} else {
				removeDitaling(cp.Dst + "," + cp.Proxy)
			}
			continue
		}
		// logger.Println("collection successful: " + cp.Dst)
		cp.StartTime = time.Now()
		cp.Stat.S += 1
		cp.IsInit = true
		cp.Conn = conn
		ConnPool.mu.Lock()
		ConnPool.conn[conn] = cp
		ConnPool.mu.Unlock()
		if !cp.IsProxy {
			removeDitaling(cp.Dst)
		} else {
			removeDitaling(cp.Dst + "," + cp.Proxy)
		}
		//ConnPool.connChan <- cp
	}
}

func run_1() {
	go recycle()
	ticker := time.NewTicker(time.Second * 2)
	go func() {
		for {
			select {
			case <-ticker.C:
				dd := 2
				dd++
				ConnPool.recycleChan <- 1
			}
		}

	}()
	for i := 0; i < 5; i++ {
		go work_1()
	}
}

func randomConn() {

}
