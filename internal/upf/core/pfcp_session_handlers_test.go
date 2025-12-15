package core_test

import (
	"context"
	"net"
	"testing"

	"github.com/ellanetworks/core/internal/upf/core"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

func TestHandlePfcpSessionModificationRequestCauseSessionContextNotFound(t *testing.T) {
	_, err := core.CreatePfcpConnection(
		"1.2.3.4",
		"nodeId",
		"2.3.4.5",
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

	conn.AddSession(remoteSEID, &core.Session{})

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

func TestHandlePfcpSessionDeletionRequestCauseSessionContextNotFound(t *testing.T) {
	_, err := core.CreatePfcpConnection(
		"1.2.3.4",
		"nodeId",
		"2.3.4.5",
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

	conn.AddSession(remoteSEID, &core.Session{})

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
