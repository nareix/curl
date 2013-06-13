
/*
	Curl library for golang (: WITHOUT libcurl.so :)
	Just using "net/http"

	import "github.com/go-av/curl"

	// Basic Usage

	// get string or bytes
	err, str := curl.CurlString("www.baidu.com")
	err, b := curl.CurlBytes("www.baidu.com")
	
	// save to file
	err := curl.CurlFile("www.baidu.com", "/tmp/index.html")

	// save to writer
	var w io.Writer
	err := curl.CurlWrite("www.ccc.com", w)

	// with timeout (both dial timeout and read timeout set)
	curl.CurlString("www.baidu.com", "timeout=10")
	curl.CurlString("www.baidu.com", "timeout=", 10)
	curl.CurlString("www.baidu.com", "timeout=", time.Second*10)

	// with different dial timeout and read timeout
	curl.CurlString("www.baidu.com", "dialtimeout=10", "readtimeout=20")
	curl.CurlString("www.baidu.com", "dialtimeout=", 10, "readtimeout=", time.Second*20)

	// with deadline (if cannot download in 10s then die)
	curl.CurlFile("xx", "xx", "deadline=", time.Now().Add(time.Second*10))
	curl.CurlFile("xx", "xx", "deadline=10")
	curl.CurlFile("xx", "xx", "deadline=", 10.0)
	curl.CurlFile("xx", "xx", "deadline=", time.Second*10)

	// set http header
	header := http.Header {
		"User-Agent" : {"curl/7.29.0"},
	}
	curl.CurlFile("http:/xxx", "file", header)

	// you can put params in any function in any order. so as below
	curl.CurlFile("kernel.org/3.6.4.tar.bz2", "haha.bz2", "timeout=", 10, header)
	curl.CurlString("www.baidu.com", header", timeout=", 10)

	// Advanced Usage

	// I just want the result
	var st curl.IocopyStat
	err := curl.CurlFile("kernel.org/3.6.4.tar.bz2", &st)

	// I want to know progress
	cb := func (st curl.IocopyStat) error {
		fmt.Println(st.Perstr, st.Speedstr, st.Sizestr, st.Lengthstr)
		if forceStop {
			return errors.New("user force stop")
		}
		return nil
	}
	curl.CurlFile("kernel.org/3.6.4.tar.bz2", "/tmp/a.bin", cb, "timeout=10")
	// it will print it per second
	// 3.0% 220K/s 2.1M 109M
	// 4.0% 110K/s 3.3M 109M
	// ...

	// set callback interval
	curl.CurlFile("xxxx", "xxx", cb, "cbinterval=3.0")  // 3 seconds
	curl.CurlFile("xxxx", "xxx", cb, "cbinterval=", 0.5) // 0.5 second
	curl.SetCallbackInterval(1.0) // 1 second

	// I want to control everything
	con := &curl.Control{}
	go curl.CurlFile("kernel.org/xxx", "xxx", con)
	// and then get stat
	st := con.Stat() 
	// or stop
	con.Stop()

	// I make everything myself ...
	err, r, length := curl.Dial("xx", "timeout=11")

	// some functions format size, speed pretty
	curl.PrettySize(13500) // 13.5K
	curl.PrettySize(2500000) // 2.5M
	curl.PrettyPer(0.345) // 34.5%
	curl.PrettySpeed(1200) // 1.2K/s
	curl.PrettyDur(time.Second*66) // 1:06

	// progressed io.Copy
	curl.IoCopy(r, 123, w, "readtimeout=12", cb)
	
*/
package curl

import (
	"os"
	"log"
	"errors"
	"fmt"
	"net"
	"net/http"
	"bytes"
	"io"
	"time"
	"strings"
)

type IocopyStat struct {
	Stat string					// dial,download
	Done bool 					// download is done
	Begin time.Time 		// download begin time
	Dur time.Duration 	// download elapsed time
	Per float64 				// complete percent. range 0.0 ~ 1.0
	Size int64 					// bytes downloaded
	Speed int64 				// bytes per second
	Length int64 				// content length
	Durstr string 			// pretty format of Dur. like: 10:11
	Perstr string 			// pretty format of Per. like: 3.9%
	Sizestr string  		// pretty format of Size. like: 1.1M, 3.5G, 33K
	Speedstr string 		// pretty format of Speed. like 1.1M/s
	Lengthstr string 		// pretty format of Length. like: 1.1M, 3.5G, 33K
}

type Control struct {
	stop bool
	st *IocopyStat
	w *myWriter
}

type IocopyCb func (st IocopyStat) error

type deadlineS interface {
	SetReadDeadline(t time.Time) error
}

func (c *Control) Stop() {
	c.stop = true
}

func (c *Control) Stat() (IocopyStat) {
	c.st.update()
	return *c.st
}

func (c *Control) MaxSpeed(s int64) {
	if s == 0 {
		c.w.hasmaxspeed = false
	} else {
		c.w.hasmaxspeed = true
		c.w.maxspeed = s
	}
}

func toFloat(o interface{}) (can bool, f float64) {
	switch o.(type) {
	case string:
		str := o.(string)
		n, _ := fmt.Sscanf(str, "%f", &f)
		if n == 1 {
			can = true
		}
	case float64:
		f = o.(float64)
		can = true
	case int:
		f = float64(o.(int))
		can = true			
	case int64:
		f = float64(o.(int64))
		can = true
	}
	return
}

func optGet(name string, opts []interface{}) (got bool, val interface{}) {
	for i, o := range opts {
		switch o.(type) {
		case string:
			stro := o.(string)
			if strings.HasPrefix(stro, name) {
				if len(stro) == len(name) {
					if i+1 < len(opts) {
						val = opts[i+1]
					}
				} else {
					val = stro[len(name):]
				}
				got = true
				return
			}
		}
	}
	return	
}

func optDuration(name string, opts []interface{}) (got bool, dur time.Duration) {
	var val interface{}
	var f float64
	if got, val = optGet(name, opts); !got { return }
	if dur, got = val.(time.Duration); got { return }
	if got, f = toFloat(val); !got { return }
	dur = time.Duration(float64(time.Second)*f)
	return
}

func optTime(name string, opts []interface{}) (got bool, tm time.Time) {
	var val interface{}
	got, val = optGet(name, opts)
	tm, got = val.(time.Time)
	return
}

func optInt64(name string, opts []interface{}) (got bool, i int64) {
	var val interface{}
	var f float64
	got, val = optGet(name, opts)
	if got, f = toFloat(val); !got { return }
	i = int64(f)
	return
}

func dbp(opts ...interface{}) {
	if false {
		log.Println(opts...)
	}
}

func (st *IocopyStat) update() {
	st.Stat = "downloading"
	if st.Length > 0 {
		st.Per = float64(st.Size)/float64(st.Length)
	}
	st.Dur = time.Since(st.Begin)
	st.Perstr = PrettyPer(st.Per)
	st.Sizestr = PrettySize(st.Size)
	st.Lengthstr = PrettySize(st.Length)
	st.Speedstr = PrettySpeed(st.Speed)
	st.Durstr = PrettyDur(st.Dur)
}

func (st *IocopyStat) finish() {
	dur := float64(time.Since(st.Begin))/float64(time.Second)
	st.Speed = int64(float64(st.Size) / dur)
	st.Per = 1.0
	st.update()
}

func optIntv(opts ...interface{}) (intv time.Duration) {
	var hasintv bool
	hasintv, intv = optDuration("cbinterval=", opts)
	if !hasintv {
		intv = time.Second
	}
	return
}

func IoCopy(
	r io.ReadCloser,
	length int64,
	w io.Writer,
	opts ...interface{},
) (err error) {
	var st *IocopyStat
	var cb IocopyCb
	var ct *Control

	myw := &myWriter{Writer:w}

	for _, o := range opts {
		switch o.(type) {
		case *IocopyStat:
			st = o.(*IocopyStat)
		case *Control:
			ct = o.(*Control)
		case func(IocopyStat)error:
			cb = o.(func(IocopyStat)error)
		}
	}

	var rto time.Duration
	var hasrto bool
	hasrto, rto = optDuration("readtimeout=", opts)
	if !hasrto {
		hasrto, rto = optDuration("timeout=", opts)
	}

	var deadtm time.Time
	var deaddur time.Duration
	var hasdeadtm bool
	var hasdeaddur bool
	hasdeadtm, deadtm = optTime("deadline=", opts)
	if !hasdeadtm {
		hasdeaddur, deaddur = optDuration("deadline=", opts)
	}
	if hasdeaddur {
		hasdeadtm = true
		deadtm = time.Now().Add(deaddur)
	}

	intv := optIntv(opts)

	myw.hasmaxspeed, myw.maxspeed = optInt64("maxspeed=", opts)

	if st == nil {
		st = &IocopyStat{}
	}
	if ct == nil {
		ct = &Control{w:myw, st:st}
	}

	st.Begin = time.Now()
	st.Length = length

	done := make(chan int, 0)
	go func () {
		N := int64(16*1024)
		if myw.hasmaxspeed {
			N = myw.maxspeed
		}
		for {
			_, err = io.CopyN(myw, r, N)
			if err != nil {
				break
			}
		}
		if err == io.EOF {
			err = nil
		}
		if myw.hasmaxspeed {
			time.Sleep(myw.tm.Sub(time.Now()))
		}
		done <- 1
	}()

	defer r.Close()

	var n, idle int64

	for {
		myw.tm = time.Now().Add(intv)
		myw.curn = 0
		select {
		case <-done:
			st.Size = myw.n
			st.Speed = myw.n - n
			st.finish()
			if cb != nil { err = cb(ct.Stat()) 	}
			if err != nil { return }
			return
		case <-time.After(intv):
			if ct.stop {
				err = errors.New("user stops")
				return
			}
			st.Size = myw.n
			st.Speed = myw.n - n
			if cb != nil { err = cb(ct.Stat()) 	}
			if err != nil { return }
			if myw.n != n {
				n = myw.n
				idle = 0
			} else {
				idle++
			}
			if hasrto && time.Duration(idle)*intv > rto {
				err = errors.New("read timeout")
				return 
			}
			if hasdeadtm && time.Now().After(deadtm) {
				err = errors.New("deadline reached")
				return
			}
		}
	}

	return
}

type myWriter struct {
	io.Writer
	n int64
	hasmaxspeed bool
	maxspeed int64
	tm time.Time
	curn int64
}

func (m *myWriter) Write(b []byte) (n int, err error) {
	n, err = m.Writer.Write(b)
	m.n += int64(n)
	m.curn += int64(n)
	//fmt.Println(m.curn, m.maxspeed)
	if m.hasmaxspeed && m.curn >= m.maxspeed {
		time.Sleep(m.tm.Sub(time.Now()))
	}
	return
}

func Dial(url string, opts ...interface{}) (
	err error, r io.ReadCloser, length int64,
) {
	var req *http.Request
	var cb IocopyCb

	req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}

	var header http.Header
	for _, o := range opts {
		switch o.(type) {
		case http.Header:
			header = o.(http.Header)
		case func(IocopyStat)error:
			cb = o.(func(IocopyStat)error)
		}
	}

	hasdto, dto := optDuration("dialtimeout=", opts)
	if !hasdto {
		hasdto, dto = optDuration("timeout=", opts)
	}

	intv := optIntv(opts)

	if header == nil {
		header = http.Header {
			"Accept" : {"*/*"},
			"User-Agent" : {"curl/7.29.0"},
		}
	}
	req.Header = header

	var resp *http.Response

	tr := &http.Transport {
		Dial: func(network, addr string) (c net.Conn, e error) {
			c, e = net.Dial(network, addr)
			return
		},
	}
	client := &http.Client{
		Transport: tr,
	}

	callcb := func (st IocopyStat) bool {
		if cb != nil {
			err = cb(st)
		}
		return err != nil
	}

	done := make(chan int, 0)
	go func() {
		resp, err = client.Do(req)
		done <- 1
	}()

	tmstart := time.Now()

	if callcb(IocopyStat{Stat:"connecting"}) { return }
	for {
		select {
		case <-done:
			break
		case <-time.After(intv):
			if callcb(IocopyStat{Stat:"connecting"}) { return }
			if hasdto && time.Since(tmstart) > dto {
				err = errors.New("dial timeout")
				return
			}
		}
	}

	if err != nil {
		return
	}

	r = resp.Body
	length = resp.ContentLength
	return
}

func String(url string, opts ...interface{}) (err error, body string) {
	var b bytes.Buffer
	err = Write(url, &b, opts...)
	body = string(b.Bytes())
	return
}

func Bytes(url string, opts ...interface{}) (err error, body []byte) {
	var b bytes.Buffer
	err = Write(url, &b, opts...)
	body = b.Bytes()
	return
}

func File(url string, path string, opts ...interface{}) (err error) {
	var w io.WriteCloser
	w, err = os.Create(path)
	if err != nil {
		return
	}
	defer w.Close()
	err = Write(url, w, opts...)
	return
}

func Write(url string, w io.Writer, opts ...interface{}) (err error) {
	var r io.ReadCloser
	var length int64
	err, r, length = Dial(url, opts...)
	if err != nil {
		return
	}
	err = IoCopy(r, length, w, opts...)
	return
}

func PrettyDur(dur time.Duration) string {
	d := float64(dur)/float64(time.Second)
	if d < 60*60 {
		return fmt.Sprintf("%d:%.2d", int(d/60), int(d)%60)
	}
	return fmt.Sprintf("%d:%.2d:%.2d", int(d/3600), int(d/60)%60, int(d)%60)
}

func PrettyPer(f float64) string {
	return fmt.Sprintf("%.1f%%", f*100)
}

func PrettySize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(size)/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1fM", float64(size)/1024/1024)
	}
	return fmt.Sprintf("%.1fG", float64(size)/1024/1024/1024)
}

func PrettySpeed(s int64) string {
	return fmt.Sprintf("%s/s", PrettySize(s))
}

