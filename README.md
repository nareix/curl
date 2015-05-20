CURL-like library for golang (NOT libcurl binding)

* Custom HTTP method and header
* Monitoring download progress and speed
* Pause/resume control

### Usage

```go
import "github.com/nareix/curl"

req := curl.New("https://kernel.org/pub/linux/kernel/v4.x/linux-4.0.4.tar.xz")

req.Method("POST")   // can be "PUT"/"POST"/"DELETE" ...

req.Header("MyHeader", "Value")     // Custom header
req.Headers = http.Header {         // Custom all headers
	"User-Agent": {"mycurl/1.0"},
}

ctrl := req.ControlDownload()       // Download control
go func () {
	// control functions are thread safe
	ctrl.Stop()   // Stop download
	ctrl.Pause()  // Pause download
	ctrl.Resume() // Resume download
}()

req.DialTimeout(time.Second * 10)   // TCP Connection Timeout
req.Timeout(time.Second * 30)       // Download Timeout

// Print progress status per one second
req.Progress(func (p curl.ProgressStatus) {
	log.Println(
		"Stat", p.Stat,   // one of curl.Connecting / curl.Downloading / curl.Closed
		"speed", curl.PrettySpeedString(p.Speed),
		"len", curl.PrettySizeString(p.ContentLength),
		"got", curl.PrettySizeString(p.Size),
		"percent", p.Percent,
		"paused", p.Paused,
	)
}, time.Second)
/*
2015/05/20 15:34:15 Stat 2 speed 0.0B/s len 78.5M got 0.0B percent 0 paused true
2015/05/20 15:34:16 Stat 2 speed 0.0B/s len 78.5M got 0.0B percent 0 paused true
2015/05/20 15:34:16 Stat 2 speed 394.1K/s len 78.5M got 197.5K percent 0.0024564497 paused false
2015/05/20 15:34:17 Stat 2 speed 87.8K/s len 78.5M got 241.5K percent 0.0030038392 paused false
2015/05/20 15:34:17 Stat 2 speed 79.8K/s len 78.5M got 281.5K percent 0.003501466 paused false
2015/05/20 15:34:18 Stat 2 speed 63.9K/s len 78.5M got 313.5K percent 0.0038995675 paused false 
*/

res, err := req.Do()

res.HttpResponse                             // related *http.Response struct
log.Println(res.Body)                        // Body in string
log.Println(res.StatusCode)                  // HTTP Status Code: 200,404,302 etc
log.Println(res.Hearders)                    // Reponse headers
log.Println(res.DownloadStatus.AverageSpeed) // Average speed
```
