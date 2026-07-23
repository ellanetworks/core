// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/nas/fgs"
)

// TestHandleGmmMessage_UnknownMessageType_NoOp verifies the default branch handles
// an unrecognized message type without panicking (it answers with a 5GMM STATUS,
// TS 24.501 §7.4) and is a no-op when the UE has no connection to answer on.
func TestHandleGmmMessage_UnknownMessageType_NoOp(t *testing.T) {
	ue := amf.NewUeContext()
	amfInstance := amf.New(nil, nil, nil)

	HandleGmmMessage(context.Background(), amfInstance, ue, 0xFF, nil, true) // unassigned message type
}

// TestHandleGmmMessage_DispatchesToConfigurationUpdateComplete verifies HandleGmmMessage
// routes a ConfigurationUpdateComplete to handleConfigurationUpdateComplete; a
// amf.Registered UE lets the handler run its success path.
func TestHandleGmmMessage_DispatchesToConfigurationUpdateComplete(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	amfInstance := amf.New(nil, nil, nil)

	HandleGmmMessage(context.Background(), amfInstance, ue, uint8(fgs.MsgConfigurationUpdateComplete), nil, true)
}

// TestHandleGmmMessage_DispatchesToStatus5GMM verifies HandleGmmMessage routes a
// GMMStatus to handleStatus5GMM; a amf.Registered UE lets the handler run its success path.
func TestHandleGmmMessage_DispatchesToStatus5GMM(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	amfInstance := amf.New(nil, nil, nil)

	HandleGmmMessage(context.Background(), amfInstance, ue, uint8(fgs.MsgGMMStatus), buildTestStatus5gmm(t), true)
}
