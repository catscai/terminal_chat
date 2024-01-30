package main

import (
	"encoding/json"
	"fmt"
	"github.com/catscai/ccat/iface/imsg"
	"github.com/catscai/ccat/impl/client"
	"github.com/catscai/ccat/impl/msg"
	"github.com/catscai/terminal_chat/pack"
	"github.com/catscai/terminal_chat/pb"
	"github.com/catscai/terminal_chat/pb/src/allpb"
	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ClientHttpHandler struct {
	IsLogin   bool
	Own       int64
	Name      string
	LoginTime int64

	ColorCode      string
	JoinGroup      sync.Map // 加入的组 group -> *JoinGroupItem
	SubscribePeers sync.Map // 订阅者 peer -> *PeerInfo
	Followers      sync.Map // 关注者 peer -> *PeerInfo
}

func (c *ClientHttpHandler) reset() {
	c.IsLogin = false
	c.Own = 0
	c.Name = ""
	c.LoginTime = 0
	c.ColorCode = ""
	c.JoinGroup.Range(func(key, value interface{}) bool {
		c.JoinGroup.Delete(key)
		return true
	})
}

type PeerInfo struct {
	Peer      int64  `json:"peer"`
	Name      string `json:"name"`
	TimeStamp string `json:"time"`
}

type MemberInfo struct {
	ID        int64
	ColorCode string
}

type JoinGroupItem struct {
	Group      int64  `json:"group"`
	Name       string `json:"name"`
	VerifyCode int64  `json:"verify"`
	CreateTime string `json:"createTime"`
	JoinTime   string `json:"joinTime"`
}

type OwnStatus struct {
	Status     string           `json:"status"`
	Own        int64            `json:"own"`
	Name       string           `json:"name"`
	LoginTime  string           `json:"loginTime"`
	Groups     []*JoinGroupItem `json:"groups"`
	Subscribes []*PeerInfo      `json:"subscribes"`
	Followers  []*PeerInfo      `json:"followers"`
}

var members sync.Map //int64 -> *MemberInfo

func Pack(packType uint32, session uint64, message proto.Message) imsg.IHeaderPack {
	data, _ := proto.Marshal(message)
	return GCli.HeaderOperator.Full(packType, data, &msg.DefaultHeader{SessionID: session})
}

var sessIDCr uint64 = 10000

func GetSessionID() uint64 {
	return atomic.AddUint64(&sessIDCr, 1)
}

func NetSend(reqType, rspType uint32, reqMsg, rspMsg proto.Message, session uint64) error {
	pkg := Pack(reqType, session, reqMsg)
	resPkg := &msg.DefaultHeader{}
	if err := GCli.Send(pkg, resPkg); err != nil {
		return err
	}
	if resPkg.PackType != rspType {
		return fmt.Errorf("包类型不匹配")
	}
	return proto.Unmarshal(resPkg.Data, rspMsg)
}

func (c *ClientHttpHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	Logger.Debug("ServeHTTP begin", zap.Any("header", req.Header))
	command := req.Header.Get("command")

	setResErr := func(code int, desc string) {
		writer.Header().Set("code", fmt.Sprintf("%d", code))
		writer.Header().Set("desc", desc)
	}
	setResHeader := func(key string, val interface{}) {
		writer.Header().Set(key, fmt.Sprintf("%v", val))
	}
	setResErr(int(pb.CodeOK), "成功")

	switch command {
	case pack.Register:
		email := req.Header.Get("email")
		name := req.Header.Get("name")
		passwd := req.Header.Get("passwd")
		verifyCodeStr := req.Header.Get("verifyCode")
		if len(email) > 20 || len(name) > 10 || len(passwd) > 20 {
			setResErr(1, "名字或密码长度过长")
			return
		}
		verifyCode, _ := strconv.ParseInt(verifyCodeStr, 10, 64)
		msgRq := &allpb.RegisterRQ{
			Email:      &email,
			Name:       &name,
			Passwd:     &passwd,
			VerifyCode: proto.Int32(int32(verifyCode)),
		}
		msgRs := &allpb.RegisterRS{}
		if err := NetSend(pb.PackRegisterRQ, pb.PackRegisterRS, msgRq, msgRs, GetSessionID()); err != nil {
			setResErr(3, "网络发送失败")
			return
		}
		if msgRs.GetErr().GetCode() != pb.CodeOK {
			setResErr(int(msgRs.GetErr().GetCode()), msgRs.GetErr().GetMsg())
			return
		}
		setResHeader("own", msgRs.GetOwn())
	case pack.Login:
		ownStr := req.Header.Get("own")
		passwd := req.Header.Get("passwd")
		if len(ownStr) == 0 || len(passwd) == 0 {
			setResErr(1, "用户ID或密码为空")
			return
		}
		own, _ := strconv.ParseInt(ownStr, 10, 64)
		msgRq := &allpb.LoginRQ{
			Own:    &own,
			Passwd: &passwd,
		}
		msgRs := &allpb.LoginRS{}
		if err := NetSend(pb.PackLoginRQ, pb.PackLoginRS, msgRq, msgRs, GetSessionID()); err != nil {
			msgPrintStatus(1, own, "", "登陆")
			setResErr(3, "网络发送失败")
			return
		}
		if msgRs.GetErr().GetCode() != pb.CodeOK {
			setResErr(int(msgRs.GetErr().GetCode()), msgRs.GetErr().GetMsg())
			msgPrintStatus(1, own, "", "登陆")
			return
		}
		// 登陆成功，将信息记录下来
		c.Own = own
		c.Name = msgRs.GetName()
		c.LoginTime = time.Now().Unix()
		c.IsLogin = true
		c.ColorCode = randomTextColor()

		members.Range(func(key, value interface{}) bool {
			members.Delete(key)
			return true
		})

		setResErr(int(msgRs.GetErr().GetCode()), msgRs.GetErr().GetMsg())
		setResHeader("name", c.Name)
		setResHeader("own", c.Own)
		if msgRs.GetErr().GetCode() == pb.CodeOK {
			msgPrintStatus(0, c.Own, c.Name, "登陆")
			// todo 登陆成功发起请求查询自身关系信息
			err2 := c.QuerySelfRelation()
			if err2 != nil {
				Logger.Error("QuerySelfRelation failed", zap.Error(err2))
			}
		} else {
			msgPrintStatus(1, c.Own, c.Name, "登陆")
		}
	case pack.Join:
		groupStr := req.Header.Get("group")
		codeStr := req.Header.Get("verifyCode")
		opStr := req.Header.Get("op")
		if !c.IsLogin {
			setResErr(1, "用户未登录,无法操作")
			return
		}
		if len(groupStr) == 0 || len(opStr) == 0 {
			setResErr(2, "参数错误,无法操作")
			return
		}
		if !(opStr == "0" || opStr == "1") {
			setResErr(2, "参数错误,无法操作")
			return
		}
		var code int64
		var err error
		if len(codeStr) > 0 {
			code, err = strconv.ParseInt(codeStr, 10, 64)
			if err != nil {
				setResErr(3, "参数格式错误,无法操作")
				return
			}
		}

		group, e1 := strconv.ParseInt(groupStr, 10, 64)
		op, e3 := strconv.ParseInt(opStr, 10, 64)
		if e1 != nil || e3 != nil {
			setResErr(3, "参数格式错误,无法操作")
			return
		}
		msgStr := "加入讨论组:"
		if op == 1 {
			msgStr = "退出讨论组:"
		}
		nowUnix := time.Now().Unix()
		msgRq := &allpb.JoinGroupRQ{
			Own:       &c.Own,
			Group:     &group,
			Code:      &code,
			TimeStamp: &nowUnix,
			Op:        proto.Int32(int32(op)),
		}
		msgRs := &allpb.JoinGroupRS{}
		if err := NetSend(pb.PackJoinGroupRQ, pb.PackJoinGroupRS, msgRq, msgRs, GetSessionID()); err != nil {
			setResErr(4, "网络发送失败")
			msgPrintStatus(1, c.Own, c.Name, msgStr+groupStr)
			return
		}
		setResErr(int(msgRs.GetErr().GetCode()), msgRs.GetErr().GetMsg())
		if msgRs.GetErr().GetCode() == pb.CodeOK {
			msgPrintStatus(0, c.Own, c.Name, msgStr+groupStr)
			if op == 0 {
				c.JoinGroup.Store(group, &JoinGroupItem{
					Group:      group,
					Name:       msgRs.GetGroupName(),
					VerifyCode: msgRq.GetCode(),
					CreateTime: timeStampToString(nowUnix),
					JoinTime:   timeStampToString(nowUnix),
				})
			} else {
				c.JoinGroup.Delete(group)
			}

		} else {
			msgPrintStatus(1, c.Own, c.Name, msgStr+groupStr)
		}
	case pack.SubscribePersonal:
		peersStr := req.Header.Get("peers")
		opStr := req.Header.Get("op")
		if !c.IsLogin {
			setResErr(1, "用户未登录,无法操作")
			return
		}
		if len(peersStr) == 0 || len(opStr) == 0 {
			setResErr(2, "参数错误,无法操作")
			return
		}

		values := strings.Split(peersStr, ",")
		var peers []int64
		for _, value := range values {
			peer, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				continue
			}
			peers = append(peers, peer)
		}

		op, e2 := strconv.ParseInt(opStr, 10, 64)
		if e2 != nil || len(peers) == 0 {
			setResErr(3, "参数格式错误,无法操作")
			return
		}
		msgStr := "订阅对方:"
		if op == 1 {
			msgStr = "取消订阅对方:"
		}
		nowUnix := time.Now().Unix()
		msgRq := &allpb.SubscribePersonRQ{
			Own:       &c.Own,
			Peers:     peers,
			Op:        proto.Int32(int32(op)),
			TimeStamp: &nowUnix,
		}
		msgRs := &allpb.SubscribePersonRS{}
		if err := NetSend(pb.PackSubscribePersonalMsgRQ, pb.PackSubscribePersonalMsgRS, msgRq, msgRs, GetSessionID()); err != nil {
			setResErr(4, "网络发送失败")
			msgPrintStatus(1, c.Own, c.Name, msgStr+peersStr)
			return
		}
		setResErr(int(msgRs.GetErr().GetCode()), msgRs.GetErr().GetMsg())
		setResHeader("peers", msgRs.GetPeers())
		if msgRs.GetErr().GetCode() == pb.CodeOK {
			msgPrintStatus(0, c.Own, c.Name, msgStr+peersStr)
			if op == 0 {
				for _, peer := range msgRs.GetPeers() {
					c.SubscribePeers.Store(peer.GetPeer(), &PeerInfo{
						Peer:      peer.GetPeer(),
						Name:      peer.GetName(),
						TimeStamp: timeStampToString(peer.GetTimeStamp()),
					})
				}
			} else {
				for _, peer := range msgRs.GetPeers() {
					c.SubscribePeers.Delete(peer)
				}
			}
		} else {
			msgPrintStatus(1, c.Own, c.Name, msgStr+peersStr)
		}
	case pack.CancelAllSubscribe:
		if !c.IsLogin {
			setResErr(1, "用户未登录,无法操作")
			return
		}
		nowUnix := time.Now().Unix()
		msgRq := &allpb.CancelSubscribeAllRQ{
			Own:       &c.Own,
			TimeStamp: &nowUnix,
		}
		msgRs := &allpb.CancelSubscribeAllRS{}
		if err := NetSend(pb.PackCancelSubscribeAllRQ, pb.PackCancelSubscribeAllRS, msgRq, msgRs, GetSessionID()); err != nil {
			setResErr(4, "网络发送失败")
			msgPrintStatus(1, c.Own, c.Name, "取消所有订阅")
			return
		}
		setResErr(int(msgRs.GetErr().GetCode()), msgRs.GetErr().GetMsg())
		if msgRs.GetErr().GetCode() == pb.CodeOK {
			msgPrintStatus(0, c.Own, c.Name, "取消所有订阅")
		} else {
			msgPrintStatus(1, c.Own, c.Name, "取消所有订阅")
		}
	case pack.SendToPersonal:
		if !c.IsLogin {
			setResErr(1, "用户未登录,无法操作")
			return
		}
		peerStr := req.Header.Get("peer")
		content := req.Header.Get("content")
		if len(peerStr) == 0 || len(content) == 0 {
			setResErr(1, "参数错误,无法操作")
			return
		}
		peer, err := strconv.ParseInt(peerStr, 10, 64)
		if err != nil {
			setResErr(2, "参数格式错误,无法操作")
			return
		}
		nowUnix := time.Now().Unix()
		msgRq := &allpb.SendToPersonalRQ{
			Own:       &c.Own,
			Peer:      &peer,
			Content:   &content,
			TimeStamp: &nowUnix,
		}
		msgRs := &allpb.SendToPersonalRS{}
		if err := NetSend(pb.PackSendToPersonalRQ, pb.PackSendToPersonalRS, msgRq, msgRs, GetSessionID()); err != nil {
			setResErr(4, "网络发送失败")
			return
		}
		setResErr(int(msgRs.GetErr().GetCode()), msgRs.GetErr().GetMsg())
		if msgRs.GetErr().GetCode() == pb.CodeOK {
			personal := formatPersonalToPeer(peer, msgRs.GetName(), msgRq.GetTimeStamp())
			formatSelf := colouration(c.ColorCode, personal)
			msgPrintOwn(formatSelf, msgRq.GetContent())
		}
	case pack.SendToGroup:
		if !c.IsLogin {
			setResErr(1, "用户未登录,无法操作")
			return
		}
		groupStr := req.Header.Get("group")
		content := req.Header.Get("content")
		if len(groupStr) == 0 || len(content) == 0 {
			setResErr(1, "参数错误,无法操作")
			return
		}
		group, err := strconv.ParseInt(groupStr, 10, 64)
		if err != nil {
			setResErr(2, "参数格式错误,无法操作")
			return
		}
		nowUnix := time.Now().Unix()
		msgRq := &allpb.SendToTempGroupRQ{
			Own:       &c.Own,
			Group:     &group,
			Content:   &content,
			TimeStamp: &nowUnix,
		}
		msgRs := &allpb.SendToTempGroupRS{}
		if err := NetSend(pb.PackSendToGroupRQ, pb.PackSendToGroupRS, msgRq, msgRs, GetSessionID()); err != nil {
			setResErr(4, "网络发送失败")
			return
		}
		setResErr(int(msgRs.GetErr().GetCode()), msgRs.GetErr().GetMsg())
		if msgRs.GetErr().GetCode() == pb.CodeOK {
			value, ok := c.JoinGroup.Load(group)
			var groupName string
			if ok && value != nil {
				info := value.(*JoinGroupItem)
				groupName = info.Name
			}
			personal := formatGroupToSend(group, groupName, msgRq.GetTimeStamp())
			formatSelf := colouration(c.ColorCode, personal)
			msgPrintOwn(formatSelf, msgRq.GetContent())
		}
	case pack.CreateGroup:
		if !c.IsLogin {
			setResErr(1, "用户未登录,无法操作")
			return
		}
		codeStr := req.Header.Get("verifyCode")
		groupName := req.Header.Get("name")
		code, err2 := strconv.ParseInt(codeStr, 10, 64)
		if err2 != nil {
			setResErr(2, "参数格式错误,无法操作")
			return
		}

		nowUnix := time.Now().Unix()
		msgRq := &allpb.CreateTempGroupRQ{
			Own:       &c.Own,
			Name:      &groupName,
			Code:      &code,
			TimeStamp: &nowUnix,
		}
		msgRs := &allpb.CreateTempGroupRS{}
		if err := NetSend(pb.PackCreateGroupRQ, pb.PackCreateGroupRS, msgRq, msgRs, GetSessionID()); err != nil {
			setResErr(4, "网络发送失败")
			return
		}
		setResErr(int(msgRs.GetErr().GetCode()), msgRs.GetErr().GetMsg())
		if msgRs.GetErr().GetCode() == pb.CodeOK {
			msgPrintStatus(0, c.Own, c.Name, "创建讨论组："+fmt.Sprintf("%s(%d);验证码:%d", msgRq.GetName(), msgRs.GetGroup(), msgRs.GetCode()))
			c.JoinGroup.Store(msgRs.GetGroup(), &JoinGroupItem{
				Group:      msgRs.GetGroup(),
				Name:       msgRq.GetName(),
				VerifyCode: msgRq.GetCode(),
				CreateTime: timeStampToString(nowUnix),
				JoinTime:   timeStampToString(nowUnix),
			})
		} else {
			msgPrintStatus(1, c.Own, c.Name, "创建讨论组："+msgRq.GetName())
		}
	case pack.ReConn:
		// 重新连接
		GCli.Close()
		GCli = client.NewClient(Logger, &msg.DefaultDataPack{}, &msg.DefaultHeaderOperator{}, uint32(SendChanLen), time.Second*3)
		if err := GCli.Connection("tcp", SerAddr, time.Second*3); err != nil {
			msgPrintStatus(1, 0, "", "重新连接服务器")
			return
		}
		msgPrintStatus(0, 0, "", "重新连接服务器")
		GCli.SetProcess(HandlerNotify)
	case pack.GetStatus:
		ownStatus := &OwnStatus{}
		if c.IsLogin {
			ownStatus.Status = "在线中"
			ownStatus.Own = c.Own
			ownStatus.Name = c.Name
			ownStatus.LoginTime = time.Unix(c.LoginTime, 0).String()
			c.JoinGroup.Range(func(key, value interface{}) bool {
				info := value.(*JoinGroupItem)
				ownStatus.Groups = append(ownStatus.Groups, info)
				return true
			})
			c.SubscribePeers.Range(func(key, value interface{}) bool {
				info := value.(*PeerInfo)
				ownStatus.Subscribes = append(ownStatus.Subscribes, info)
				return true
			})
			c.Followers.Range(func(key, value interface{}) bool {
				info := value.(*PeerInfo)
				ownStatus.Followers = append(ownStatus.Followers, info)
				return true
			})
		} else {
			ownStatus.Status = "未登录"
		}

		data, _ := json.Marshal(ownStatus)
		setResHeader("result", string(data))
	case pack.GroupMembers:
		if !c.IsLogin {
			setResErr(1, "用户未登录,无法操作")
			return
		}
		groupStr := req.Header.Get("group")
		group, err := strconv.ParseInt(groupStr, 10, 64)
		if err != nil {
			setResErr(2, "参数格式错误,无法操作")
			return
		}
		value, ok := c.JoinGroup.Load(group)
		if !ok {
			setResErr(3, "没有查看权限")
			return
		}
		groupInfo := value.(*JoinGroupItem)
		msgRq := &allpb.GroupMembersRQ{
			Own:   &c.Own,
			Group: &group,
		}
		msgRs := &allpb.GroupMembersRS{}
		if err := NetSend(pb.PackGroupMemberRQ, pb.PackGroupMemberRS, msgRq, msgRs, GetSessionID()); err != nil {
			setResErr(4, "网络发送失败")
			return
		}
		setResErr(int(msgRs.GetErr().GetCode()), msgRs.GetErr().GetMsg())
		if msgRs.GetErr().GetCode() == pb.CodeOK {
			msgPrintStatus(0, c.Own, c.Name, "查询讨论组成员："+fmt.Sprintf("%s(%d)", groupInfo.Name, groupInfo.Group))
			data, _ := json.Marshal(msgRs.GetMembers())
			setResHeader("result", string(data))
		} else {
			msgPrintStatus(1, c.Own, c.Name, "查询讨论组成员："+fmt.Sprintf("%s(%d)", groupInfo.Name, groupInfo.Group))
		}
	}
}

func (c *ClientHttpHandler) QuerySelfRelation() error {
	if !c.IsLogin {
		return fmt.Errorf("offline")
	}
	msgRq := &allpb.SelfRelationRQ{
		Own: &c.Own,
	}
	msgRs := &allpb.SelfRelationRS{}
	if err := NetSend(pb.PackSelfRelationRQ, pb.PackSelfRelationRS, msgRq, msgRs, GetSessionID()); err != nil {
		return err
	}

	if msgRs.GetErr().GetCode() != pb.CodeOK {
		return fmt.Errorf(msgRs.GetErr().String())
	}

	c.JoinGroup.Range(func(key, value interface{}) bool {
		c.JoinGroup.Delete(key)
		return true
	})
	c.SubscribePeers.Range(func(key, value interface{}) bool {
		c.SubscribePeers.Delete(key)
		return true
	})
	c.Followers.Range(func(key, value interface{}) bool {
		c.Followers.Delete(key)
		return true
	})

	for _, item := range msgRs.GetGroups() {
		c.JoinGroup.Store(item.GetGroup(), &JoinGroupItem{
			Group:      item.GetGroup(),
			Name:       item.GetName(),
			CreateTime: timeStampToString(item.GetCreateTime()),
			JoinTime:   timeStampToString(item.GetJoinTime()),
			VerifyCode: item.GetCode(),
		})
	}

	for _, item := range msgRs.GetSubscribers() {
		c.SubscribePeers.Store(item.GetPeer(), &PeerInfo{
			Peer:      item.GetPeer(),
			Name:      item.GetName(),
			TimeStamp: timeStampToString(item.GetTimeStamp())})
	}

	for _, item := range msgRs.GetFollowers() {
		c.Followers.Store(item.GetPeer(), &PeerInfo{
			Peer:      item.GetPeer(),
			Name:      item.GetName(),
			TimeStamp: timeStampToString(item.GetTimeStamp())})
	}

	return nil
}

func timeStampToString(t int64) string {
	return time.Unix(t, 0).Format("2006-01-02 15:04:05")
}
