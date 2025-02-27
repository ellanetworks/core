package core_test

import (
	"net"
	"testing"

	"github.com/ellanetworks/core/internal/upf/core"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

func TestHandlePfcpSessionModificationRequestCauseNoEstablishedPFCPAssociation(t *testing.T) {
	_, err := core.CreatePfcpConnection(
		"1.2.3.4",
		"nodeId",
		"2.3.4.5",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Error creating PFCP connection: %v", err)
	}
	remoteSEID := uint64(1)
	sequenceNumber := uint32(1)
	localSEID := uint64(2)
	fseidIPv4Address := net.ParseIP("1.2.3.4")
	ies := make([]*ie.IE, 0)
	ies = append(ies, ie.NewFSEID(localSEID, fseidIPv4Address, nil))

	msg := message.NewSessionModificationRequest(
		0,
		0,
		remoteSEID,
		sequenceNumber,
		0,
		ies...,
	)
	response, err := core.HandlePfcpSessionModificationRequest(msg)
	if err != nil {
		t.Fatalf("Error handling session modification request: %v", err)
	}
	if response == nil {
		t.Fatalf("Response is nil")
	}
	CauseIE, err := response.Cause.Cause()
	if err != nil {
		t.Fatalf("Error getting Cause IE: %v", err)
	}
	if CauseIE != ie.CauseNoEstablishedPFCPAssociation {
		t.Fatalf("Cause IE is not CauseNoEstablishedPFCPAssociation: %v", CauseIE)
	}
}

func TestHandlePfcpSessionModificationRequestCauseSessionContextNotFound(t *testing.T) {
	conn, err := core.CreatePfcpConnection(
		"1.2.3.4",
		"nodeId",
		"2.3.4.5",
		nil,
		nil,
	)
	conn.SmfNodeAssociation = &core.NodeAssociation{
		ID: "nodeId",
	}
	if err != nil {
		t.Fatalf("Error creating PFCP connection: %v", err)
	}
	remoteSEID := uint64(1)
	sequenceNumber := uint32(1)
	localSEID := uint64(2)
	fseidIPv4Address := net.ParseIP("1.2.3.4")
	ies := make([]*ie.IE, 0)
	ies = append(ies, ie.NewFSEID(localSEID, fseidIPv4Address, nil))

	msg := message.NewSessionModificationRequest(
		0,
		0,
		remoteSEID,
		sequenceNumber,
		0,
		ies...,
	)
	response, err := core.HandlePfcpSessionModificationRequest(msg)
	if err != nil {
		t.Fatalf("Error handling session modification request: %v", err)
	}
	if response == nil {
		t.Fatalf("Response is nil")
	}
	CauseIE, err := response.Cause.Cause()
	if err != nil {
		t.Fatalf("Error getting Cause IE: %v", err)
	}
	if CauseIE != ie.CauseSessionContextNotFound {
		t.Fatalf("Cause IE is not CauseSessionContextNotFound: %v", CauseIE)
	}
}

func TestHandlePfcpSessionModificationRequestCauseRequestAccepted(t *testing.T) {
	conn, err := core.CreatePfcpConnection(
		"1.2.3.4",
		"nodeId",
		"2.3.4.5",
		nil,
		nil,
	)
	remoteSEID := uint64(1)
	sequenceNumber := uint32(1)
	localSEID := uint64(2)
	fseidIPv4Address := net.ParseIP("1.2.3.4")
	conn.SmfNodeAssociation = &core.NodeAssociation{
		ID:       "nodeId",
		Sessions: make(map[uint64]*core.Session),
	}
	conn.SmfNodeAssociation.Sessions[remoteSEID] = &core.Session{}
	if err != nil {
		t.Fatalf("Error creating PFCP connection: %v", err)
	}

	ies := make([]*ie.IE, 0)
	ies = append(ies, ie.NewFSEID(localSEID, fseidIPv4Address, nil))

	msg := message.NewSessionModificationRequest(
		0,
		0,
		remoteSEID,
		sequenceNumber,
		0,
		ies...,
	)
	response, err := core.HandlePfcpSessionModificationRequest(msg)
	if err != nil {
		t.Fatalf("Error handling session modification request: %v", err)
	}
	if response == nil {
		t.Fatalf("Response is nil")
	}
	CauseIE, err := response.Cause.Cause()
	if err != nil {
		t.Fatalf("Error getting Cause IE: %v", err)
	}
	if CauseIE != ie.CauseRequestAccepted {
		t.Fatalf("Cause IE is not CauseRequestAccepted: %v", CauseIE)
	}
}
