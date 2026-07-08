// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleUplinkNasTransport_UnknownUeConn_SendsErrorIndication(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	ngap.HandleUplinkNasTransport(context.Background(), amfInstance, ran, decode.UplinkNASTransport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
		NASPDU:      []byte{0x7E, 0x00, 0x55},
	})

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("ErrorIndications sent = %d, want 1", len(sender.SentErrorIndications))
	}

	cause := sender.SentErrorIndications[0].Cause
	if cause == nil || cause.Present != ngapType.CausePresentRadioNetwork {
		t.Fatal("expected RadioNetwork cause")
	}

	if cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID {
		t.Errorf("cause = %d, want UnknownLocalUENGAPID (%d)",
			cause.RadioNetwork.Value, ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID)
	}
}

// TestHandleUplinkNasTransport_UnknownAmfUeNgapID_SendsErrorIndication covers
// TS 38.413: an AMF UE NGAP ID the AMF never allocated is an unknown local
// AP ID, so the AMF answers with an Error Indication carrying the received AP IDs
// and cause "Unknown local UE NGAP ID".
func TestHandleUplinkNasTransport_UnknownAmfUeNgapID_SendsErrorIndication(t *testing.T) {
	fakeNAS := &fakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	ngap.HandleUplinkNasTransport(context.Background(), amfInstance, ran, decode.UplinkNASTransport{
		AMFUENGAPID: 99999,
		RANUENGAPID: 1,
		NASPDU:      []byte{0x7E, 0x00, 0x55},
	})

	errInd := assertSingleErrorIndication(t, sender, ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 99999, 1)

	if len(fakeNAS.Calls) != 0 {
		t.Errorf("NAS handler must not be invoked on ID mismatch, got %d calls", len(fakeNAS.Calls))
	}
}

// TestHandleUplinkNasTransport_InconsistentRanUeNgapID_SendsErrorIndication
// covers TS 38.413: a RAN UE NGAP ID different from the one stored for the
// connection is an inconsistent remote AP ID, so the AMF answers with an Error
// Indication carrying the received AP IDs and cause "Inconsistent remote UE NGAP
// ID".
func TestHandleUplinkNasTransport_InconsistentRanUeNgapID_SendsErrorIndication(t *testing.T) {
	fakeNAS := &fakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	ngap.HandleUplinkNasTransport(context.Background(), amfInstance, ran, decode.UplinkNASTransport{
		AMFUENGAPID: 10,
		RANUENGAPID: 2,
		NASPDU:      []byte{0x7E, 0x00, 0x55},
	})

	errInd := assertSingleErrorIndication(t, sender, ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 10, 2)

	if len(fakeNAS.Calls) != 0 {
		t.Errorf("NAS handler must not be invoked on ID mismatch, got %d calls", len(fakeNAS.Calls))
	}
}

func TestHandleUplinkNasTransport_NilUeContext_RemovesUeConn(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	ngap.HandleUplinkNasTransport(context.Background(), amfInstance, ran, decode.UplinkNASTransport{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		NASPDU:      []byte{0x7E, 0x00, 0x55},
	})

	if amfInstance.FindUEByRanUeNgapID(ran, 1) != nil {
		t.Error("expected UeConn to be removed from radio's map")
	}
}

func TestHandleUplinkNasTransport_HappyPath_NASDispatched(t *testing.T) {
	fakeNAS := &fakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	nasPDU := []byte{0xAA, 0xBB}

	ngap.HandleUplinkNasTransport(context.Background(), amfInstance, ran, decode.UplinkNASTransport{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		NASPDU:      nasPDU,
	})

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}

	if !bytes.Equal(fakeNAS.Calls[0].NASPDU, nasPDU) {
		t.Errorf("NAS PDU = %x, want %x", fakeNAS.Calls[0].NASPDU, nasPDU)
	}

	if ueConn.Radio() != ran {
		t.Error("ueConn.Radio not set to ran")
	}
}

func TestHandleUplinkNasTransport_LocationUpdatedBeforeNAS(t *testing.T) {
	fakeNAS := &fakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	ngap.HandleUplinkNasTransport(context.Background(), amfInstance, ran, decode.UplinkNASTransport{
		AMFUENGAPID:             10,
		RANUENGAPID:             1,
		NASPDU:                  []byte{0xCC},
		UserLocationInformation: decode.UserLocationInformation{},
	})

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}
}
