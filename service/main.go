package main

import (
	"github.com/catscai/ccat/iface"
	"github.com/catscai/ccat/server"
	"github.com/catscai/terminal_chat/service/process"
)

const AppName = "terminal-chat"

var GSer iface.IServer
var GConnM iface.IConnManager

func main() {
	GSer = server.GetServer(AppName)
	process.RegisterHandler(GSer.GetDispatcher())
	GConnM = GSer.GetConnManager()
	GSer.Run()
}
