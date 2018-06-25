package main

import (
	"bytes"
	"log"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"net/url"
	"encoding/binary"
	"encoding/hex"
)

var re *regexp.Regexp
var regexpTirmRand *regexp.Regexp
var regexpClear  *regexp.Regexp
var ragexptype *regexp.Regexp
var ragexpup5 *regexp.Regexp

const (
	SUCCESSFUL = 1
	FAILED     = 2
	SUCCESSFULCACHED = 3
	Wait = 6
)

func init() {
	re, _ = regexp.Compile(".*(img=[^&]*)(&rand=\\d*)?$")
	regexpTirmRand, _ = regexp.Compile("&rand=\\d*$")
	regexpClear, _=regexp.Compile("[^\\w\\W\\d\\D\\[\\]]|\\\\|\\/|:|\\*|\\?|\"|<|>|\\||_|\\s")
	ragexptype, _=regexp.Compile("type=[^&]*&?")
	ragexpup5, _=regexp.Compile("%u....")
}

func u2s(form string) (to string, err error) {
    bs, err := hex.DecodeString(strings.Replace(form, `\u`, ``, -1))
    if err != nil {
        return
    }
    for i, bl, br, r := 0, len(bs), bytes.NewReader(bs), uint16(0); i < bl; i += 2 {
        binary.Read(br, binary.BigEndian, &r)
        to += string(r)
    }
    return
}
func run_server() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		Montior.Lock.Lock()
		Montior.workServerCount++
		Montior.Lock.Unlock()
		defer func() {
			Montior.Lock.Lock()
			Montior.workServerCount--
			Montior.Lock.Unlock()
		}()
		r.ParseForm()
		v := r.Form["img"]
		if len(v) > 0 && v[0]!=""{
			rawurl := strings.TrimPrefix(re.ReplaceAllString(r.URL.RawQuery, "$1"), "img=")
		    rawurl, _ = url.QueryUnescape(rawurl)
			// fmt.Println(r.URL.RawQuery, "---------------------")
			append := ""
			if ragexptype.MatchString(r.URL.RawQuery){
				typeP := ragexptype.FindString(r.URL.RawQuery)
				typeP =strings.TrimRight(strings.TrimLeft(typeP, "type="), "&")
				allup5 := ragexpup5.FindAllString(typeP, -1)
				new := ""
				if allup5!= nil{
					for _,z:=range allup5{
						ustr :=strings.Replace(z, "%", "\\", 1)
						u, err := u2s(ustr)
						if err!=nil{
							dd:=2
							dd++
						}
						escape := url.QueryEscape(u)
						typeP = strings.Replace(typeP, z, escape, 1)
						new+=u
					}
				}
				unescape, err := url.QueryUnescape(typeP)
				if err ==nil{
					append=unescape
					// logger.Println(append)
				}
			}
			if len(append) > 0 {
				append = regexpClear.ReplaceAllString(append, "")
				if utf8.RuneCountInString(append) > 30{
					append = Substr2(append,20,30)
				}else if utf8.RuneCountInString(append) > 20{
					append = Substr2(append,9,20)
				}else if utf8.RuneCountInString(append) > 10{
					append = Substr2(append,1,9)
				}else{
					append = Substr2(append,0,utf8.RuneCountInString(append)-1)
				}
			}
			chan0 := make(chan int32, 20) // must be greater than one

			if(tryUseCache(rawurl, append)){
				chan0<-SUCCESSFULCACHED
			}else{
				requestImg <- RequestOptions{
					ImgUri: rawurl,
					Type0:  append,
					Notice: chan0,
				}
			}

			md5 := getMD5(rawurl)
			var diffmapping DiffMap
			// watch := time.Now()

			// over time
			ticker := time.NewTicker(time.Second * 6)
			select {
			case nr := <-chan0:
				if nr == SUCCESSFULCACHED{
					// unsafe, but 
					diffmapping = diffsource[append+"_"+getNumTag(rawurl)]
					md5=diffmapping.MD5
				}
				if nr == SUCCESSFUL || nr == SUCCESSFULCACHED {
					tasklock.RLock()
					if stat, ok := task[md5]; ok && (stat == TSUCCESS || stat == TCACHED) {
						tasklock.RUnlock()
						tasklock.Lock()
						if stat, ok := task[md5]; ok && (stat == TSUCCESS || stat == TCACHED) {
							if stat == TSUCCESS {
								w.Header().Set("Content-Type", "image/"+strings.TrimPrefix(path.Ext(rawurl), "."))
								w.WriteHeader(200)
								if _, ok:=taskResult[md5];!ok{
									nop()
									nop()
								}
								if taskResult[md5].Buf==nil{
									nop()
									nop()
								}
								temp:=taskResult[md5].Buf.Bytes()

								// taskResult[md5].Buf = bytes.NewBuffer(temp)
								taskResult[md5].Buf.Reset()
								taskResult[md5].Buf.Write(temp)
								tasklock.Unlock()

								w.Write(temp)
							} else {
								tasklock.Unlock()
								Montior.Lock.Lock()
								Montior.serverCached++
								Montior.Lock.Unlock()
								newuri := ""
								if nr == SUCCESSFULCACHED{
									newuri = "/cache/" + diffmapping.Path
								}else {
									newuri = "/cache/" + getFileName(rawurl, append)
									nop()
								}
								w.Header().Set("Location", newuri)
								w.WriteHeader(302)
							}
						}else {
							tasklock.Unlock()
						}

					}else{
						tasklock.RUnlock()
					}
				} else if nr==FAILED{
					SendLocation(w, r.RequestURI)
				}else if nr==Wait{
					// xxxxxxxxxxxxxx
				}else{
					w.WriteHeader(404)
				}
				return
				// over time
			case <-ticker.C:
				SendLocation(w, r.RequestURI)
				return
			}
		}
		// end:
		w.WriteHeader(404)
		//fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
	})

	http.Handle("/cache/", http.StripPrefix("/cache/", http.FileServer(http.Dir("cache"))))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func SendImage(w http.ResponseWriter, buf *bytes.Buffer, rawurl string) {
	defer func() {
		// lockForresultMap.Lock()
		if _, ok := resultMap[rawurl]; ok {
			delete(resultMap, rawurl)
		}
		// lockForresultMap.Unlock()
	}()
	data := make([]byte, buf.Len())
	buf.Read(data)
	w.Header().Set("Content-Type", "image/"+strings.TrimPrefix(path.Ext(rawurl), "."))
	w.WriteHeader(200)
	w.Write(data)
}

func SendLocation(w http.ResponseWriter, oldUri string) {
	rndnum := strconv.Itoa(int(time.Now().Unix()))
	newurl := regexpTirmRand.ReplaceAllString(oldUri, "")
	newurl = newurl + "&rand=" + rndnum
	w.Header().Set("Location", newurl)
	w.WriteHeader(302)
}

func Substr2(str string, start int, end int) string {
	rs := []rune(str)
	length := len(rs)

	if start < 0 || start > length {
		panic("start is wrong")
	}

	if end < 0 || end > length {
		panic("end is wrong")
	}

	return string(rs[start:end])
}

func tryUseCache(rawUri, type0 string) bool{
	numtag:=getNumTag(rawUri)
	if len(numtag) + len(type0) > 1{
		diffmaplock.RLock()	
		if _, ok := diffsource[type0+"_"+numtag];ok{
			diffmaplock.RUnlock()
			return true
		}
		diffmaplock.RUnlock()
	}
	return false
}