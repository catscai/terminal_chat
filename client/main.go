package main

import (
	"flag"
	"fmt"
	"github.com/catscai/ccat/clog"
	"github.com/catscai/ccat/impl/client"
	"github.com/catscai/ccat/impl/msg"
	daemon2 "github.com/sevlyar/go-daemon"
	"log"
	"net/http"
	"os"
	"time"
)

var Logger clog.ICatLog

const AppName = "terminal-chat-client"

var GCli *client.Client
var SerAddr string
var SendChanLen int
var Cmd string
var LogDir string
var CliAddr string
var chatFile string

func main() {
	flag.StringVar(&LogDir, "logdir", "./logs/", "日志输出目录")
	flag.IntVar(&SendChanLen, "sendChanLen", 300, "发送管道长度")
	flag.StringVar(&SerAddr, "addr", "", "服务器地址")
	if len(SerAddr) == 0 {
		SerAddr = "127.0.0.1:2233"
	}
	flag.StringVar(&chatFile, "chatOut", "", "聊天内容重定向输出,进程后台运行时有效")
	var background bool
	flag.BoolVar(&background, "d", false, "后台启动运行")
	CliAddr = "127.0.0.1:22330"
	flag.Parse()

	if background {
		if len(chatFile) > 0 {
			f, err := os.OpenFile(chatFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				fmt.Println(err)
				return
			}
			os.Stdout = f
			os.Stderr = f
		}
		context := new(daemon2.Context)
		child, _ := context.Reborn()
		if child != nil {

		} else {
			defer func() {
				if err := context.Release(); err != nil {
					log.Printf("Unable to release pid-file: %s", err.Error())
				}
			}()
			StartServer()
		}

	} else {
		StartServer()
	}

}

func StartServer() {

	Logger = clog.NewZapLogger("debug", AppName,
		LogDir, 128, 7, 30, false, false)

	GCli = client.NewClient(Logger, &msg.DefaultDataPack{}, &msg.DefaultHeaderOperator{}, uint32(SendChanLen), time.Second*3)
	if err := GCli.Connection("tcp", SerAddr, time.Second*3); err != nil {
		fmt.Println(err)
		return
	}
	msgPrintStatus(0, 0, "", "连接服务器")
	GCli.SetProcess(HandlerNotify)
	http.Handle("/api", &ClientHttpHandler{})
	err := http.ListenAndServe(CliAddr, nil)
	panic(err)
}
