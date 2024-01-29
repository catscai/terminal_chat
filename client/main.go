package main

import (
	"flag"
	"fmt"
	"github.com/catscai/ccat/clog"
	"github.com/catscai/ccat/impl/client"
	"github.com/catscai/ccat/impl/msg"
	"net/http"
	"time"
)

var Logger clog.ICatLog

const AppName = "terminal-chat-client"

var GCli *client.Client
var SerAddr string
var SendChanLen int

func main() {
	var logdir, cliAddr string
	flag.StringVar(&logdir, "logdir", "./logs/", "日志输出目录")
	flag.IntVar(&SendChanLen, "sendChanLen", 300, "发送管道长度")
	flag.StringVar(&SerAddr, "addr", "127.0.0.1:2233", "服务器地址")
	cliAddr = "127.0.0.1:22331"
	flag.Parse()
	Logger = clog.NewZapLogger("debug", AppName,
		logdir, 128, 7, 30, false, false)

	GCli = client.NewClient(Logger, &msg.DefaultDataPack{}, &msg.DefaultHeaderOperator{}, uint32(SendChanLen), time.Second*3)
	if err := GCli.Connection("tcp", SerAddr, time.Second*3); err != nil {
		fmt.Println(err)
		return
	}
	msgPrintStatus(0, 0, "", "连接服务器")
	GCli.SetProcess(HandlerNotify)
	http.Handle("/api", &ClientHttpHandler{})
	err := http.ListenAndServe(cliAddr, nil)
	panic(err)
}
