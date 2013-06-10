
package main

import (
	"github.com/go-av/curl"
	"fmt"
	//"time"
)
		//"http://www.kernel.org/pub/linux/kernel/v3.x/linux-3.9.4.tar.xz",
		//"http://youku.com",
		//"http://dldir1.qq.com/qqfile/qq/QQ2013/2013Beta3/6565/QQ2013Beta3.exe",

func test1() {
  var st curl.IocopyStat
  curl.File(
  	"http://tk.wangyuehd.com/soft/skycn/WinRAR.exe_2.exe", 
  	"a.exe",
  	&st)
  fmt.Println("size=", st.Size, "average speed=", st.Speed)
}

func test2() {
	curl.File(
		"http://tk.wangyuehd.com/soft/skycn/WinRAR.exe_2.exe",
		"a.exe",
		func (st curl.IocopyStat) error {
			fmt.Println(st.Perstr, st.Sizestr, st.Lengthstr, st.Speedstr, st.Durstr)
			return nil
		},
	)
}

func test3() {
	curl.File(
		"http://tk.wangyuehd.com/soft/skycn/WinRAR.exe_2.exe",
		"a.exe",
		func (st curl.IocopyStat) error {
			fmt.Println(st.Perstr, st.Sizestr, st.Lengthstr, st.Speedstr, st.Durstr)
			return nil
		},
		"maxspeed=", 30*1000,
	)
}

func main() {
	test3()
}
