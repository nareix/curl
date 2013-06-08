
package main

import (
	"github.com/go-av/curl"
	"log"
	//"time"
)

func main() {
	err := curl.CurlFile(
		//"http://www.kernel.org/pub/linux/kernel/v3.x/linux-3.9.4.tar.xz",
		//"http://youku.com",
		//"http://dldir1.qq.com/qqfile/qq/QQ2013/2013Beta3/6565/QQ2013Beta3.exe",
		"http://tk.wangyuehd.com/soft/skycn/WinRAR.exe_2.exe",
		"qq.exe",
		func (st curl.IocopyStat) error {
			log.Println(st.Perstr, st.Sizestr, st.Lengthstr, st.Speedstr, st.Durstr)
			return nil
		},
		"cbinterval=1",
		//"readtimeout=", 0.1,
	)
	log.Println(err)
}
