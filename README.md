Curl library for golang (: WITHOUT libcurl.so :)
Just using "net/http"
====

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
