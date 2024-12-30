package message_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp/message"
	"github.com/ellanetworks/core/internal/smf/pfcp/udp"
	"github.com/wmnsk/go-pfcp/ie"
	pfcp_message "github.com/wmnsk/go-pfcp/message"
	"go.uber.org/zap"
)

func TestSendPfcpAssociationSetupRequest(t *testing.T) {
	upNodeID := context.NodeID{
		NodeIdType:  context.NodeIdTypeIpv4Address,
		NodeIdValue: net.ParseIP("127.0.0.1").To4(),
	}
	localAddress := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8801,
	}

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

	err = message.SendPfcpAssociationSetupRequest(upNodeID, 8801)
	if err != nil {
		t.Errorf("error sending PFCP Association Setup Request: %v", err)
	}
}

func TestSendPfcpAssociationSetupResponse(t *testing.T) {
	localAddress := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8802,
	}

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

	upNodeID := context.NodeID{
		NodeIdType:  context.NodeIdTypeIpv4Address,
		NodeIdValue: net.ParseIP("127.0.0.1").To4(),
	}

	err = message.SendPfcpAssociationSetupResponse(upNodeID, ie.CauseRequestAccepted, 8802)
	if err != nil {
		t.Errorf("error sending PFCP Association Setup Response: %v", err)
	}
}

// When the User Plane Node exists in the stored context, then the PFCP Session Establishment Request is sent
func TestSendPfcpSessionEstablishmentRequestUpNodeExists(t *testing.T) {
	const upNodeIDStr = "127.0.0.1"
	upNodeID := context.NodeID{
		NodeIdType:  context.NodeIdTypeIpv4Address,
		NodeIdValue: net.ParseIP(upNodeIDStr).To4(),
	}
	config := zap.NewProductionConfig()
	log, err := config.Build()
	if err != nil {
		panic(err)
	}
	mockLog := log.Sugar()
	smContext := &context.SMContext{
		PFCPContext: map[string]*context.PFCPSessionContext{
			upNodeIDStr: {
				NodeID: upNodeID,
			},
		},
		SubPduSessLog: mockLog,
		SubPfcpLog:    mockLog,
	}

	pdrList := []*context.PDR{}
	farList := []*context.FAR{}
	barList := []*context.BAR{}
	qerList := []*context.QER{}

	localAddress := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8803,
	}

	conn, err := net.ListenUDP("udp", localAddress)
	if err != nil {
		t.Fatalf("error listening on UDP: %v", err)
	}

	defer func() {
		if err = conn.Close(); err != nil {
			t.Logf("error closing connection: %v", err)
		}
	}()

	udp.Server = &udp.PfcpServer{
		Conn: conn,
	}

	err = message.SendPfcpSessionEstablishmentRequest(upNodeID, smContext, pdrList, farList, barList, qerList, 8803)
	if err != nil {
		t.Errorf("error sending PFCP Session Establishment Request: %v", err)
	}
}

// Given the User Plane Node does not exist in the stored context, then the PFCP Session Establishment Request is not sent
func TestSendPfcpSessionEstablishmentRequestUpNodeDoesNotExist(t *testing.T) {
	const upNodeIDStr = "127.0.0.1"
	upNodeID := context.NodeID{
		NodeIdType:  context.NodeIdTypeIpv4Address,
		NodeIdValue: net.ParseIP(upNodeIDStr).To4(),
	}
	smContext := &context.SMContext{}

	pdrList := []*context.PDR{}
	farList := []*context.FAR{}
	barList := []*context.BAR{}
	qerList := []*context.QER{}

	localAddress := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8804,
	}

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

	err = message.SendPfcpSessionEstablishmentRequest(upNodeID, smContext, pdrList, farList, barList, qerList, 8804)
	if err == nil {
		t.Errorf("expected error sending PFCP Session Establishment Request")
	}
}

func TestSendPfcpSessionModificationRequest(t *testing.T) {
	const upNodeIDStr = "127.0.0.1"
	upNodeID := context.NodeID{
		NodeIdType:  context.NodeIdTypeIpv4Address,
		NodeIdValue: net.ParseIP(upNodeIDStr).To4(),
	}
	config := zap.NewProductionConfig()
	log, err := config.Build()
	if err != nil {
		panic(err)
	}
	mockLog := log.Sugar()
	smContext := &context.SMContext{
		PFCPContext: map[string]*context.PFCPSessionContext{
			upNodeIDStr: {
				NodeID: upNodeID,
			},
		},
		SubPduSessLog: mockLog,
		SubPfcpLog:    mockLog,
	}

	pdrList := []*context.PDR{}
	farList := []*context.FAR{}
	barList := []*context.BAR{}
	qerList := []*context.QER{}

	localAddress := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8806,
	}

	conn, err := net.ListenUDP("udp", localAddress)
	if err != nil {
		t.Fatalf("error listening on UDP: %v", err)
	}

	defer func() {
		if err = conn.Close(); err != nil {
			t.Logf("error closing connection: %v", err)
		}
	}()

	udp.Server = &udp.PfcpServer{
		Conn: conn,
	}

	err = message.SendPfcpSessionModificationRequest(upNodeID, smContext, pdrList, farList, barList, qerList, 8806)
	if err != nil {
		t.Errorf("error sending PFCP Session Modification Request: %v", err)
	}
}

func TestSendPfcpSessionDeletionRequest(t *testing.T) {
	const upNodeIDStr = "127.0.0.1"
	upNodeID := context.NodeID{
		NodeIdType:  context.NodeIdTypeIpv4Address,
		NodeIdValue: net.ParseIP(upNodeIDStr).To4(),
	}
	config := zap.NewProductionConfig()
	log, err := config.Build()
	if err != nil {
		panic(err)
	}
	mockLog := log.Sugar()
	smContext := &context.SMContext{
		PFCPContext: map[string]*context.PFCPSessionContext{
			upNodeIDStr: {
				NodeID: upNodeID,
			},
		},
		SubPduSessLog: mockLog,
		SubPfcpLog:    mockLog,
	}

	localAddress := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8807,
	}

	conn, err := net.ListenUDP("udp", localAddress)
	if err != nil {
		t.Fatalf("error listening on UDP: %v", err)
	}

	defer func() {
		if err = conn.Close(); err != nil {
			t.Logf("error closing connection: %v", err)
		}
	}()

	udp.Server = &udp.PfcpServer{
		Conn: conn,
	}

	err = message.SendPfcpSessionDeletionRequest(upNodeID, smContext, 8807)
	if err != nil {
		t.Errorf("error sending PFCP Session Deletion Request: %v", err)
	}
}

func TestSendPfcpSessionReportResponse(t *testing.T) {
	const upNodeIDStr = "127.0.0.1"
	remoteAddr := &net.UDPAddr{
		IP:   net.ParseIP(upNodeIDStr),
		Port: 8808,
	}

	localAddress := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8808,
	}

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

	flags := context.PFCPSRRspFlags{}
	err = message.SendPfcpSessionReportResponse(remoteAddr, ie.CauseRequestAccepted, flags, 1, 1)
	if err != nil {
		t.Errorf("error sending PFCP Session Report Response: %v", err)
	}
}

func TestSendHeartbeatRequest(t *testing.T) {
	const upNodeIDStr = "127.0.0.1"
	upNodeID := context.NodeID{
		NodeIdType:  context.NodeIdTypeIpv4Address,
		NodeIdValue: net.ParseIP(upNodeIDStr).To4(),
	}

	localAddress := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8809,
	}

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

	err = message.SendHeartbeatRequest(upNodeID, 8809)
	if err != nil {
		t.Errorf("error sending Heartbeat Request: %v", err)
	}
}

func TestSendHeartbeatResponse(t *testing.T) {
	const upNodeIDStr = "127.0.0.1"
	remoteAddr := &net.UDPAddr{
		IP:   net.ParseIP(upNodeIDStr),
		Port: 7001,
	}

	localAddress := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8810,
	}

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

	err = message.SendHeartbeatResponse(remoteAddr, 1)
	if err != nil {
		t.Errorf("error sending Heartbeat Response: %v", err)
	}
}

func JsonBodyToPfcpHeartbeatReq(body []byte) pfcp_message.HeartbeatRequest {
	heartbeatRequest, err := pfcp_message.ParseHeartbeatRequest(body)
	if err != nil {
		panic(fmt.Sprintf("error parsing JSON: %v", err))
	}
	return *heartbeatRequest
}

func TestSendPfcpMsgToAdapter(t *testing.T) {
	timestamp := time.Now()
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected method %s, got %s", http.MethodPost, r.Method)
			return
		}

		// Validate request body
		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("could not read request body: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		var udpPodMsg message.UdpPodPfcpMsg
		err = json.Unmarshal(reqBody, &udpPodMsg)
		if err != nil {
			t.Fatalf("error unmarshalling JSON: %v", err)
			return
		}

		heartbeatRequest := JsonBodyToPfcpHeartbeatReq(udpPodMsg.Msg.Body)
		if heartbeatRequest.RecoveryTimeStamp == nil {
			t.Fatalf("expected RecoveryTimeStamp, got nil")
			return
		}

		receivedTimestamp, err := heartbeatRequest.RecoveryTimeStamp.RecoveryTimeStamp()
		if err != nil {
			t.Fatalf("error getting RecoveryTimeStamp: %v", err)
			return
		}
		if timestamp.Truncate(1*time.Second) != receivedTimestamp.Truncate(1*time.Second) {
			t.Fatalf("expected timestamp %v, got %v", timestamp, receivedTimestamp)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	upNodeID := context.NewNodeID("testNodeID")
	msg := pfcp_message.NewHeartbeatRequest(
		1,
		ie.NewRecoveryTimeStamp(timestamp),
		nil,
	)
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	rsp, err := message.SendPfcpMsgToAdapter(*upNodeID, msg, addr, nil, testServer.URL)
	if err != nil {
		t.Fatalf("error sending PFCP message to adapter: %v", err)
	}
	defer func() {
		if err := rsp.Body.Close(); err != nil {
			log.Fatalf("Failed to close database: %v", err)
		}
	}()
	if rsp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rsp.StatusCode)
	}
}
