package process

import (
	"github.com/catscai/ccat/clog"
	"github.com/catscai/ccat/impl/msg"
	"github.com/catscai/terminal_chat/pb"
	"github.com/catscai/terminal_chat/pb/src/allpb"
	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
)

func AsyncSendGroupMemberChange(groupInfo *TempGroupInfo, op int32, own int64, name string, t int64) {
	groupInfo.Members.Range(func(key, value interface{}) bool {
		peer := key.(int64)
		val, ok := GUserSession.Load(peer)
		if !ok || val == nil {
			return true
		}
		if peer == own {
			return true
		}
		sess := val.(*UserSession)

		_ = GAsyncIoPool.SendTask(uint64(peer), func() {
			msgRq := &allpb.PublishGroupMemberChangeRQ{
				Op:        &op,
				Peer:      &own,
				Name:      &name,
				Group:     &groupInfo.Group,
				GroupName: &groupInfo.Name,
				TimeStamp: &t,
			}
			data, _ := proto.Marshal(msgRq)
			pkg := GPackOp.Full(pb.PackPublishGroupMemberChangeRQ, data, &msg.DefaultHeader{})
			_ = sess.conn.SendMsg(pkg)
		})
		return true
	})
}

func AsyncSendToGroup(ctx clog.ICatLog, groupInfo *TempGroupInfo, id int64, name, content string, t int64) {
	const funcName = "AsyncSendToGroup"
	msgRq := &allpb.PublishGroupMsgRQ{
		Group:     &groupInfo.Group,
		Name:      &groupInfo.Name,
		Content:   &content,
		TimeStamp: &t,
		Peer:      &id,
		PeerName:  &name,
	}
	data, _ := proto.Marshal(msgRq)
	pkg := GPackOp.Full(pb.PackPublishGroupMsgRQ, data, &msg.DefaultHeader{})

	//广播给组成员
	groupInfo.Members.Range(func(key, value interface{}) bool {
		uid := key.(int64)
		if uid == id {
			// 发送给组的消息 过滤掉自己
			return true
		}
		sessValue, ok := GUserSession.Load(uid)
		if !ok || sessValue == nil {
			return true
		}
		sess := sessValue.(*UserSession)

		_ = GAsyncIoPool.SendTask(uint64(uid), func() {
			if err := sess.conn.SendMsg(pkg); err != nil {
				ctx.Error(funcName+" SendMsg failed", zap.Any("peer", uid), zap.Any("group", groupInfo.Group), zap.Error(err))
			}
		})

		return true
	})

}
