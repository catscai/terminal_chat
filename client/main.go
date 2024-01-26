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

func main() {
	logdir := *flag.String("logdir", "./logs/", "日志输出目录")
	sendChanLen := *flag.Int("sendChanLen", 300, "发送管道长度")
	addr := *flag.String("addr", "127.0.0.1:2233", "服务器地址")
	Logger = clog.NewZapLogger("debug", AppName,
		logdir, 128, 7, 30, false, false)
	GCli = client.NewClient(Logger, &msg.DefaultDataPack{}, &msg.DefaultHeaderOperator{}, uint32(sendChanLen), time.Second*3)
	if err := GCli.Connection("tcp", addr, time.Second*3); err != nil {
		fmt.Println(err)
		return
	}
	GCli.SetProcess(HandlerNotify)
	err := http.ListenAndServe("127.0.0.1:2323", &ClientHttpHandler{})
	panic(err)
}
