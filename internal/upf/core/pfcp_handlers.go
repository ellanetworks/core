// Copyright 2024 Ella Networks
package core

import (
	"net"

	"github.com/ellanetworks/core/internal/logger"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

type PfcpFunc func(conn *PfcpConnection, msg message.Message, addr string) (message.Message, error)

type PfcpHandlerMap map[uint8]PfcpFunc

func (handlerMap PfcpHandlerMap) Handle(conn *PfcpConnection, buf []byte, addr *net.UDPAddr) error {
	incomingMsg, err := message.Parse(buf)
	if err != nil {
		logger.UpfLog.Warnf("Ignored undecodable message: %x, error: %s", buf, err)
		return err
	}
	if handler, ok := handlerMap[incomingMsg.MessageType()]; ok {
		stringIpAddr := addr.IP.String()
		if stringIpAddr == "::1" {
			logger.UpfLog.Debugf("Got loopback address, setting to 0.0.0.0")
			stringIpAddr = "0.0.0.0"
		}
		outgoingMsg, err := handler(conn, incomingMsg, stringIpAddr)
		if err != nil {
			logger.UpfLog.Warnf("Error handling PFCP message: %s", err.Error())
			return err
		}
		// Now assumption that all handlers will return a message to send is not true.
		if outgoingMsg != nil {
			return conn.SendMessage(outgoingMsg, addr)
		}
		return nil
	} else {
		logger.UpfLog.Warnf("Got unexpected message %s: %s, from: %s", incomingMsg.MessageTypeName(), incomingMsg, addr)
	}
	return nil
}

func newIeNodeID(nodeID string) *ie.IE {
	ip := net.ParseIP(nodeID)
	if ip != nil {
		if ip.To4() != nil {
			return ie.NewNodeID(nodeID, "", "")
		}
		return ie.NewNodeID("", nodeID, "")
	}
	return ie.NewNodeID("", "", nodeID)
}
