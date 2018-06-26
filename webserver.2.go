package main

import (
	"bytes"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//http://www.w3school.com.cn
var baseUrl string = "https://www.baidu.com"

func run_server2() {
	myHander := &MyHander{}
	s := &http.Server{
		Addr:           ":8090",
		Handler:        myHander,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
}

type MyHander struct {
}

func (mh *MyHander) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rawUri := baseUrl + r.RequestURI
	ISSSL := 0
	if strings.HasPrefix(baseUrl, "https") {
		ISSSL++
	}
	chan0 := make(chan int32, 20) // must be greater than one
	requestImg <- RequestOptions{
		ImgUri:         rawUri,
		Type0:          "",
		Notice:         chan0,
		NotCache:       1,
		NeedRespHeader: 1,
		ISSSL:          int32(ISSSL),
	}
	md5 := getMD5(rawUri)

	// var diffmapping DiffMap

	// over time
	ticker := time.NewTicker(time.Second * 6)
	select {
	case nr := <-chan0:
		if nr == SUCCESSFUL || nr == SUCCESSFULCACHED {
			tasklock.RLock() // 多余
			if stat, ok := task[md5]; ok && (stat == TSUCCESS || stat == TCACHED) {
				tasklock.RUnlock() // 多余
				tasklock.Lock()
				if stat, ok := task[md5]; ok && (stat == TSUCCESS || stat == TCACHED) {
					if stat == TSUCCESS {
						temp := taskResult[md5].Buf.Bytes()
						tasklock.Unlock()
						index := bytes.Index(temp, []byte("\r\n"))
						statsLine := temp[:index+1]
						temp = temp[index+2:]
						respHeader := getHeader(temp)
						index = bytes.Index(temp, []byte("\r\n\r\n"))
						for k, v := range respHeader {
							w.Header().Set(k, strings.Join(v, ";"))
						}
						statusCode, _ := strconv.Atoi(strings.Split(string(statsLine), " ")[1])
						if when_cache(respHeader) {
							nop()
							nop()
						}
						w.WriteHeader(statusCode)
						w.Write(temp[index+4:])
					} else {
						tasklock.Unlock()
						Montior.Lock.Lock()
						Montior.serverCached++
						Montior.Lock.Unlock()
						// os.open
					}
				} else {
					tasklock.Unlock()
				}
			} else {
				tasklock.RUnlock()
			}
		} else if nr == FAILED {
			SendLocation(w, r.RequestURI)
		} else if nr == Wait {
			// xxxxxxxxxxxxxx
		} else {
			w.WriteHeader(404)
		}
		return
		// over time
	case <-ticker.C:
		SendLocation(w, r.RequestURI)
		return
	}
	// time.Sleep(time.Second * 1)
	// // mu2.RLock()
	// // temp := buf[:]
	// // mu2.RUnlock()
	// w.Header().Set("Content-Type", "image/jpg")
	// w.Header().Set("Keep-Alive", "Close")
	// w.WriteHeader(200)
	// w.Write(temp)
	return
}

// func ditalCache()

func when_cache(h http.Header) bool {
	if v, ok := h["Content-Type"]; ok && strings.Contains(strings.Join(v, ";"), "image") {
		return true
	}
	if v, ok := h["content-type"]; ok && strings.Contains(strings.Join(v, ";"), "image") {
		return true
	}
	return false
}
