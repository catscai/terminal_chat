package pb

type PackType uint16

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
)

// server -> client
const (
	PackPublishPersonalMsgRQ PackType = iota + 10000
	PackPublishPersonalMsgRS
	PackPublishGroupMsgRQ
	PackPublishGroupMsgRS
)

type ErrCodeType int32

const (
	CodeRegisterError ErrCodeType = iota + 10000
)
