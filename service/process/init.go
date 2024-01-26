package process

import (
	"github.com/catscai/ccat/iface"
	"github.com/catscai/ccat/iface/imsg"
	"github.com/catscai/terminal_chat/service/taskgroup"
	"github.com/syndtr/goleveldb/leveldb"
)

var GSer iface.IServer
var GConnM iface.IConnManager
var GLevelDB *leveldb.DB
var GAsyncIoPool *taskgroup.TaskGroup
var GPackOp imsg.IHeaderOperator

func OnInit() {
	GAsyncIoPool = taskgroup.NewTaskGroup(1000, 200)
	OnInitUtil()
}
