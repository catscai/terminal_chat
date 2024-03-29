package main

import (
	"fmt"
	"github.com/catscai/ccat/iface/imsg"
	"github.com/catscai/terminal_chat/pb"
	"github.com/catscai/terminal_chat/pb/src/allpb"
	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
	"net"
)

var routeHandleMap = map[uint32]func(data []byte) error{
	pb.PackPublishPersonalMsgRQ:       HandlerPublishPersonal,
	pb.PackPublishGroupMsgRQ:          HandlerPublishGroup,
	pb.PackPublishSubscribeMsgRQ:      HandlerPublishSubscribe,
	pb.PackPublishGroupMemberChangeRQ: HandlerGroupMemberChange,
}

func HandlerNotify(conn net.Conn, header imsg.IHeaderPack) error {
	const funcName = "HandlerNotify"
	packType := header.GetPackType().(uint32)
	handler, ok := routeHandleMap[packType]
	if !ok {
		Logger.Warn(funcName+" receive no register handler message", zap.Any("packType", header.GetPackType()))
		return fmt.Errorf("no deal")
	}
	Logger.Debug(funcName+" receive publish message", zap.Any("packType", header.GetPackType()))
	return handler(header.GetData())
}

func HandlerPublishPersonal(data []byte) error {
	const funcName = "HandlerPublishPersonal"
	pkg := &allpb.PublishPersonalMsgRQ{}
	if err := proto.Unmarshal(data, pkg); err != nil {
		Logger.Error(funcName+" proto.Unmarshal failed", zap.Error(err), zap.Any("data.size", len(data)))
		return err
	}

	var info *MemberInfo
	var id int64
	if pkg.GetOp() == 1 {
		// 世界消息
		id = 0
	} else {
		id = pkg.GetPeer()
	}

	if value, ok := members.Load(id); ok && value != nil {
		info = value.(*MemberInfo)
	} else {
		info = &MemberInfo{
			ID:        id,
			ColorCode: randomTextColor(),
		}
		members.Store(id, info)
	}
	var msgStr string
	if id == 0 {
		msgStr = formatPersonalWorld(pkg.GetPeer(), pkg.GetName(), pkg.GetTimeStamp())
	} else {
		msgStr = formatPersonal(pkg.GetPeer(), pkg.GetName(), pkg.GetTimeStamp())
	}

	colorPersonal := colouration(info.ColorCode, msgStr)
	msgPrintPeer(colorPersonal, pkg.GetContent())
	return nil
}

func HandlerPublishGroup(data []byte) error {
	const funcName = "HandlerPublishGroup"
	pkg := &allpb.PublishGroupMsgRQ{}
	if err := proto.Unmarshal(data, pkg); err != nil {
		Logger.Error(funcName+" proto.Unmarshal failed", zap.Error(err), zap.Any("data.size", len(data)))
		return err
	}

	var info *MemberInfo
	if value, ok := members.Load(pkg.GetPeer()); ok && value != nil {
		info = value.(*MemberInfo)
	} else {
		info = &MemberInfo{
			ID:        pkg.GetPeer(),
			ColorCode: randomTextColor(),
		}
		members.Store(pkg.GetPeer(), info)
	}

	groupMsg := formatGroup(pkg.GetPeer(), pkg.GetGroup(), pkg.GetPeerName(), pkg.GetName(), pkg.GetTimeStamp())
	colorGroupMsg := colouration(info.ColorCode, groupMsg)
	msgPrintPeer(colorGroupMsg, pkg.GetContent())
	return nil
}

func HandlerPublishSubscribe(data []byte) error {
	const funcName = "HandlerPublishSubscribe"
	pkg := &allpb.PublishSubscribeMsgRQ{}
	if err := proto.Unmarshal(data, pkg); err != nil {
		Logger.Error(funcName+" proto.Unmarshal failed", zap.Error(err), zap.Any("data.size", len(data)))
		return err
	}

	var info *MemberInfo
	if value, ok := members.Load(pkg.GetPeer()); ok && value != nil {
		info = value.(*MemberInfo)
	} else {
		info = &MemberInfo{
			ID:        pkg.GetPeer(),
			ColorCode: randomTextColor(),
		}
		members.Store(pkg.GetPeer(), info)
	}
	if pkg.GetOp() == 0 {
		httpHandler.Followers.Store(pkg.GetPeer(), &PeerInfo{
			Peer:      pkg.GetPeer(),
			Name:      pkg.GetName(),
			TimeStamp: timeStampToString(pkg.GetTimeStamp()),
		})

		msgPrintStatus(0, pkg.GetPeer(), pkg.GetName(), "订阅了我的消息")
	} else {
		httpHandler.Followers.Delete(pkg.GetPeer())
		msgPrintStatus(0, pkg.GetPeer(), pkg.GetName(), "取消对我的订阅")
	}

	return nil
}

func HandlerGroupMemberChange(data []byte) error {
	const funcName = "HandlerGroupMemberChange"
	pkg := &allpb.PublishGroupMemberChangeRQ{}
	if err := proto.Unmarshal(data, pkg); err != nil {
		Logger.Error(funcName+" proto.Unmarshal failed", zap.Error(err), zap.Any("data.size", len(data)))
		return err
	}

	var msgStr string
	if pkg.GetOp() == 0 {
		msgStr = fmt.Sprintf("加入讨论组:%s[%d]-%s", pkg.GetGroupName(), pkg.GetGroup(), timeStampToString(pkg.GetTimeStamp()))
	} else {
		msgStr = fmt.Sprintf("退出讨论组:%s[%d]-%s", pkg.GetGroupName(), pkg.GetGroup(), timeStampToString(pkg.GetTimeStamp()))
	}
	msgPrintStatus(0, pkg.GetPeer(), pkg.GetName(), msgStr)
	return nil
}
