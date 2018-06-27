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
	p := Provider{"123.125.115.110:80"}
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
