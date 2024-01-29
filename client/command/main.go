package main

import (
	"flag"
	"fmt"
	"github.com/catscai/terminal_chat/pack"
	"github.com/catscai/terminal_chat/pb"
	"net/http"
	"strconv"
)

type LineParams struct {
	Login           bool // -l
	Register        bool // -r
	SubscribePerson bool // -sp
	JoinGroup       bool // -jg
	CreateGroup     bool // -c
	Cancel          bool // -n
	CancelALl       bool // -na
	Send            bool // -s
	ReConn          bool // -reset

	Own     int64  // -u
	Peers   string // -ps
	Peer    int64  // -p
	Group   int64  // -g
	Passwd  string // -passwd
	Content string // -t

	Name       string // -name
	Email      string // -email
	VerifyCode int64  // -verify
}

var LP LineParams

func ParseFlag() {
	flag.BoolVar(&LP.Register, "r", false, "用户注册,需要指定name,email,verify选项")
	flag.StringVar(&LP.Name, "name", "terminal", "用户名,不能过长")
	flag.StringVar(&LP.Email, "email", "", "邮箱,用户安全验证,账号找回")
	flag.Int64Var(&LP.VerifyCode, "verify", 0, "验证码,创建组时需要指定进入验证码")

	flag.BoolVar(&LP.Login, "l", false, "用户登陆,需要指定u,passwd信息")
	flag.Int64Var(&LP.Own, "u", 0, "用户ID")
	flag.StringVar(&LP.Passwd, "passwd", "", "用户密码")
	flag.BoolVar(&LP.SubscribePerson, "sp", false, "订阅用户,需要指定p或ps选项")
	flag.StringVar(&LP.Peers, "ps", "", "指定peers,用','分隔多个peer")
	flag.Int64Var(&LP.Peer, "p", 0, "指定peer")
	flag.BoolVar(&LP.JoinGroup, "jg", false, "加入讨论组,需要指定g,verify选项")
	flag.Int64Var(&LP.Group, "g", 0, "讨论组ID")
	flag.BoolVar(&LP.CreateGroup, "cg", false, "创建讨论组,需要指定name,verify选项")
	flag.BoolVar(&LP.Cancel, "n", false, "取消订阅,可以指定g,p选项用于取消group和peer的消息订阅")
	flag.BoolVar(&LP.CancelALl, "na", false, "取消所有订阅项,包含peer和group")
	flag.BoolVar(&LP.Send, "s", false, "发送消息,需要指定p或g,用于发送消息给peer或group")
	flag.StringVar(&LP.Content, "t", "", "发送的消息内容")
	flag.BoolVar(&LP.ReConn, "reset", false, "重新连接,需要重新登陆")

	flag.Parse()
}

func check() (bool, string) {
	opts := make([]bool, 0, 10)
	opts = append(opts, LP.Register, LP.Login, LP.CreateGroup, LP.JoinGroup, LP.Send, LP.CancelALl, LP.SubscribePerson, LP.Cancel, LP.ReConn)
	optMap := make(map[bool]int)
	for _, opt := range opts {
		optMap[opt]++
	}
	if cnt, ok := optMap[true]; !ok || cnt > 1 {
		return false, "主选项缺失或冲突"
	}

	if LP.Register {
		if len(LP.Name) == 0 || len(LP.Email) == 0 {
			return false, "注册缺少Name或Email"
		}
	}
	if LP.Login {
		if LP.Own == 0 || len(LP.Passwd) == 0 {
			return false, "登陆信息缺少"
		}
	}
	if LP.SubscribePerson {
		if len(LP.Peers) == 0 && LP.Peer == 0 {
			return false, "订阅Peer缺少信息"
		}
	}
	if LP.CreateGroup {
		if LP.VerifyCode == 0 || len(LP.Name) == 0 {
			return false, "创建讨论组缺少数据信息"
		}
	}
	if LP.JoinGroup {
		if LP.Group == 0 || LP.VerifyCode == 0 {
			return false, "加入讨论组缺少数据信息"
		}
	}
	if LP.Cancel {
		if LP.Peer == 0 && LP.Group == 0 && len(LP.Peers) == 0 {
			return false, "取消订阅时需要指定peer或group"
		}
		if (LP.Peer > 0 || len(LP.Peers) > 0) && LP.Group > 0 {
			return false, "取消订阅时指定peer,group不能同时指定"
		}
	}
	if LP.Send {
		if LP.Peer == 0 && LP.Group == 0 {
			return false, "发送消息时需要指定peer或group"
		}
		if LP.Peer > 0 && LP.Group > 0 {
			return false, "发送消息时指定的peer,group不能同时指定"
		}
	}

	return true, ""
}

func deal() error {
	var err error
	var header http.Header

	getRes := func() (int32, string) {
		codeStr := header.Get("code")
		desc := header.Get("desc")
		code, _ := strconv.ParseInt(codeStr, 10, 32)
		return int32(code), desc
	}

	if LP.ReConn {
		err, header = SendCommand(pack.ReConn, 0, "", "", "", "", "", 0, 0, 0, -1)
		if err != nil {
			return err
		}
		return nil
	}

	if LP.Register {
		err, header = SendCommand(pack.Register, 0, LP.Name, LP.Passwd, LP.Email, "", "", LP.VerifyCode, 0, 0, -1)
		if err != nil {
			return err
		}

		code, desc := getRes()
		if code != pb.CodeOK {
			fmt.Println(code, desc)
		} else {
			fmt.Println(desc, "账号："+header.Get("own"))
		}

		return nil
	}

	if LP.Login {
		err, header = SendCommand(pack.Login, LP.Own, "", LP.Passwd, "", "", "", 0, 0, 0, -1)
	}

	if LP.SubscribePerson {
		var op int
		if LP.Cancel {
			op = 1
		}
		if len(LP.Peers) == 0 {
			LP.Peers = fmt.Sprintf("%d", LP.Peer)
		} else {
			LP.Peers += fmt.Sprintf(",%d", LP.Peer)
		}
		err, header = SendCommand(pack.SubscribePersonal, 0, "", "", "", LP.Peers, "", 0, 0, 0, op)
	}

	if LP.CreateGroup {
		err, header = SendCommand(pack.CreateGroup, 0, LP.Name, "", "", "", "", LP.VerifyCode, 0, 0, -1)
	}

	if LP.JoinGroup {
		var op int
		if LP.Cancel {
			op = 1
		}
		err, header = SendCommand(pack.Join, 0, "", "", "", "", "", LP.VerifyCode, 0, LP.Group, op)
	}

	if LP.CancelALl {
		err, header = SendCommand(pack.CancelAllSubscribe, 0, "", "", "", "", "", 0, 0, 0, -1)
	}

	if LP.Send {
		if LP.Peer > 0 {
			err, header = SendCommand(pack.SendToPersonal, 0, "", "", "", "", LP.Content, 0, LP.Peer, 0, -1)
		} else {
			err, header = SendCommand(pack.SendToGroup, 0, "", "", "", "", LP.Content, 0, 0, LP.Group, -1)
		}
	}

	if LP.Cancel {
		if len(LP.Peers) == 0 {
			LP.Peers = fmt.Sprintf("%d", LP.Peer)
		} else {
			LP.Peers += fmt.Sprintf(",%d", LP.Peer)
		}
		if len(LP.Peers) > 0 {
			err, header = SendCommand(pack.SubscribePersonal, 0, "", "", "", LP.Peers, "", 0, 0, 0, 1)
		} else {
			err, header = SendCommand(pack.Join, 0, "", "", "", "", "", 0, 0, LP.Group, 1)
		}
	}

	if err != nil {
		return err
	}
	code, desc := getRes()
	fmt.Println(code, desc)
	return nil
}

func main() {
	ParseFlag()

	if ok, msg := check(); !ok {
		fmt.Println("Error:" + msg)
		return
	}

	if err := deal(); err != nil {
		fmt.Println(err)
	}

}
