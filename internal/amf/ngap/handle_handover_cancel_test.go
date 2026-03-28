// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleHandoverCancel_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	msg := &ngapType.HandoverCancel{}

	assertNoPanic(t, "HandleHandoverCancel(empty IEs)", func() {
		ngap.HandleHandoverCancel(context.Background(), ran, msg)
	})
}

// TestHandleHandoverCancel_UnknownRanUeNgapID verifies that a HandoverCancel
// with a RAN_UE_NGAP_ID that doesn't match any existing UE context is handled
// gracefully — no panic, and an ErrorIndication is sent.
// Regression test inspired by open5gs/open5gs#4378.
func TestHandleHandoverCancel_UnknownRanUeNgapID(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	msg := &ngapType.HandoverCancel{}
	ies := &msg.ProtocolIEs

	// AMF UE NGAP ID
	amfIE := ngapType.HandoverCancelIEs{}
	amfIE.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	amfIE.Value.Present = ngapType.HandoverCancelIEsPresentAMFUENGAPID
	amfIE.Value.AMFUENGAPID = &ngapType.AMFUENGAPID{Value: 1099511627776}
	ies.List = append(ies.List, amfIE)

	// RAN UE NGAP ID — bogus value
	ranIE := ngapType.HandoverCancelIEs{}
	ranIE.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ranIE.Value.Present = ngapType.HandoverCancelIEsPresentRANUENGAPID
	ranIE.Value.RANUENGAPID = &ngapType.RANUENGAPID{Value: 99}
	ies.List = append(ies.List, ranIE)

	// Cause
	causeIE := ngapType.HandoverCancelIEs{}
	causeIE.Id.Value = ngapType.ProtocolIEIDCause
	causeIE.Value.Present = ngapType.HandoverCancelIEsPresentCause
	causeIE.Value.Cause = &ngapType.Cause{
		Present:      ngapType.CausePresentRadioNetwork,
		RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem},
	}
	ies.List = append(ies.List, causeIE)

	assertNoPanic(t, "HandleHandoverCancel(unknown RAN UE NGAP ID)", func() {
		ngap.HandleHandoverCancel(context.Background(), ran, msg)
	})

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}

	errInd := sender.SentErrorIndications[0]
	if errInd.Cause == nil || errInd.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Fatal("expected RadioNetwork cause in ErrorIndication")
	}

	if errInd.Cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID {
		t.Fatalf("expected UnknownLocalUENGAPID, got %d", errInd.Cause.RadioNetwork.Value)
	}
}

// TestHandleHandoverCancel_NilMessage verifies that passing a nil message does
// not panic.
func TestHandleHandoverCancel_NilMessage(t *testing.T) {
	ran := newTestRadio()

	assertNoPanic(t, "HandleHandoverCancel(nil)", func() {
		ngap.HandleHandoverCancel(context.Background(), ran, nil)
	})
}
