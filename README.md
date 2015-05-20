CURL-like library for golang (NOT libcurl binding)

* Custom HTTP method and header
* Monitoring download/upload progress and speed
* Pause/resume control

### Usage

		import "github.com/nareix/curl"

		req := curl.New("https://kernel.org/pub/linux/kernel/v4.x/linux-4.0.4.tar.xz")

		req.Method("POST")   // "PUT"/"POST"/"DELETE" ...

		req.Header("MyHeader", "Value")     // Custom header
		req.Headers = http.Header {         // Set all HTTP Headers
			"User-Agent": {"mycurl/1.0"},
		}

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

		res, err := req.Do()

		log.Println(res.Body)                        // Body in string
		log.Println(res.StatusCode)                  // HTTP Status Code: 200,404,302 etc
		log.Println(res.DownloadStatus.AverageSpeed) // Average speed

