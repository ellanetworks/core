// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package udp_test

import (
	"log"
	"net"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp/udp"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

var heartbeatRequestReceived bool

type Server struct {
	addr *net.UDPAddr
	Conn *net.UDPConn
}

func HandlePfcpHeartbeatRequestTest(msg *udp.Message) {
	heartbeatRequestReceived = true
}

func Dispatch(msg *udp.Message) {
	if msg.PfcpMessage == nil {
		return
	}
	msgType := msg.PfcpMessage.MessageType()
	switch msgType {
	case message.MsgTypeHeartbeatRequest:
		HandlePfcpHeartbeatRequestTest(msg)
	}
}

func (s *Server) Start() error {
	conn, err := net.ListenUDP("udp", s.addr)
	if err != nil {
		return err
	}
	s.Conn = conn
	return nil
}

func (s *Server) SendPFCPMessage(msg message.Message, remoteAddress *net.UDPAddr) error {
	buf := make([]byte, msg.MarshalLen())
	err := msg.MarshalTo(buf)
	if err != nil {
		return err
	}

	_, err = s.Conn.WriteToUDP(buf, remoteAddress)
	if err != nil {
		return err
	}
	return nil
}

func TestRun(t *testing.T) {
	context.SMF_Self().CPNodeID = context.NodeID{
		NodeIdType:  context.NodeIdTypeIpv4Address,
		NodeIdValue: net.ParseIP("127.0.0.1").To4(),
	}
	context.SMF_Self().PFCPPort = 8811

	localAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8811,
	}
	remoteAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 1234,
	}
	go udp.Run(Dispatch)
	err := udp.WaitForServer()
	if err != nil {
		t.Fatalf("failed to start PFCP server: %v", err)
	}

	if udp.Server == nil {
		t.Fatalf("expected Server to be initialized")
	}

	if udp.Server.Conn == nil {
		t.Fatalf("expected Server to be listening")
	}

	defer func() {
		if err = udp.Server.Conn.Close(); err != nil {
			log.Printf("error closing connection: %v", err)
		}
	}()

	setupRequest := message.NewHeartbeatRequest(
		1,
		ie.NewRecoveryTimeStamp(time.Now()),
		nil,
	)

	server := &Server{
		addr: remoteAddr,
	}
	err = server.Start()
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	err = server.SendPFCPMessage(setupRequest, localAddr)
	if err != nil {
		t.Fatalf("failed to send PFCP message: %v", err)
	}

	time.Sleep(1 * time.Second)

	if !heartbeatRequestReceived {
		t.Error("expected Heartbeat Request to be received")
	}
}

func TestServerSendPfcp(t *testing.T) {
	localAddress := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8807,
	}
	remoteAddress := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8808,
	}

	msg := message.NewAssociationSetupResponse(1)

	conn, err := net.ListenUDP("udp", localAddress)
	if err != nil {
		t.Fatalf("error listening on UDP: %v", err)
	}

	defer func() {
		if err = conn.Close(); err != nil {
			log.Printf("error closing connection: %v", err)
		}
	}()

	udp.Server = &udp.PfcpServer{
		Conn: conn,
	}

	err = udp.SendPfcp(msg, remoteAddress, nil)
	if err != nil {
		t.Errorf("failed to send PFCP message: %v", err)
	}
}

func TestServerNotSetSendPfcp(t *testing.T) {
	udp.Server = nil
	remoteAddress := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8806,
	}

	msg := message.NewAssociationSetupResponse(1)

	err := udp.SendPfcp(msg, remoteAddress, nil)

	if err == nil {
		t.Error("expected error, got nil")
	}
}
