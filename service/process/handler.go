package process

import (
	"github.com/catscai/ccat/clog"
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
	conn      iface.IConn
	LoginTime int64
	Name      string
	subscribe sync.Map // int64 -> int64 peer -> timeStamp 被peer订阅

	ownSubscribe         sync.Map // int64 -> int64 peer -> timeStamp own 订阅的人
	ownJoinGroup         sync.Map // int64 -> int64 group -> timeStamp own 订阅的组
	LastSendWorldMsgTime int64    // 上一次发送世界消息时间戳 用作发送CD
	IsSubscribeWorld     bool     // 是否订阅了世界消息
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
			var users []int64
			var groups []int64
			GUserSession.Range(func(key, value interface{}) bool {
				info := value.(*UserSession)
				if info.conn.Valid() {
					return true
				}
				GUserSession.Delete(key)
				users = append(users, info.Own)
				// 用户下线需要通知所有该用户订阅的人, 取消订阅
				info.ownSubscribe.Range(func(key, value interface{}) bool {
					peer := key.(int64)
					val, ok := GUserSession.Load(peer)
					if !ok {
						return true
					}
					sess := val.(*UserSession)
					sess.subscribe.Delete(peer)
					nowUnix := time.Now().Unix()
					_ = GAsyncIoPool.SendTask(uint64(peer), func() {
						msgRq := &allpb.PublishSubscribeMsgRQ{
							Peer:      proto.Int64(info.Own),
							Name:      &info.Name,
							TimeStamp: &nowUnix,
							Op:        proto.Int32(1),
						}
						data, _ := proto.Marshal(msgRq)
						pkg := GPackOp.Full(pb.PackPublishSubscribeMsgRQ, data, &msg.DefaultHeader{})
						_ = sess.conn.SendMsg(pkg)
					})
					return true
				})

				// 通知自己所加入组的成员退出讨论组消息
				info.ownJoinGroup.Range(func(key, value interface{}) bool {
					group := key.(int64)
					val, ok := GGroupMap.Load(group)
					if !ok {
						return true
					}
					groupInfo := val.(*TempGroupInfo)
					groupInfo.Members.Delete(info.Own)
					AsyncSendGroupMemberChange(groupInfo, 1, info.Own, info.Name, time.Now().Unix())
					return true
				})
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
					genGroupLock.Lock()
					delete(genGroupMap, key.(int64))
					genGroupLock.Unlock()
					groups = append(groups, info.Group)
				}

				return true
			})

			clog.AppLogger().Debug("timer monitor clean", zap.Any("users", users), zap.Any("groups", groups))
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
		Own:              req.GetOwn(),
		Name:             userInfo.GetName(),
		LoginTime:        time.Now().Unix(),
		conn:             ctx.IConn,
		IsSubscribeWorld: true, // 登陆成功后默认订阅世界消息
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
		if err := peerSess.conn.SendMsg(pkg); err != nil {
			ctx.Error(funcName+" send msg to personal failed", zap.Error(err), zap.Any("msgRq", msgRq))
		}
	})

	res.Name = &peerSess.Name
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
	// 异步发送广播给组成员
	AsyncSendToGroup(ctx, groupInfo, req.GetOwn(), sessInfo.Name, req.GetContent(), req.GetTimeStamp())

	res.Name = &groupInfo.Name
	return
}

func HandleSubscribePersonalRQ(ctx *iface.CatContext, reqMsg, rspMsg proto.Message) (err error) {
	const funcName = "HandleSubscribePersonalRQ"
	req := reqMsg.(*allpb.SubscribePersonRQ)
	res := rspMsg.(*allpb.SubscribePersonRS)
	var desc string
	switch req.GetOwn() {
	case 0:
		desc = "订阅对方消息成功"
	case 1:
		desc = "取消订阅对方消息成功"
	case 2:
		desc = "订阅世界消息成功"
	case 3:
		desc = "取消订阅世界消息成功"
	}
	res.Err = GenErr(pb.CodeOK, desc)

	var sessInfo *UserSession

	ctx.Debug(funcName+" begin", zap.Any("req", req), zap.Any("self", sessInfo))

	defer func() {
		ctx.Debug(funcName+" end", zap.Any("res", res), zap.Any("self", sessInfo))
	}()

	if !(req.GetOp() >= 0 && req.GetOp() <= 3) {
		res.Err = GenErr(pb.CodeSendError, "参数错误")
		ctx.Warn(funcName + " params is invalid")
		return
	}

	sessInfo = GetSelf(ctx)
	if sessInfo == nil {
		res.Err = GenErr(pb.CodeSendError, "用户不在线上,请先登陆")
		ctx.Warn(funcName + " user is offline")
		return
	}
	nowUnix := time.Now().Unix()
	if req.GetOp() < 2 {
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

			_ = GAsyncIoPool.SendTask(uint64(peer), func() {
				msgRq := &allpb.PublishSubscribeMsgRQ{
					Peer:      proto.Int64(req.GetOwn()),
					Name:      &sessInfo.Name,
					TimeStamp: req.TimeStamp,
					Op:        proto.Int32(req.GetOp()),
				}
				data, _ := proto.Marshal(msgRq)
				pkg := GPackOp.Full(pb.PackPublishSubscribeMsgRQ, data, &msg.DefaultHeader{})
				if err := sess.conn.SendMsg(pkg); err != nil {
					ctx.Error(funcName+" publish subscribe personal failed", zap.Error(err), zap.Any("msgRq", msgRq))
				}
			})

			res.Peers = append(res.Peers, &allpb.PeerInfo{
				Peer:      proto.Int64(peer),
				Name:      &sess.Name,
				TimeStamp: req.TimeStamp,
			})
		}
	} else {
		if req.GetOp() == 2 {
			sessInfo.IsSubscribeWorld = true
		} else {
			sessInfo.IsSubscribeWorld = false
		}
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
		if _, ok := groupInfo.Members.Load(req.GetOwn()); ok {
			res.Err = GenErr(pb.CodeJoinError, "该用户已在讨论组中")
			ctx.Warn(funcName+" the user already exists at the group", zap.Any("group", req.GetGroup()), zap.Any("own", req.GetOwn()))
			return
		}
		nowUnix := time.Now().Unix()
		groupInfo.Members.Store(req.GetOwn(), nowUnix)
		res.Err = GenErr(pb.CodeOK, "加入讨论组成功")
		sessInfo.ownJoinGroup.Store(req.GetGroup(), nowUnix)
	} else {
		if _, ok := groupInfo.Members.Load(req.GetOwn()); !ok {
			res.Err = GenErr(pb.CodeJoinError, "该用户不在讨论组中")
			ctx.Warn(funcName+" the user not exists at the group", zap.Any("group", req.GetGroup()), zap.Any("own", req.GetOwn()))
			return
		}
		groupInfo.Members.Delete(req.GetOwn())
		res.Err = GenErr(pb.CodeOK, "退出讨论组成功")
		sessInfo.ownJoinGroup.Delete(req.GetGroup())
	}
	res.GroupName = &groupInfo.Name

	// 异步向讨论组中其他成员推送该消息
	AsyncSendGroupMemberChange(groupInfo, req.GetOp(), req.GetOwn(), sessInfo.Name, req.GetTimeStamp())
	return
}

func HandleGroupMembersRQ(ctx *iface.CatContext, reqMsg, rspMsg proto.Message) (err error) {
	const funcName = "HandleGroupMembersRQ"
	req := reqMsg.(*allpb.GroupMembersRQ)
	res := rspMsg.(*allpb.GroupMembersRS)
	res.Err = GenErr(pb.CodeOK, "查询讨论组成员成功")
	var sessInfo *UserSession

	ctx.Debug(funcName+" begin", zap.Any("req", req), zap.Any("self", sessInfo))
	defer func() {
		ctx.Debug(funcName+" end", zap.Any("res", res), zap.Any("self", sessInfo))
	}()

	sessInfo = GetSelf(ctx)
	if sessInfo == nil {
		res.Err = GenErr(pb.CodeGroupMemberError, "用户不在线上,请先登陆")
		ctx.Warn(funcName + " user is offline")
		return
	}

	value, ok := GGroupMap.Load(req.GetGroup())
	if !ok || value == nil {
		res.Err = GenErr(pb.CodeGroupMemberError, "讨论组不存在")
		ctx.Warn(funcName+" this group not exists", zap.Any("group", req.GetGroup()))
		return
	}

	groupInfo := value.(*TempGroupInfo)

	if _, ok := groupInfo.Members.Load(req.GetOwn()); !ok {
		res.Err = GenErr(pb.CodeGroupMemberError, "没有查询组成员权限")
		ctx.Warn(funcName+" the user not in this group", zap.Any("group", req.GetGroup()))
		return
	}

	groupInfo.Members.Range(func(key, value interface{}) bool {
		uid := key.(int64)
		timeStamp := value.(int64)
		info, err := GetUserInfo(ctx, uid)
		if err != nil {
			return true
		}
		res.Members = append(res.Members, &allpb.GroupMemberItem{
			Uid:      &uid,
			Name:     info.Name,
			JoinTime: &timeStamp,
		})
		return true
	})

	return
}

func HandleSelfRelationRQ(ctx *iface.CatContext, reqMsg, rspMsg proto.Message) (err error) {
	const funcName = "HandleSelfRelationRQ"
	req := reqMsg.(*allpb.SelfRelationRQ)
	res := rspMsg.(*allpb.SelfRelationRS)
	res.Err = GenErr(pb.CodeOK, "查询自身关系信息成功")
	var sessInfo *UserSession

	ctx.Debug(funcName+" begin", zap.Any("req", req), zap.Any("self", sessInfo))
	defer func() {
		ctx.Debug(funcName+" end", zap.Any("res", res), zap.Any("self", sessInfo))
	}()

	sessInfo = GetSelf(ctx)
	if sessInfo == nil {
		res.Err = GenErr(pb.CodeSelfRelationError, "用户不在线上,请先登陆")
		ctx.Warn(funcName + " user is offline")
		return
	}
	sessInfo.ownSubscribe.Range(func(key, value interface{}) bool {
		peer := key.(int64)
		t := value.(int64)
		info, err := GetUserInfo(ctx, peer)
		if err != nil {
			return true
		}
		res.Subscribers = append(res.Subscribers, &allpb.PeerInfo{
			Peer:      &peer,
			Name:      info.Name,
			TimeStamp: &t,
		})
		return true
	})

	sessInfo.subscribe.Range(func(key, value interface{}) bool {
		peer := key.(int64)
		t := value.(int64)
		info, err := GetUserInfo(ctx, peer)
		if err != nil {
			return true
		}
		res.Followers = append(res.Followers, &allpb.PeerInfo{
			Peer:      &peer,
			Name:      info.Name,
			TimeStamp: &t,
		})
		return true
	})

	sessInfo.ownJoinGroup.Range(func(key, value interface{}) bool {
		group := key.(int64)
		t := value.(int64)
		val, ok := GGroupMap.Load(group)
		if !ok {
			return true
		}
		groupInfo := val.(*TempGroupInfo)
		res.Groups = append(res.Groups, &allpb.GroupInfo{
			Group:      &group,
			Name:       &groupInfo.Name,
			CreateTime: &groupInfo.CreateTime,
			JoinTime:   &t,
			Code:       &groupInfo.Code,
		})
		return true
	})

	return nil
}

func HandleSendWorldMsgRQ(ctx *iface.CatContext, reqMsg, rspMsg proto.Message) (err error) {
	const funcName = "HandleSendWorldMsgRQ"
	req := reqMsg.(*allpb.SendWorldMessageRQ)
	res := rspMsg.(*allpb.SendWorldMessageRS)
	res.Err = GenErr(pb.CodeOK, "发送世界消息成功")
	var sessInfo *UserSession

	ctx.Debug(funcName+" begin", zap.Any("req", req), zap.Any("self", sessInfo))
	defer func() {
		ctx.Debug(funcName+" end", zap.Any("res", res), zap.Any("self", sessInfo))
	}()

	sessInfo = GetSelf(ctx)
	if sessInfo == nil {
		res.Err = GenErr(pb.CodeSendWorldMsgError, "用户不在线上,请先登陆")
		ctx.Warn(funcName + " user is offline")
		return
	}
	nowUnix := time.Now().Unix()
	if nowUnix-sessInfo.LastSendWorldMsgTime < 15 {
		res.Err = GenErr(pb.CodeSendWorldMsgError, "发送太频繁,处于发送CD中")
		ctx.Warn(funcName + " send cd be limited")
		return
	}
	sessInfo.LastSendWorldMsgTime = nowUnix

	GUserSession.Range(func(key, value interface{}) bool {
		sess := value.(*UserSession)
		if sess.conn.Valid() == false {
			return true
		}
		if sess.Own == req.GetOwn() {
			return true
		}

		if !sess.IsSubscribeWorld {
			return true
		}

		_ = GAsyncIoPool.SendTask(uint64(sess.Own), func() {
			msgRq := &allpb.PublishPersonalMsgRQ{
				Peer:      req.Own,
				Name:      &sessInfo.Name,
				Content:   req.Content,
				TimeStamp: req.TimeStamp,
				Op:        proto.Int32(1), // 世界消息
			}
			data, _ := proto.Marshal(msgRq)
			pkg := GPackOp.Full(pb.PackPublishPersonalMsgRQ, data, &msg.DefaultHeader{})
			_ = sess.conn.SendMsg(pkg)
		})

		return true
	})

	return nil
}
