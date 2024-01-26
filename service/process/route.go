package process

import (
	"github.com/catscai/ccat/iface"
	"github.com/catscai/terminal_chat/pb"
	"github.com/catscai/terminal_chat/pb/src/allpb"
)

func RegisterHandler(dispatcher iface.IDispatcher) {
	dispatcher.RegisterHandlerSimplePB(pb.PackRegisterRQ, pb.PackRegisterRS, &allpb.RegisterRQ{}, &allpb.RegisterRS{}, HandleRegisterRQ)
	dispatcher.RegisterHandlerSimplePB(pb.PackLoginRQ, pb.PackLoginRS, &allpb.LoginRQ{}, &allpb.LoginRS{}, HandleLoginRQ)
	dispatcher.RegisterHandlerSimplePB(pb.PackSendToPersonalRQ, pb.PackSendToPersonalRS, &allpb.SendToPersonalRQ{}, &allpb.SendToPersonalRS{}, HandleSendToPersonalRQ)
	dispatcher.RegisterHandlerSimplePB(pb.PackSendToGroupRQ, pb.PackSendToGroupRS, &allpb.SendToTempGroupRQ{}, &allpb.SendToTempGroupRS{}, HandleSendToGroupRQ)
	dispatcher.RegisterHandlerSimplePB(pb.PackSubscribePersonalMsgRQ, pb.PackSubscribePersonalMsgRS, &allpb.SubscribePersonRQ{}, &allpb.SubscribePersonRS{}, HandleSubscribePersonalRQ)
	dispatcher.RegisterHandlerSimplePB(pb.PackSubscribeGroupMsgRQ, pb.PackSubscribeGroupMsgRS, &allpb.SubscribeTempGroupRQ{}, &allpb.SubscribeTempGroupRS{}, HandleSubscribeGroupRQ)
	dispatcher.RegisterHandlerSimplePB(pb.PackCancelSubscribeAllRQ, pb.PackCancelSubscribeAllRS, &allpb.CancelSubscribeAllRQ{}, &allpb.CancelSubscribeAllRS{}, HandleCancelSubscribeAllRQ)
	dispatcher.RegisterHandlerSimplePB(pb.PackCreateGroupRQ, pb.PackCreateGroupRS, &allpb.CreateTempGroupRQ{}, &allpb.CreateTempGroupRS{}, HandleCreateGroupRQ)
	dispatcher.RegisterHandlerSimplePB(pb.PackJoinGroupRQ, pb.PackJoinGroupRS, &allpb.JoinGroupRQ{}, &allpb.JoinGroupRS{}, HandleJoinGroupRQ)
}
