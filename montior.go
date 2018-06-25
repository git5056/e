package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

var Montior struct {
	work1           map[int32]bool
	work2           map[int32]bool
	workServerCount int
	serverCached    int
	serverNot       int
	before			int
	ok     	        int
	Lock            *sync.RWMutex
	Chan            chan interface{}
	Handler         []interface{}
	Work            map[string]bool
}

type M interface {
	Do()
	Continue() bool
}

type M0 struct {
	Id       int32
	WorkType int32
	WorkId   int32
}

func (m *M0) Do() {
	ret := false
	if m.Id == IDBUSY {
		ret = true
	}
	if m.WorkType == FlagWorkType1 {
		Montior.work1[m.WorkId] = ret
	} else {
		Montior.work2[m.WorkId] = ret
	}
}

func (m *M0) Continue() bool {
	return false
}

const (
	IDBUSY        = 1
	IDIDLE        = 99
	IDFRESH       = 100
	FlagWorkType1 = 1
	FlagWorkType2 = 2
)

type m2m struct {
	aa string
}

type Demo struct {
	m2m
}

func (d *Demo) test(s string) {
	d.aa = ""
}

var PID int

func init() {
	Montior.work1 = make(map[int32]bool)
	Montior.work2 = make(map[int32]bool)
	Montior.workServerCount = 0
	Montior.Lock = &sync.RWMutex{}
	Montior.Chan = make(chan interface{}, 20)
	Montior.Handler = make([]interface{}, 0)
	PID = os.Getpid()
}

func itbusy(worktype, workid int32) {
	Montior.Chan <- &M0{
		Id:       IDBUSY,
		WorkType: worktype,
		WorkId:   workid,
	}
}

func itidle(worktype, workid int32) {
	Montior.Chan <- &M0{
		Id:       IDIDLE,
		WorkType: worktype,
		WorkId:   workid,
	}
}

func netstat() *bytes.Buffer {
	var out bytes.Buffer
	c1 := exec.Command("netstat", "-nao")
	c2 := exec.Command("findstr", strconv.Itoa(PID))
	c2.Stdin, _ = c1.StdoutPipe()
	// c1.Stdout, _ = c2.StdinPipe()
	c2.Stdout = &out
	c2.Start()
	c1.Run()
	c2.Wait()
	return &out
}

var lastOutPut = ""

func fresh() {
	// go func() {
	// lastOutPut := ""
	// for {
	count1 := 0
	allcount1 := 0
	count2 := 0
	allcount2 := 0
	for _, b := range Montior.work1 {
		allcount1++
		if b {
			count1++
		}
	}
	for _, b := range Montior.work2 {
		allcount2++
		if b {
			count2++
		}
	}
	// countConn := 0
	allcountConn := 0
	countBusy := 0
	for _, cp := range ConnPool.conn {
		allcountConn++
		if cp.IsBusy {
			countBusy++
		}
	}
	out := netstat()
	str := string(out.Bytes())
	var newOutPut = "work2:{" + strconv.Itoa(count2) + "/" + strconv.Itoa(allcount2) + "};work1:{" + strconv.Itoa(count1) + "/" + strconv.Itoa(allcount1) + "}\n" +
		"collection:{" + strconv.Itoa(countBusy) + "/" + strconv.Itoa(allcountConn) + "}\n" +
		"dital:{" + strconv.Itoa(len(ConnPool.ditaling)) + "}\n" +
		"server:{" + strconv.Itoa(Montior.workServerCount) + "}\n" +
		"serverNot:{" + strconv.Itoa(Montior.serverNot) + "}\n" +
		"incress:{" + strconv.Itoa(Montior.ok) + "}  should:{" + strconv.Itoa(Montior.ok+ Montior.before) +"}  before:{"+ strconv.Itoa(Montior.before)  +"}\n" +
		"servercached:{" + strconv.Itoa(Montior.serverCached) + "}\n" +
		"" + str + "\n"
	if lastOutPut == newOutPut {
		time.Sleep(time.Millisecond * 300)
		// continue
		return
	}
	lastOutPut = newOutPut
	var f *os.File
	f, _ = os.OpenFile("./m.log", os.O_CREATE|os.O_WRONLY, 0666)
	f.Seek(0, 0)
	var input bytes.Buffer
	fmt.Fprintf(&input, newOutPut)
	fmt.Fprintf(f, newOutPut)
	err := f.Truncate(int64(input.Len()))
	if err != nil {
		fmt.Println(err.Error())
	}
	// fmt.Printf("work2:{%d/%d};work1:{%d/%d}\n", count2, allcount2, count1, allcount1)
	f.Close()
	time.Sleep(time.Millisecond * 300)
	// }
	// }()
}

func addHandle(i interface{}) []interface{} {
	Montior.Lock.Lock()
	defer Montior.Lock.Unlock()
	Montior.Handler = append(Montior.Handler[:], i)
	return Montior.Handler
}

func runTimer() {
	ticker := time.NewTicker(time.Second * 1)
	for {
		select {
		case <-ticker.C:
			Montior.Chan <- IDFRESH
		}
	}
}

func run_m() {
	for {
		im := <-Montior.Chan
		if m, ok := im.(M); ok {
			m.Do()
			if !m.Continue() {
				continue
			}
		}
		id := im.(int)
		if IDFRESH == id {
			fresh()
			continue
		}
		if id > IDIDLE {
			Montior.Work["work"+strconv.Itoa(id-IDIDLE)] = false
		} else {
			Montior.Work["work"+strconv.Itoa(id-IDBUSY)] = true
		}
	}
}
