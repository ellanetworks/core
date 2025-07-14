package core_test

import (
	"context"
	"net"
	"testing"

	"github.com/ellanetworks/core/internal/upf/core"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

func TestGetTransportLevelMarking(t *testing.T) {
	// Create CreateFAR_IE with TransportLevelMarking
	CreateFAR := ie.NewCreateFAR(
		ie.NewFARID(10),
		ie.NewTransportLevelMarking(55),
	)

	tlm, err := core.GetTransportLevelMarking(CreateFAR)
	if err != nil {
		t.Errorf("Error getting TransportLevelMarking: %s", err.Error())
	}
	if tlm != 55 {
		t.Errorf("Expected TransportLevelMarking to be 55, got %d", tlm)
	}
}

func TestHandlePfcpSessionModificationRequestCauseNoEstablishedPFCPAssociation(t *testing.T) {
	_, err := core.CreatePfcpConnection(
		"1.2.3.4",
		"nodeId",
		"2.3.4.5",
		"1.1.1.1",
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
	response, err := core.HandlePfcpSessionModificationRequest(context.Background(), msg)
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
		"1.1.1.1",
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
	response, err := core.HandlePfcpSessionModificationRequest(context.Background(), msg)
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

func TestHandlePfcpSessionDeletionRequestCauseRequestAccepted(t *testing.T) {
	conn, err := core.CreatePfcpConnection(
		"1.2.3.4",
		"nodeId",
		"2.3.4.5",
		"1.1.1.1",
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

	msg := message.NewSessionDeletionRequest(
		0,
		0,
		remoteSEID,
		sequenceNumber,
		0,
		ies...,
	)
	response, err := core.HandlePfcpSessionDeletionRequest(context.Background(), msg)
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

func TestHandlePfcpSessionDeletionRequestCauseNoEstablishedPFCPAssociation(t *testing.T) {
	_, err := core.CreatePfcpConnection(
		"1.2.3.4",
		"nodeId",
		"2.3.4.5",
		"1.1.1.1",
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

	msg := message.NewSessionDeletionRequest(
		0,
		0,
		remoteSEID,
		sequenceNumber,
		0,
		ies...,
	)
	response, err := core.HandlePfcpSessionDeletionRequest(context.Background(), msg)
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

func TestHandlePfcpSessionDeletionRequestCauseSessionContextNotFound(t *testing.T) {
	conn, err := core.CreatePfcpConnection(
		"1.2.3.4",
		"nodeId",
		"2.3.4.5",
		"1.1.1.1",
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
	response, err := core.HandlePfcpSessionModificationRequest(context.Background(), msg)
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
		"1.1.1.1",
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
	response, err := core.HandlePfcpSessionModificationRequest(context.Background(), msg)
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
