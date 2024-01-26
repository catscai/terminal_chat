package pb

type PackType uint32

// client -> server
const (
	PackRegisterRQ PackType = iota
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
)

// server -> client
const (
	PackPublishPersonalMsgRQ PackType = iota + 10000
	PackPublishPersonalMsgRS
	PackPublishGroupMsgRQ
	PackPublishGroupMsgRS
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
)
