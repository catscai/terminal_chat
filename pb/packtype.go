package pb

// client -> server
const (
	PackRegisterRQ uint32 = iota
	PackRegisterRS
	PackLoginRQ
	PackLoginRS
	PackSendToPersonalRQ
	PackSendToPersonalRS
	PackSendToGroupRQ
	PackSendToGroupRS
	PackSubscribePersonalMsgRQ
	PackSubscribePersonalMsgRS
	PackSubscribeGroupMsgRQ
	PackSubscribeGroupMsgRS
	PackCancelSubscribeAllRQ
	PackCancelSubscribeAllRS
	PackCreateGroupRQ
	PackCreateGroupRS
	PackJoinGroupRQ
	PackJoinGroupRS
	PackGroupMemberRQ
	PackGroupMemberRS
	PackSelfRelationRQ
	PackSelfRelationRS
	PackSendWorldMsgRQ
	PackSendWorldMsgRS
)

// server -> client
const (
	PackPublishPersonalMsgRQ uint32 = iota + 10000
	PackPublishPersonalMsgRS
	PackPublishGroupMsgRQ
	PackPublishGroupMsgRS
	PackPublishSubscribeMsgRQ
	PackPublishSubscribeMsgRS
	PackPublishGroupMemberChangeRQ
	PackPublishGroupMemberChangeRS
)

const (
	CodeOK int32 = iota + 10000
	CodeRegisterError
	CodeLoginError
	CodeSendError
	CodeSubscribeError
	CodeJoinError
	CodeCreateGroupError
	CodeCancelSubscribeError
	CodeGroupMemberError
	CodeSelfRelationError
	CodeSendWorldMsgError
)
