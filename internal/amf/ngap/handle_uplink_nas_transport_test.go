// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleUplinkNasTransport_UnknownRanUe_SendsErrorIndication(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)
	amfInstance := newTestAMF()

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
// TS 38.413 §10.6: an AMF UE NGAP ID the AMF never allocated is an unknown local
// AP ID, so the AMF answers with an Error Indication carrying the received AP IDs
// (§8.7.5.2) and cause "Unknown local UE NGAP ID".
func TestHandleUplinkNasTransport_UnknownAmfUeNgapID_SendsErrorIndication(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)
	fakeNAS := &FakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)

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
// covers TS 38.413 §10.6: a RAN UE NGAP ID different from the one stored for the
// connection is an inconsistent remote AP ID, so the AMF answers with an Error
// Indication carrying the received AP IDs and cause "Inconsistent remote UE NGAP
// ID".
func TestHandleUplinkNasTransport_InconsistentRanUeNgapID_SendsErrorIndication(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)
	fakeNAS := &FakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)

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

func TestHandleUplinkNasTransport_NilUeContext_RemovesRanUe(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)

	ngap.HandleUplinkNasTransport(context.Background(), amfInstance, ran, decode.UplinkNASTransport{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		NASPDU:      []byte{0x7E, 0x00, 0x55},
	})

	if ran.FindUEByRanUeNgapID(1) != nil {
		t.Error("expected RanUe to be removed from radio's map")
	}
}

func TestHandleUplinkNasTransport_HappyPath_NASDispatched(t *testing.T) {
	fakeNAS := &FakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio()

	amfUe := amf.NewUeContext()
	amfUe.Log = logger.AmfLog

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

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

	if ranUe.Radio() != ran {
		t.Error("ranUe.Radio not set to ran")
	}
}

func TestHandleUplinkNasTransport_LocationUpdatedBeforeNAS(t *testing.T) {
	fakeNAS := &FakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio()

	amfUe := amf.NewUeContext()
	amfUe.Log = logger.AmfLog

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

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

// An error from the NAS handler means no NAS response was produced (a delivered
// reject returns nil), so the AMF answers with a 5GMM STATUS #111 (TS 24.501
// §7.x) rather than leaving the UE without a response.
func TestHandleUplinkNasTransport_NASError_SendsStatus5GMM(t *testing.T) {
	fakeNAS := &FakeNASHandler{Err: errors.New("unhandled NAS message")}
	cause := uplinkNASStatusCause(t, fakeNAS)

	if cause != nasMessage.Cause5GMMProtocolErrorUnspecified {
		t.Errorf("STATUS cause = %d, want %d (#111)", cause, nasMessage.Cause5GMMProtocolErrorUnspecified)
	}
}

// uplinkNASStatusCause drives an uplink NAS transport whose handler returns
// fakeNAS.Err, and returns the 5GMM cause of the single 5GMM STATUS the AMF
// sends in response.
func uplinkNASStatusCause(t *testing.T, fakeNAS *FakeNASHandler) uint8 {
	t.Helper()

	amfInstance := newTestAMFWithNAS(fakeNAS)
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	amfUe := amf.NewUeContext()
	amfUe.Log = logger.AmfLog
	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	ngap.HandleUplinkNasTransport(context.Background(), amfInstance, ran, decode.UplinkNASTransport{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		NASPDU:      []byte{0xAA, 0xBB},
	})

	if len(sender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected exactly one 5GMM STATUS, got %d", len(sender.SentDownlinkNASTransport))
	}

	pdu := sender.SentDownlinkNASTransport[0].NasPdu
	m := new(nas.Message)
	m.SecurityHeaderType = nas.GetSecurityHeaderType(pdu) & 0x0f

	if err := m.PlainNasDecode(&pdu); err != nil {
		t.Fatalf("decode 5GMM STATUS: %v", err)
	}

	if m.GmmHeader.GetMessageType() != nas.MsgTypeStatus5GMM {
		t.Fatalf("message type = %d, want 5GMM STATUS", m.GmmHeader.GetMessageType())
	}

	return m.Status5GMM.GetCauseValue()
}
