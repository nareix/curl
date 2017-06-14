package curl

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	c := New("http://opvbhjo1o.bkt.clouddn.com/2017/6/13/video_2017_06_13.mp4", true)
	c.SaveToFile("test.mp4")
	c.Progress(func(p ProgressStatus) {
		// 打印下载状态
		fmt.Println(
			"Stat", p.Stat, // one of curl.Connecting / curl.Downloading / curl.Closed
			"speed", PrettySpeedString(p.Speed),
			"len", PrettySizeString(p.ContentLength),
			"got", PrettySizeString(p.Size),
			"percent", p.Percent,
			"paused", p.Paused,
		)
	}, time.Second)
	res, err := c.Do()
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("response:", res)
	}

	fmt.Println(os.Stat("test.mp4"))
}
