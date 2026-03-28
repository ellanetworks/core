// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandlePDUSessionResourceSetupResponse_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	msg := &ngapType.PDUSessionResourceSetupResponse{}

	assertNoPanic(t, "HandlePDUSessionResourceSetupResponse(empty IEs)", func() {
		ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)
	})
}

// TestHandlePDUSessionResourceSetupResponse_UnknownAMFUENGAPID verifies that a
// PDUSessionResourceSetupResponse with an AMF_UE_NGAP_ID that doesn't match
// any existing UE context is handled gracefully — no panic, no further
// processing.
// Regression test inspired by open5gs/open5gs#4377.
func TestHandlePDUSessionResourceSetupResponse_UnknownAMFUENGAPID(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	msg := &ngapType.PDUSessionResourceSetupResponse{}
	ies := &msg.ProtocolIEs

	// AMF UE NGAP ID — bogus value with no matching context
	amfIE := ngapType.PDUSessionResourceSetupResponseIEs{}
	amfIE.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	amfIE.Value.Present = ngapType.PDUSessionResourceSetupResponseIEsPresentAMFUENGAPID
	amfIE.Value.AMFUENGAPID = &ngapType.AMFUENGAPID{Value: 1379640380095}
	ies.List = append(ies.List, amfIE)

	// RAN UE NGAP ID — bogus value
	ranIE := ngapType.PDUSessionResourceSetupResponseIEs{}
	ranIE.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ranIE.Value.Present = ngapType.PDUSessionResourceSetupResponseIEsPresentRANUENGAPID
	ranIE.Value.RANUENGAPID = &ngapType.RANUENGAPID{Value: 99}
	ies.List = append(ies.List, ranIE)

	assertNoPanic(t, "HandlePDUSessionResourceSetupResponse(unknown AMF UE NGAP ID)", func() {
		ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)
	})
}

// TestHandlePDUSessionResourceSetupResponse_NilMessage verifies that passing a
// nil message does not panic.
func TestHandlePDUSessionResourceSetupResponse_NilMessage(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	assertNoPanic(t, "HandlePDUSessionResourceSetupResponse(nil)", func() {
		ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, nil)
	})
}

// TestHandlePDUSessionResourceSetupResponse_OnlyUnknownRANUENGAPID verifies
// that when only the RAN_UE_NGAP_ID is present but unknown, the handler does
// not crash or process further.
func TestHandlePDUSessionResourceSetupResponse_OnlyUnknownRANUENGAPID(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	msg := &ngapType.PDUSessionResourceSetupResponse{}
	ies := &msg.ProtocolIEs

	ranIE := ngapType.PDUSessionResourceSetupResponseIEs{}
	ranIE.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ranIE.Value.Present = ngapType.PDUSessionResourceSetupResponseIEsPresentRANUENGAPID
	ranIE.Value.RANUENGAPID = &ngapType.RANUENGAPID{Value: 42}
	ies.List = append(ies.List, ranIE)

	assertNoPanic(t, "HandlePDUSessionResourceSetupResponse(only unknown RAN ID)", func() {
		ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)
	})
}
