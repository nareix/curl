package curl

import (
	"bytes"
	_ "errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

/*
var (
	ErrConnectTimeout     = errors.New("Connecting timeout")
	ErrDNSResolve         = errors.New("DNS resolve failed")
	ErrDownloadTimeout    = errors.New("Download timeout")
	ErrCreateDownloadFile = errors.New("Create download file error")
)
*/

type Request struct {
	url     string
	method  string
	Headers http.Header
	body    string

	bodyUploadEntry    *uploadEntry
	mpartUploadEntries []uploadEntry

	uploadMonitor   *Monitor
	downloadMonitor *Monitor

	downloadToFile string

	stat       int
	progressCb MonitorProgressCb

	Upload   ProgressStatus
	Download ProgressStatus

	reqbodyTracer io.Writer
	reqTracer     io.Writer

	progressCloseEvent chan int
	progressInterval   time.Duration

	dialTimeout     time.Duration
	transferTimeout time.Duration
}

func New(url string) *Request {
	req := &Request{
		url: url,
		Headers: http.Header{
			"User-Agent": {"curl/7.29.0"},
		},
	}

	req.uploadMonitor = &Monitor{ioTracker: &ioTracker{}}
	req.downloadMonitor = &Monitor{ioTracker: &ioTracker{}}

	return req
}

func Get(url string) *Request {
	return New(url).Method("GET")
}

func Post(url string) *Request {
	return New(url).Method("POST")
}

func (req *Request) Method(method string) *Request {
	req.method = method
	return req
}

func (req *Request) Header(k, v string) *Request {
	req.Headers[k] = []string{v}
	return req
}

func (req *Request) UserAgent(v string) *Request {
	return req.Header("User-Agent", v)
}

func (req *Request) BodyString(v string) *Request {
	req.body = v
	return req
}

func (req *Request) TraceRequestBody(w io.Writer) *Request {
	req.reqbodyTracer = w
	return req
}

func (req *Request) TraceRequest(w io.Writer) *Request {
	req.reqTracer = w
	return req
}

type uploadEntry struct {
	filename string
	filepath string
}

func (e uploadEntry) getFileReader() (reader io.Reader, length int64, err error) {
	var file *os.File
	var fileInfo os.FileInfo

	if file, err = os.Open(e.filepath); err != nil {
		return
	}
	if fileInfo, err = file.Stat(); err != nil {
		return
	}

	length = fileInfo.Size()
	reader = file

	return
}

func (req *Request) BodyUploadFile(filename, filepath string) *Request {
	req.bodyUploadEntry = &uploadEntry{
		filename: filename,
		filepath: filepath,
	}
	return req
}

func (req *Request) SaveToFile(filepath string) *Request {
	req.downloadToFile = filepath
	return req
}

type ioTracker struct {
	io.Reader
	io.Writer
	Bytes         int64
	whenDataComes func()
	pausedCond    *sync.Cond
	pausedLock    *sync.Mutex
	paused        bool
	stop          bool
}

func (tracker *ioTracker) newPauseCond() {
	tracker.pausedLock = &sync.Mutex{}
	tracker.pausedCond = sync.NewCond(tracker.pausedLock)
}

func (tracker *ioTracker) WhenDataComes(cb func()) {
	tracker.whenDataComes = cb
}

func (tracker *ioTracker) preIO() (err error) {
	if tracker.stop {
		return io.EOF
	}
	if tracker.whenDataComes != nil {
		tracker.whenDataComes()
		tracker.whenDataComes = nil
	}
	if tracker.pausedCond != nil {
		tracker.pausedCond.L.Lock()
		if tracker.paused {
			tracker.pausedCond.Wait()
		}
		tracker.pausedCond.L.Unlock()
	}
	return
}

func (tracker *ioTracker) Write(p []byte) (n int, err error) {
	if err = tracker.preIO(); err != nil {
		return
	}
	n, err = tracker.Writer.Write(p)
	tracker.Bytes += int64(n)
	return
}

func (tracker *ioTracker) Read(p []byte) (n int, err error) {
	if err = tracker.preIO(); err != nil {
		return
	}
	n, err = tracker.Reader.Read(p)
	tracker.Bytes += int64(n)
	return
}

func (req *Request) getBodyUploadReader() (body io.Reader, length int64, err error) {
	return req.bodyUploadEntry.getFileReader()
}

func (req *Request) getMpartUploadReaderAndContentType() (body io.Reader, contentType string) {
	pReader, pWriter := io.Pipe()
	mpart := multipart.NewWriter(pWriter)

	go func() {
		defer pWriter.Close()
		defer mpart.Close()

		entry := req.mpartUploadEntries[0]

		part, err := mpart.CreateFormFile(entry.filename, entry.filepath)
		if err != nil {
			return
		}

		var f io.ReadCloser
		if f, err = os.Open(entry.filepath); err != nil {
			return
		}
		defer f.Close()

		_, err = io.Copy(part, f)
	}()

	return pReader, mpart.FormDataContentType()
}

const (
	Connecting = iota
	Uploading
	Downloading
	Closed
)

type ProgressStatus struct {
	Stat          int
	ContentLength int64
	Size          int64
	Percent       float32
	AverageSpeed  int64
	Speed         int64
	MaxSpeed      int64
	TimeElapsed   time.Duration
	Paused        bool
}

type Monitor struct {
	ioTracker     *ioTracker
	contentLength int64
	timeStarted   time.Time
	finished      bool
}

type MonitorProgressCb func(p ProgressStatus)

type snapShot struct {
	bytes int64
	time  time.Time
}

func (mon *Monitor) currentProgressSnapshot() (shot snapShot) {
	shot.bytes = mon.ioTracker.Bytes
	shot.time = time.Now()
	return
}

func (mon *Monitor) getProgressStatus(lastSnapshot *snapShot) (stat ProgressStatus) {
	now := time.Now()

	stat.TimeElapsed = now.Sub(mon.timeStarted)
	stat.ContentLength = mon.contentLength
	stat.Size = mon.ioTracker.Bytes

	if lastSnapshot != nil {
		stat.Speed = (stat.Size - lastSnapshot.bytes) * 1000 / (int64(now.Sub(lastSnapshot.time)) / int64(time.Millisecond))
	}

	stat.AverageSpeed = stat.Size * 1000 / (int64(now.Sub(mon.timeStarted)) / int64(time.Millisecond))

	if stat.ContentLength > 0 {
		stat.Percent = float32(stat.Size) / float32(stat.ContentLength)
	}

	stat.Paused = mon.ioTracker.paused

	return
}

type traceConn struct {
	net.Conn
	io.Writer
}

func (conn traceConn) Write(b []byte) (n int, err error) {
	if conn.Writer != nil {
		conn.Writer.Write(b)
	}
	return conn.Conn.Write(b)
}

func (req *Request) MonitorDownload() (mon *Monitor) {
	mon = &Monitor{}
	req.downloadMonitor = mon
	return
}

func (req *Request) MonitorUpload() (mon *Monitor) {
	mon = &Monitor{}
	req.uploadMonitor = mon
	return
}

func (req *Request) enterStat(stat int) {
	if req.progressCb != nil {
		req.progressCb(ProgressStatus{Stat: stat})
	}

	progressCall := func(stat int, mon *Monitor) {
		var shot snapShot
		if mon != nil {
			shot = mon.currentProgressSnapshot()
		}

		go func() {
			for {
				select {
				case <-time.After(req.progressInterval):
					ps := ProgressStatus{}
					if mon != nil {
						ps = mon.getProgressStatus(&shot)
						shot = mon.currentProgressSnapshot()
					}
					ps.Stat = stat
					if req.progressCb != nil {
						req.progressCb(ps)
					}

				case <-req.progressCloseEvent:
					return
				}
			}
		}()
	}

	switch stat {
	case Connecting:
		req.progressCloseEvent = make(chan int)
		progressCall(stat, nil)

	case Uploading:
		req.progressCloseEvent <- 0
		progressCall(stat, req.uploadMonitor)

	case Downloading:
		req.progressCloseEvent <- 0
		progressCall(stat, req.downloadMonitor)

	case Closed:
		req.progressCloseEvent <- 0
	}
}

func (req *Request) Progress(cb MonitorProgressCb, interval time.Duration) *Request {
	req.progressCb = cb
	req.progressInterval = interval
	return req
}

func (req *Request) DialTimeout(timeout time.Duration) *Request {
	req.dialTimeout = timeout
	return req
}

func (req *Request) Timeout(timeout time.Duration) *Request {
	req.transferTimeout = timeout
	return req
}

type Control struct {
	ioTracker *ioTracker
}

func (ctrl *Control) Stop() {
	ctrl.ioTracker.stop = true
}

func (ctrl *Control) Resume() {
	ctrl.ioTracker.paused = false
	ctrl.ioTracker.pausedCond.Broadcast()
}

func (ctrl *Control) Pause() {
	ctrl.ioTracker.paused = true
	ctrl.ioTracker.pausedCond.Broadcast()
}

func (req *Request) ControlDownload() (ctrl *Control) {
	ctrl = &Control{
		ioTracker: req.downloadMonitor.ioTracker,
	}
	ctrl.ioTracker.newPauseCond()
	return
}

func (req *Request) Do() (res Response, err error) {
	var httpreq *http.Request
	var httpres *http.Response
	var reqbody io.Reader
	var reqbodyLength int64

	if len(req.mpartUploadEntries) > 0 {
		var contentType string
		reqbody, contentType = req.getMpartUploadReaderAndContentType()
		req.Headers["Content-Type"] = []string{contentType}
		req.method = "POST"
	} else if req.bodyUploadEntry != nil {
		if reqbody, reqbodyLength, err = req.getBodyUploadReader(); err != nil {
			return
		}
	} else {
		reqbody = strings.NewReader(req.body)
		reqbodyLength = int64(len(req.body))
	}

	if req.reqbodyTracer != nil && reqbody != nil {
		reqbody = io.TeeReader(reqbody, req.reqbodyTracer)
	}

	req.uploadMonitor.contentLength = reqbodyLength
	req.uploadMonitor.timeStarted = time.Now()
	req.downloadMonitor.timeStarted = time.Now()

	req.uploadMonitor.ioTracker.Reader = reqbody
	reqbody = req.uploadMonitor.ioTracker

	req.enterStat(Connecting)

	req.uploadMonitor.ioTracker.WhenDataComes(func() {
		req.enterStat(Uploading)
	})

	req.downloadMonitor.ioTracker.WhenDataComes(func() {
		req.enterStat(Downloading)
	})

	defer req.enterStat(Closed)

	if httpreq, err = http.NewRequest(req.method, req.url, reqbody); err != nil {
		return
	}
	httpreq.Header = req.Headers
	httpreq.ContentLength = reqbodyLength

	httptrans := &http.Transport{
		Dial: func(network, addr string) (conn net.Conn, err error) {
			if req.dialTimeout != time.Duration(0) {
				conn, err = net.DialTimeout(network, addr, req.dialTimeout)
			} else {
				conn, err = net.Dial(network, addr)
			}
			if conn != nil {
				conn = traceConn{Conn: conn, Writer: req.reqTracer}
				if req.transferTimeout != time.Duration(0) {
					conn.SetDeadline(time.Now().Add(req.transferTimeout))
				}
			}
			return
		},
		DisableCompression: true,
	}

	httpclient := http.Client{
		Transport: httptrans,
	}

	if httpres, err = httpclient.Do(httpreq); err != nil {
		return
	}
	defer httpres.Body.Close()

	var resbodyBuffer *bytes.Buffer
	var resbody io.Writer

	if req.downloadToFile != "" {
		var f *os.File
		if f, err = os.Create(req.downloadToFile); err != nil {
			return
		}
		resbody = f
	} else {
		resbodyBuffer = &bytes.Buffer{}
		resbody = resbodyBuffer
	}

	req.downloadMonitor.contentLength = httpres.ContentLength
	req.downloadMonitor.ioTracker.Writer = resbody
	resbody = req.downloadMonitor.ioTracker

	res.StatusCode = httpres.StatusCode
	res.Headers = httpres.Header
	res.HttpResponse = httpres

	if _, err = io.Copy(resbody, httpres.Body); err != nil {
		return
	}

	if resbodyBuffer != nil {
		res.Body = resbodyBuffer.String()
	}

	req.stat = Closed

	req.uploadMonitor.finished = true
	req.downloadMonitor.finished = true
	res.UploadStatus = req.uploadMonitor.getProgressStatus(nil)
	res.DownloadStatus = req.downloadMonitor.getProgressStatus(nil)

	return
}

type Response struct {
	HttpResponse   *http.Response
	StatusCode     int
	Headers        http.Header
	Body           string
	UploadStatus   ProgressStatus
	DownloadStatus ProgressStatus
}

func PrettySizeString(size int64) string {
	unit := "B"
	fsize := float64(size)

	if fsize > 1024 {
		unit = "K"
		fsize /= 1024
	}
	if fsize > 1024 {
		unit = "M"
		fsize /= 1024
	}
	if fsize > 1024 {
		unit = "G"
		fsize /= 1024
	}

	return fmt.Sprintf("%.1f%s", fsize, unit)
}

func PrettySpeedString(speed int64) string {
	return fmt.Sprintf("%s/s", PrettySizeString(speed))
}
