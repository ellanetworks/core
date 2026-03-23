// Copyright 2026 Ella Networks

package gmm

import (
	"context"
	"testing"

	amfContext "github.com/ellanetworks/core/internal/amf"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

// TestHandleGmmMessage_UnknownMessageType_Error verifies the default branch
// returns an error for an unrecognized message type.
func TestHandleGmmMessage_UnknownMessageType_Error(t *testing.T) {
	ue := amfContext.NewAmfUe()
	amf := amfContext.New(nil, nil, nil)

	m := nas.NewGmmMessage()
	m.SetMessageType(0xFF) // unassigned message type

	err := HandleGmmMessage(context.Background(), amf, ue, m)
	if err == nil {
		t.Fatal("expected error for unknown message type, got nil")
	}

	expected := "message type 255 handling not implemented"
	if err.Error() != expected {
		t.Fatalf("expected error %q, got %q", expected, err.Error())
	}
}

// TestHandleGmmMessage_DispatchesToConfigurationUpdateComplete verifies that
// HandleGmmMessage correctly dispatches a ConfigurationUpdateComplete message
// to handleConfigurationUpdateComplete. We use a Registered UE so the handler
// succeeds, confirming proper dispatch.
func TestHandleGmmMessage_DispatchesToConfigurationUpdateComplete(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceState(amfContext.Registered)

	amf := amfContext.New(nil, nil, nil)

	m := nas.NewGmmMessage()
	cuc := nasMessage.NewConfigurationUpdateComplete(0)
	cuc.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	cuc.SetSpareHalfOctet(0x00)
	cuc.SetMessageType(nas.MsgTypeConfigurationUpdateComplete)

	m.ConfigurationUpdateComplete = cuc
	m.SetMessageType(nas.MsgTypeConfigurationUpdateComplete)

	err = HandleGmmMessage(context.Background(), amf, ue, m)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// TestHandleGmmMessage_DispatchesToStatus5GMM verifies that HandleGmmMessage
// correctly dispatches a Status5GMM message to handleStatus5GMM. We use a
// Registered UE so the handler succeeds, confirming proper dispatch.
func TestHandleGmmMessage_DispatchesToStatus5GMM(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceState(amfContext.Registered)

	amf := amfContext.New(nil, nil, nil)

	m := buildTestStatus5gmm()

	err = HandleGmmMessage(context.Background(), amf, ue, m)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
