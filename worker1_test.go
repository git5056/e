package main

import (
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	go main()
	time.Sleep(time.Second * 5)
	op := ProviderOption{
		Count: 5,
	}
	p := Provider{Dst: "123.125.115.110:80"}
	p.Init()
	r := (&p).Pop(op)
	for {
		a := <-r
		logger.Println(a)
	}

	Sleep()
	for {
		Sleep()
	}
}
