package udp

type EventType uint8

const (
	ReceiveResendRequest EventType = iota
	ReceiveValidResponse
)
