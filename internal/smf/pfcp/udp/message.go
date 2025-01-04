// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// Copyright 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

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
