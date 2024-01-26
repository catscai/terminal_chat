package main

import (
	"github.com/catscai/ccat/server"
	"github.com/catscai/terminal_chat/service/process"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const AppName = "terminal-chat"

func main() {
	process.GSer = server.GetServer(AppName)
	process.RegisterHandler(process.GSer.GetDispatcher())
	process.GConnM = process.GSer.GetConnManager()
	process.GPackOp = process.GSer.GetHeaderOperator()
	var err error
	process.GLevelDB, err = leveldb.OpenFile("./db/", &opt.Options{})
	if err != nil {
		panic(err)
	}
	process.OnInit()
	process.GSer.Run()
}
