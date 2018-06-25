package main

import (
	"crypto/md5"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

var dnsMap map[string][]string
var mu *sync.Mutex
var regexPath *regexp.Regexp
var regexPathReplaced *regexp.Regexp
var regex_num *regexp.Regexp
var regexfileindex *regexp.Regexp

func init() {
	dnsMap = make(map[string][]string)
	mu = &sync.Mutex{}
	regexPath, _ = regexp.Compile("(\\d*)\\.[^\\.]*$")
	regexPathReplaced, _ = regexp.Compile("\\..*$")
	regex_num, _ = regexp.Compile("_\\d*_")
	regexfileindex, _ = regexp.Compile("fileindex=\\d+;?")
}

func setHeader(req *http.Request) {
	header := map[string][]string{
		// "Accept-Encoding": {"gzip, deflate"},
		"Accept-Language":           {"en-us"},
		"Connection":                {"keep-alive"}, //keep-alive
		"Upgrade-Insecure-Requests": {"1"},
		"User-Agent":                {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/66.0.3359.181 Safari/537.36"},
	}
	req.Header = header
}

func getHttpOrHttpsPort(url *url.URL) string {
	if len(url.Port()) == 0 {
		if url.Scheme == "http" {
			return "80"
		}
		if url.Scheme == "https" {
			return "443"
		}
		return ""
	}
	return url.Port()
}

func getAddr(rawurl string) []string {
	mu.Lock()
	if d, ok := dnsMap[rawurl]; ok {
		mu.Unlock()
		return d
	}
	mu.Unlock()
	url, err := url.Parse(rawurl)
	if err != nil {
		logger.Fatalln(err)
		return nil
	}
	addrs, err := net.LookupHost(url.Hostname())
	if err != nil {
		logger.Fatalln(err)
		return nil
	}
	mu.Lock()
	dnsMap[rawurl] = addrs
	mu.Unlock()
	return addrs
}

func getMD5(input string) string {
	h := md5.New()
	h.Write([]byte(input))
	cipherStr := h.Sum(nil)
	return hex.EncodeToString(cipherStr)
}

func getFileName(rawUri, type0 string) string {
	name := getMD5(rawUri) + path.Ext(rawUri)
	if regexfileindex.MatchString(rawUri) {
		name = stringPadding(
			strings.TrimRight(strings.TrimLeft(regexfileindex.FindString(rawUri), "fileindex="), ";"),
			"0", 9) + "_" + name
	} else {
		name = "unknow_" + name
	}
	if utf8.RuneCountInString(type0) > 0 {
		name = type0 + "_" + name
	} else {
		name = "unknow_" + name
	}
	return name
}

func stringPadding(input, pad string, len int) string {
	remain := len - utf8.RuneCountInString(input)
	if remain > 0 {
		return strings.Repeat(pad, remain) + input
	}
	return input
}

func getNumTag(rawUri string)string{
	if regexfileindex.MatchString(rawUri) {
		return stringPadding(
		strings.TrimRight(strings.TrimLeft(regexfileindex.FindString(rawUri), "fileindex="), ";"),
		"0", 9)
	}
	return ""
}

func nop(){
	
}