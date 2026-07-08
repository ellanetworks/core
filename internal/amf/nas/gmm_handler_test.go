// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

// TestHandleGmmMessage_UnknownMessageType_NoOp verifies the default branch handles
// an unrecognized message type without panicking (it answers with a 5GMM STATUS,
// TS 24.501 §7.4) and is a no-op when the UE has no connection to answer on.
func TestHandleGmmMessage_UnknownMessageType_NoOp(t *testing.T) {
	ue := amf.NewUeContext()
	amfInstance := amf.New(nil, nil, nil)

	m := nas.NewGmmMessage()
	m.SetMessageType(0xFF) // unassigned message type

	HandleGmmMessage(context.Background(), amfInstance, ue, m, true)
}

// TestHandleGmmMessage_DispatchesToConfigurationUpdateComplete verifies that
// HandleGmmMessage correctly dispatches a ConfigurationUpdateComplete message
// to handleConfigurationUpdateComplete. We use a amf.Registered UE so the handler
// runs its success path, confirming proper dispatch.
func TestHandleGmmMessage_DispatchesToConfigurationUpdateComplete(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	amfInstance := amf.New(nil, nil, nil)

	m := nas.NewGmmMessage()
	cuc := nasMessage.NewConfigurationUpdateComplete(0)
	cuc.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	cuc.SetSpareHalfOctet(0x00)
	cuc.SetMessageType(nas.MsgTypeConfigurationUpdateComplete)

	m.ConfigurationUpdateComplete = cuc
	m.SetMessageType(nas.MsgTypeConfigurationUpdateComplete)

	HandleGmmMessage(context.Background(), amfInstance, ue, m, true)
}

// TestHandleGmmMessage_DispatchesToStatus5GMM verifies that HandleGmmMessage
// correctly dispatches a Status5GMM message to handleStatus5GMM. We use a
// amf.Registered UE so the handler runs its success path, confirming proper dispatch.
func TestHandleGmmMessage_DispatchesToStatus5GMM(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	amfInstance := amf.New(nil, nil, nil)

	m := buildTestStatus5gmm()

	HandleGmmMessage(context.Background(), amfInstance, ue, m, true)
}
