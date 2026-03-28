// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleUEContextReleaseRequest_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.UEContextReleaseRequest{}

	assertNoPanic(t, "HandleUEContextReleaseRequest(empty IEs)", func() {
		ngap.HandleUEContextReleaseRequest(context.Background(), amf, ran, msg)
	})
}

// TestHandleUEContextReleaseRequest_UnknownUENGAPIDs verifies that a
// UEContextReleaseRequest with AMF_UE_NGAP_ID and RAN_UE_NGAP_ID values that
// don't match any existing UE context is handled gracefully — no panic, and an
// ErrorIndication with UnknownLocalUENGAPID cause is sent back.
// Regression test inspired by open5gs/open5gs#4371.
func TestHandleUEContextReleaseRequest_UnknownUENGAPIDs(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	msg := &ngapType.UEContextReleaseRequest{}
	ies := &msg.ProtocolIEs

	// AMF UE NGAP ID — bogus value with no matching context
	amfIE := ngapType.UEContextReleaseRequestIEs{}
	amfIE.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	amfIE.Value.Present = ngapType.UEContextReleaseRequestIEsPresentAMFUENGAPID
	amfIE.Value.AMFUENGAPID = &ngapType.AMFUENGAPID{Value: 999999}
	ies.List = append(ies.List, amfIE)

	// RAN UE NGAP ID — bogus value
	ranIE := ngapType.UEContextReleaseRequestIEs{}
	ranIE.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ranIE.Value.Present = ngapType.UEContextReleaseRequestIEsPresentRANUENGAPID
	ranIE.Value.RANUENGAPID = &ngapType.RANUENGAPID{Value: 888888}
	ies.List = append(ies.List, ranIE)

	// Cause
	causeIE := ngapType.UEContextReleaseRequestIEs{}
	causeIE.Id.Value = ngapType.ProtocolIEIDCause
	causeIE.Value.Present = ngapType.UEContextReleaseRequestIEsPresentCause
	causeIE.Value.Cause = &ngapType.Cause{
		Present:      ngapType.CausePresentRadioNetwork,
		RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUserInactivity},
	}
	ies.List = append(ies.List, causeIE)

	assertNoPanic(t, "HandleUEContextReleaseRequest(unknown IDs)", func() {
		ngap.HandleUEContextReleaseRequest(context.Background(), amfInstance, ran, msg)
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

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(sender.SentUEContextReleaseCommands))
	}
}

// TestHandleUEContextReleaseRequest_NilMessage verifies that passing a nil
// message does not panic.
func TestHandleUEContextReleaseRequest_NilMessage(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	assertNoPanic(t, "HandleUEContextReleaseRequest(nil)", func() {
		ngap.HandleUEContextReleaseRequest(context.Background(), amfInstance, ran, nil)
	})
}
