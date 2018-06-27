package main

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
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
var md5lock sync.RWMutex

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
		logger.Println(err)
		return nil
	}
	addrs, err := net.LookupHost(url.Hostname())
	if err != nil {
		logger.Println(err, url.Hostname())
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

func getNumTag(rawUri string) string {
	if regexfileindex.MatchString(rawUri) {
		return stringPadding(
			strings.TrimRight(strings.TrimLeft(regexfileindex.FindString(rawUri), "fileindex="), ";"),
			"0", 9)
	}
	return ""
}

func nop() {

}

func getName(s string) string {
	regexpClear, _ := regexp.Compile("[^\\w\\W\\d\\D\\[\\]]|\\\\|\\/|:|\\*|\\?|\"|<|>|\\||_|\\s")
	if len(s) > 0 {
		s = regexpClear.ReplaceAllString(s, "")
		if utf8.RuneCountInString(s) > 30 {
			s = Substr2(s, 20, 30)
		} else if utf8.RuneCountInString(s) > 20 {
			s = Substr2(s, 9, 20)
		} else if utf8.RuneCountInString(s) > 10 {
			s = Substr2(s, 1, 9)
		}
	}
	return s
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

func hackQueryUnescape(s string) (string, error) {
	ragexpUnicode, _ := regexp.Compile(`%u[\w]{4}`)
	s = ragexpUnicode.ReplaceAllStringFunc(s, func(m string) string {
		u, err := u2s(strings.Replace(m, "%", `\`, -1))
		if err != nil {
			return m
		}
		return url.QueryEscape(u)
	})
	s, err := url.QueryUnescape(s)
	if err != nil {
		return "", err
	}
	return s, nil
}

func getBytes(h http.Header) []byte {
	var b bytes.Buffer
	for a, v := range h {
		b.WriteString(a + ":" + strings.Join(v, ";") + "\r\n")
	}
	b.WriteString("\r\n")
	return b.Bytes()
}

func getHeader(b []byte) http.Header {
	var h http.Header = make(http.Header)
	l := len(b)
	s := 0
	for i, _ := range b {
		if i < l-2 {
			if string(b[i:i+2]) == "\r\n" {
				spIndex := bytes.Index(b[s:i], []byte(":"))
				if spIndex == -1 {
					goto end
				}
				nop()
				nop()
				h[strings.Trim(string(b[s:s+spIndex]), " ")] = append(h[strings.Trim(string(b[s:s+spIndex]), " ")], strings.Trim(string(b[s+spIndex+1:i]), " "))
				//strings.Split(
				//, ";")
				s = i + 2
			}
		}
	}
	nop()
end:
	return h
}
