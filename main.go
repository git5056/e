package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("start")
	// Init()
	// go runfetch()
	// for {
	// 	time.Sleep(time.Second)
	// }
	// main1()
	go run(9923)

	Init()
	run_1()
	go postTask()

	run_2()
	// run_montior()
	// go func() {
	// 	time.Sleep(time.Second * 3)
	// 	requestImg <- "http://pics.sc.chinaz.com/files/pic/pic9/201806/zzpic12389.jpg"
	// 	return
	// 	// pushConn("123.125.115.110:80")
	// 	// time.Sleep(time.Second * 3)
	// 	// cp := getConn("123.125.115.110:80")
	// 	// if cp != nil {
	// 	// }
	// 	// return

	// montior
	go run_m()
	go runTimer()

	// work2
	go run_cached()
	go run_server()
	go runmany()

	go run_server1()
	run_server2()
	//run(12345)
	for {
		time.Sleep(1 * time.Second)
	}

}

func Sleep() {
	time.Sleep(time.Second * 60)
}
