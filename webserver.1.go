package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
)

var buf []byte
var mu2 = &sync.RWMutex{}

func init() {
	f, _ := os.OpenFile("./p.jpg", os.O_RDONLY, 0666)
	buf = make([]byte, 1024*1000000)
	index, _ := f.Read(buf)
	buf = buf[:index]
	f.Close()
}

func run_server1() {
	http.HandleFunc("/p", func(w http.ResponseWriter, r *http.Request) {
		// time.Sleep(time.Second * 5)
		mu2.RLock()
		temp := buf[:]
		mu2.RUnlock()
		w.Header().Set("Content-Type", "image/jpg")
		w.WriteHeader(200)
		w.Write(temp)
		return
	})
	log.Fatal(http.ListenAndServe(":9080", nil))
}

var countx int32

func runmany() {
	return
	for i := 0; i < 100; i++ {
		go func(url string) {
			resp, err := http.Get(url)
			if err != nil {
				// handle error
			}
			// defer
			body, _ := ioutil.ReadAll(resp.Body)
			if body != nil {

			}
			if len(body) == 0 {
				logger.Println("xxxxxxxxxxxxxxxxxxxxxxxxxxx", atomic.AddInt32(&countx, 1))
			}
			resp.Body.Close()
			// fmt.Println(resp.Close)
		}("http://127.0.0.1:8080/?type=123&img=http://127.0.0.1:9080/p?i=" + strconv.Itoa(i) + ".jpg")
	}
}
