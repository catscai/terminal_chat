package process

import (
	"github.com/catscai/ccat/iface"
	"github.com/catscai/ccat/impl/msg"
	"github.com/catscai/terminal_chat/pb"
	"github.com/catscai/terminal_chat/pb/src/allpb"
	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
	"sync"
	"time"
)

type UserSession struct {
	Own       int64
	Conn      iface.IConn
	LoginTime int64
	Name      string
	subscribe sync.Map // int64 -> int64 peer -> timeStamp 被peer订阅

	ownSubscribe sync.Map // int64 -> int64 peer -> timeStamp own 订阅的人
	ownJoinGroup sync.Map // int64 -> int64 group -> timeStamp own 订阅的组
}

type TempGroupInfo struct {
	Group      int64
	Name       string
	Code       int64
	Members    sync.Map // int64 -> int64 peer -> timeStamp
	CreateTime int64
}

var GUserSession sync.Map // int64 -> *UserSession uid -> info
var GGroupMap sync.Map    // int64 -> *TempGroupInfo group -> info

func init() {
	// 定时器定时扫描 是否存在无效的连接，将缓存清除
	go func() {
		t := time.NewTicker(time.Second * 30)
		for range t.C {
			GUserSession.Range(func(key, value interface{}) bool {
				info := value.(*UserSession)
				if info.Conn.Valid() == false {
					GUserSession.Delete(key)
				}
				return true
			})

			// 扫描没有用户成员的组, 自动解散
			GGroupMap.Range(func(key, value interface{}) bool {
				info := value.(*TempGroupInfo)
				flag := false
				info.Members.Range(func(key, value interface{}) bool {
					id := key.(int64)
					if _, ok := GUserSession.Load(id); ok {
						flag = true
						return true
					}
					return true
				})
				if !flag {
					// 这个组已经不存在成员了,则将组删除
					GGroupMap.Delete(key)
				}

				return true
			})
		}
	}()
}

const (
	Self = "self"
)

func GenErr(code int32, desc string) *allpb.ErrInfo {
	return &allpb.ErrInfo{Code: &code, Msg: &desc}
}

func GetSelf(ctx *iface.CatContext) *UserSession {
	sessInfoValue := ctx.GetProperty(Self)
	if sessInfoValue == nil {
		return nil
	}
	return sessInfoValue.(*UserSession)
}

func HandleRegisterRQ(ctx *iface.CatContext, reqMsg, rspMsg proto.Message) (err error) {
	const funcName = "HandleRegisterRQ"
	req := reqMsg.(*allpb.RegisterRQ)
	res := rspMsg.(*allpb.RegisterRS)
	res.Err = GenErr(pb.CodeOK, "注册成功")

	ctx.Debug(funcName+" begin", zap.Any("req", req))
	defer func() {
		ctx.Debug(funcName+" end", zap.Any("res", res))
	}()

	uid := GenUID(ctx)
	if uid == -1 {
		res.Err = GenErr(pb.CodeRegisterError, "UID生成失败,请重试")
		return
	}
	userInfo := &allpb.UserInfo{
		Own:          &uid,
		Email:        req.Email,
		Name:         req.Name,
		Passwd:       req.Passwd,
		RegisterTime: proto.Int64(time.Now().Unix()),
	}

	if err = SaveUserInfo(ctx, userInfo); err != nil {
		res.Err = GenErr(pb.CodeRegisterError, "用户数据保存失败,请重试")
		ctx.Error(funcName+" SaveUserInfo failed", zap.Error(err), zap.Any("userInfo", userInfo))
		return
	}

	res.Own = &uid

	return
}

func HandleLoginRQ(ctx *iface.CatContext, reqMsg, rspMsg proto.Message) (err error) {
	const funcName = "HandleLoginRQ"
	req := reqMsg.(*allpb.LoginRQ)
	res := rspMsg.(*allpb.LoginRS)
	res.Err = GenErr(pb.CodeOK, "登陆成功")

	ctx.Debug(funcName+" begin", zap.Any("req", req))
	defer func() {
		ctx.Debug(funcName+" end", zap.Any("res", res))
	}()

	if _, ok := GUserSession.Load(req.GetOwn()); ok {
		res.Err = GenErr(pb.CodeLoginError, "用户已在线上")
		ctx.Warn(funcName + " user already login success")
		return
	}

	genUidLock.RLock()
	if _, ok := genUidMap[req.GetOwn()]; !ok {
		res.Err = GenErr(pb.CodeLoginError, "用户账号或密码错误")
		ctx.Warn(funcName+" user not exists", zap.Any("own", req.GetOwn()))
		genUidLock.RUnlock()
		return
	}
	genUidLock.RUnlock()

	userInfo, err := GetUserInfo(ctx, req.GetOwn())
	if err != nil {
		res.Err = GenErr(pb.CodeLoginError, "用户信息获取失败")
		ctx.Error(funcName+" GetUserInfo failed", zap.Error(err), zap.Int64("own", req.GetOwn()))
		return
	}

	if userInfo.GetPasswd() != req.GetPasswd() {
		res.Err = GenErr(pb.CodeLoginError, "用户账号或密码错误")
		ctx.Error(funcName + " passwd not match")
		return
	}

	res.Name = userInfo.Name
	sessInfo := &UserSession{
		Own:       req.GetOwn(),
		Name:      userInfo.GetName(),
		LoginTime: time.Now().Unix(),
		Conn:      ctx.IConn,
	}
	GUserSession.Store(req.GetOwn(), sessInfo)

	if oldSessValue := ctx.GetProperty(Self); oldSessValue != nil {
		oldSess := oldSessValue.(*UserSession)
		GUserSession.Delete(oldSess.Own)
	}

	ctx.SetProperty(Self, sessInfo)
	return
}

func HandleSendToPersonalRQ(ctx *iface.CatContext, reqMsg, rspMsg proto.Message) (err error) {
	const funcName = "HandleSendToPersonalRQ"
	req := reqMsg.(*allpb.SendToPersonalRQ)
	res := rspMsg.(*allpb.SendToPersonalRS)
	res.Err = GenErr(pb.CodeOK, "私人消息发送成功")

	var sessInfo *UserSession

	ctx.Debug(funcName+" begin", zap.Any("req", req), zap.Any("self", sessInfo))

	defer func() {
		ctx.Debug(funcName+" end", zap.Any("res", res), zap.Any("self", sessInfo))
	}()

	sessInfo = GetSelf(ctx)
	if sessInfo == nil {
		res.Err = GenErr(pb.CodeSendError, "用户不在线上,请先登陆")
		ctx.Warn(funcName + " user is offline")
		return
	}

	value, ok := GUserSession.Load(req.GetPeer())
	if !ok || value == nil {
		res.Err = GenErr(pb.CodeSendError, "发送失败,对方不在线")
		ctx.Warn(funcName+" peer is offline", zap.Any("peer", req.GetPeer()), zap.Any("self", sessInfo))
		return
	}
	peerSess := value.(*UserSession)
	if _, ok := peerSess.ownSubscribe.Load(req.GetOwn()); !ok {
		res.Err = GenErr(pb.CodeSendError, "发送失败,对方没有订阅你")
		ctx.Warn(funcName+" peer not subscribe own", zap.Any("own", req.GetOwn()), zap.Any("peer", req.GetPeer()), zap.Any("self", sessInfo))
		return
	}

	// 异步发送
	_ = GAsyncIoPool.SendTask(uint64(req.GetPeer()), func() {
		msgRq := &allpb.PublishPersonalMsgRQ{
			Peer:      req.Own,
			Name:      &sessInfo.Name,
			Content:   req.Content,
			TimeStamp: req.TimeStamp,
		}
		data, _ := proto.Marshal(msgRq)
		pkg := GPackOp.Full(pb.PackPublishPersonalMsgRQ, data, &msg.DefaultHeader{})
		if err := peerSess.Conn.SendMsg(pkg); err != nil {
			ctx.Error(funcName+" send msg to personal failed", zap.Error(err), zap.Any("msgRq", msgRq))
		}
	})
	return
}

func HandleSendToGroupRQ(ctx *iface.CatContext, reqMsg, rspMsg proto.Message) (err error) {
	const funcName = "HandleSendToGroupRQ"
	req := reqMsg.(*allpb.SendToTempGroupRQ)
	res := rspMsg.(*allpb.SendToTempGroupRS)
	res.Err = GenErr(pb.CodeOK, "组消息发送成功")

	var sessInfo *UserSession

	ctx.Debug(funcName+" begin", zap.Any("req", req), zap.Any("self", sessInfo))

	defer func() {
		ctx.Debug(funcName+" end", zap.Any("res", res), zap.Any("self", sessInfo))
	}()

	sessInfo = GetSelf(ctx)
	if sessInfo == nil {
		res.Err = GenErr(pb.CodeSendError, "用户不在线上,请先登陆")
		ctx.Warn(funcName + " user is offline")
		return
	}

	value, ok := GGroupMap.Load(req.GetGroup())
	if !ok || value == nil {
		res.Err = GenErr(pb.CodeSendError, "讨论组不存在")
		ctx.Warn(funcName+" group not exists", zap.Any("group", req.GetGroup()))
		return
	}

	groupInfo := value.(*TempGroupInfo)

	if _, ok := groupInfo.Members.Load(req.GetOwn()); !ok {
		res.Err = GenErr(pb.CodeSendError, "用户没有加入该讨论组")
		ctx.Warn(funcName+" not join the group", zap.Any("group", req.GetGroup()))

		return
	}
	// 异步发送

	_ = GAsyncIoPool.SendTask(uint64(req.GetGroup()), func() {
		msgRq := &allpb.PublishGroupMsgRQ{
			Group:     req.Group,
			Name:      &groupInfo.Name,
			Content:   req.Content,
			TimeStamp: req.TimeStamp,
			Peer:      &sessInfo.Own,
			PeerName:  &sessInfo.Name,
		}
		data, _ := proto.Marshal(msgRq)
		groupInfo.Members.Range(func(key, value interface{}) bool {
			uid := key.(int64)
			if uid == req.GetOwn() {
				// 发送给组的消息 过滤掉自己
				return true
			}
			sessValue, ok := GUserSession.Load(uid)
			if !ok || sessValue == nil {
				return true
			}
			sess := sessValue.(*UserSession)
			pkg := GPackOp.Full(pb.PackPublishGroupMsgRQ, data, &msg.DefaultHeader{})
			if err := sess.Conn.SendMsg(pkg); err != nil {
				ctx.Error(funcName+" SendMsg failed", zap.Any("peer", uid), zap.Any("group", req.GetGroup()), zap.Error(err))
			}
			return true
		})
	})

	return
}

func HandleSubscribePersonalRQ(ctx *iface.CatContext, reqMsg, rspMsg proto.Message) (err error) {
	const funcName = "HandleSubscribePersonalRQ"
	req := reqMsg.(*allpb.SubscribePersonRQ)
	res := rspMsg.(*allpb.SubscribePersonRS)
	res.Err = GenErr(pb.CodeOK, "订阅对方消息成功")

	var sessInfo *UserSession

	ctx.Debug(funcName+" begin", zap.Any("req", req), zap.Any("self", sessInfo))

	defer func() {
		ctx.Debug(funcName+" end", zap.Any("res", res), zap.Any("self", sessInfo))
	}()

	sessInfo = GetSelf(ctx)
	if sessInfo == nil {
		res.Err = GenErr(pb.CodeSendError, "用户不在线上,请先登陆")
		ctx.Warn(funcName + " user is offline")
		return
	}
	nowUnix := time.Now().Unix()
	for _, peer := range req.GetPeers() {
		value, ok := GUserSession.Load(peer)
		if !ok || value == nil {
			continue
		}
		if req.GetOwn() == peer {
			// 不能自己订阅自己
			continue
		}
		sess := value.(*UserSession)
		if req.GetOp() == 0 {
			sess.subscribe.Store(req.GetOwn(), nowUnix)
			sessInfo.ownSubscribe.Store(peer, nowUnix)
			ctx.Debug("subscribe info", zap.Any("own", req.GetOwn()), zap.Any("peer", peer), zap.Any("syncMap", sess.subscribe))
		} else {
			sess.subscribe.Delete(req.GetOwn())
			sessInfo.ownSubscribe.Delete(peer)
			ctx.Debug("subscribe info delete", zap.Any("own", req.GetOwn()), zap.Any("peer", peer), zap.Any("syncMap", sess.subscribe))
		}
		res.Peers = append(res.Peers, peer)
	}

	return
}

func HandleSubscribeGroupRQ(ctx *iface.CatContext, reqMsg, rspMsg proto.Message) (err error) {

	return
}

func HandleCancelSubscribeAllRQ(ctx *iface.CatContext, reqMsg, rspMsg proto.Message) (err error) {
	const funcName = "HandleCancelSubscribeAllRQ"
	req := reqMsg.(*allpb.CancelSubscribeAllRQ)
	res := rspMsg.(*allpb.CancelSubscribeAllRS)
	res.Err = GenErr(pb.CodeOK, "取消所有订阅成功")

	var sessInfo *UserSession

	ctx.Debug(funcName+" begin", zap.Any("req", req), zap.Any("self", sessInfo))
	defer func() {
		ctx.Debug(funcName+" end", zap.Any("res", res), zap.Any("self", sessInfo))
	}()

	sessInfo = GetSelf(ctx)
	if sessInfo == nil {
		res.Err = GenErr(pb.CodeSendError, "用户不在线上,请先登陆")
		ctx.Warn(funcName + " user is offline")
		return
	}
	sessInfo.ownSubscribe.Range(func(key, value interface{}) bool {
		peer := key.(int64)
		val, ok := GUserSession.Load(peer)
		if !ok || val == nil {
			return true
		}
		peerSess := val.(*UserSession)
		peerSess.subscribe.Delete(req.GetOwn())
		return true
	})

	sessInfo.ownJoinGroup.Range(func(key, value interface{}) bool {
		group := key.(int64)
		val, ok := GGroupMap.Load(group)
		if !ok || val == nil {
			return true
		}
		groupInfo := val.(*TempGroupInfo)
		groupInfo.Members.Delete(req.GetOwn())
		return true
	})

	return
}

func HandleCreateGroupRQ(ctx *iface.CatContext, reqMsg, rspMsg proto.Message) (err error) {
	const funcName = "HandleCreateGroupRQ"
	req := reqMsg.(*allpb.CreateTempGroupRQ)
	res := rspMsg.(*allpb.CreateTempGroupRS)
	res.Err = GenErr(pb.CodeOK, "创建讨论组成功")

	var sessInfo *UserSession

	ctx.Debug(funcName+" begin", zap.Any("req", req), zap.Any("self", sessInfo))
	defer func() {
		ctx.Debug(funcName+" end", zap.Any("res", res), zap.Any("self", sessInfo))
	}()

	sessInfo = GetSelf(ctx)
	if sessInfo == nil {
		res.Err = GenErr(pb.CodeSendError, "用户不在线上,请先登陆")
		ctx.Warn(funcName + " user is offline")
		return
	}

	group := GenGroup()

	ctx.Debug(funcName+" GenGroup", zap.Any("group", group))

	groupInfo := &TempGroupInfo{
		Group:      group,
		Name:       req.GetName(),
		Code:       req.GetCode(),
		CreateTime: req.GetTimeStamp(),
	}
	groupInfo.Members.Store(req.GetOwn(), time.Now().Unix())

	GGroupMap.Store(group, groupInfo)

	res.Group = &group
	res.Code = req.Code

	return
}

func HandleJoinGroupRQ(ctx *iface.CatContext, reqMsg, rspMsg proto.Message) (err error) {
	const funcName = "HandleJoinGroupRQ"
	req := reqMsg.(*allpb.JoinGroupRQ)
	res := rspMsg.(*allpb.JoinGroupRS)

	var sessInfo *UserSession

	ctx.Debug(funcName+" begin", zap.Any("req", req), zap.Any("self", sessInfo))
	defer func() {
		ctx.Debug(funcName+" end", zap.Any("res", res), zap.Any("self", sessInfo))
	}()

	sessInfo = GetSelf(ctx)
	if sessInfo == nil {
		res.Err = GenErr(pb.CodeSendError, "用户不在线上,请先登陆")
		ctx.Warn(funcName + " user is offline")
		return
	}

	value, ok := GGroupMap.Load(req.GetGroup())
	if !ok || value == nil {
		res.Err = GenErr(pb.CodeJoinError, "讨论组不存在")
		ctx.Warn(funcName+" this group not exists", zap.Any("group", req.GetGroup()))
		return
	}

	groupInfo := value.(*TempGroupInfo)
	if req.GetOp() == 0 {
		if groupInfo.Code != req.GetCode() {
			res.Err = GenErr(pb.CodeJoinError, "讨论组加入码不正确")
			ctx.Warn(funcName+" the code not match", zap.Any("groupInfo.code", groupInfo.Code), zap.Any("code", req.GetCode()))
			return
		}
		nowUnix := time.Now().Unix()
		groupInfo.Members.Store(req.GetOwn(), nowUnix)
		res.Err = GenErr(pb.CodeOK, "加入讨论组成功")
		sessInfo.ownJoinGroup.Store(req.GetGroup(), nowUnix)
	} else {
		groupInfo.Members.Delete(req.GetOwn())
		res.Err = GenErr(pb.CodeOK, "退出讨论组成功")
		sessInfo.ownJoinGroup.Delete(req.GetGroup())
	}
	res.GroupName = &groupInfo.Name
	return
}
