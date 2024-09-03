package udp

import (
	"net"

	"github.com/wmnsk/go-pfcp/message"
)

type Message struct {
	RemoteAddr  *net.UDPAddr
	PfcpMessage message.Message
	EventData   interface{}
}

func NewMessage(remoteAddr *net.UDPAddr, pfcpMessage message.Message, eventData interface{}) (msg Message) {
	msg = Message{}
	msg.RemoteAddr = remoteAddr
	msg.PfcpMessage = pfcpMessage
	msg.EventData = eventData
	return
}
