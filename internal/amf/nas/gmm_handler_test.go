// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
)

// TestHandleGmmMessage_UnknownMessageType_NoOp verifies the default branch handles
// an unrecognized message type without panicking (it answers with a 5GMM STATUS,
// TS 24.501 §7.4) and is a no-op when the UE has no connection to answer on.
func TestHandleGmmMessage_UnknownMessageType_NoOp(t *testing.T) {
	ue := amf.NewUeContext()
	amfInstance := amf.New(nil, nil, nil)

	HandleGmmMessage(context.Background(), amfInstance, ue, 0xFF, nil, true, false) // unassigned message type
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

	HandleGmmMessage(context.Background(), amfInstance, ue, uint8(fgs.MsgConfigurationUpdateComplete), nil, true, false)
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

	HandleGmmMessage(context.Background(), amfInstance, ue, uint8(fgs.MsgGMMStatus), buildTestStatus5gmmPlain(t), true, false)
}

// A SERVICE REQUEST is not confined to the first NAS message of a connection: TS 24.501
// §5.6.1.1 cases b), e), h), i), j) and o) are sent in 5GMM-CONNECTED mode, and §5.6.1.7 a)
// names that trigger explicitly. Such a request reaches the AMF in an NGAP UPLINK NAS
// TRANSPORT (TS 38.413 §8.6.3.1), so HandleGmmMessage must dispatch it to the service
// request procedure rather than answer 5GMM STATUS #97 (§7.4), whose cause is untrue of a
// message type the AMF implements.
func TestHandleGmmMessage_DispatchesToServiceRequest(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: `["000001"]`},
		},
		nil,
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	ue.SetSecuredForTest(true)

	plain := encSR(t, buildTestServiceRequest())

	got := HandleGmmMessage(context.Background(), amfInstance, ue, uint8(fgs.MsgServiceRequest), plain, true, false)

	if got.Action == nasreply.ActionStatus && got.Cause == nasreply.CauseMessageTypeNotImplemented {
		t.Fatal("SERVICE REQUEST answered with 5GMM STATUS #97; it must run the service request procedure")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("downlinks = %d, want 1 (SERVICE ACCEPT)", len(ngapSender.SentDownlinkNASTransport))
	}
}

// The dispatch above does not weaken the integrity gate: SERVICE REQUEST is absent from the
// TS 24.501 §4.4.4.3 list of messages the AMF processes without integrity protection, so one
// arriving as plain NAS is discarded before it reaches the service request procedure.
func TestHandleNAS_PlainServiceRequest_Discarded(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: `["000001"]`},
		},
		nil,
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	ue.SetSecuredForTest(true)

	HandleNAS(context.Background(), amfInstance, ue.Conn(), encSR(t, buildTestServiceRequest()))

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("plain SERVICE REQUEST drew %d downlinks, want 0 (TS 24.501 §4.4.4.3)", len(ngapSender.SentDownlinkNASTransport))
	}
}
