// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandlePDUSessionResourceSetupResponse_EmptyMessage(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)
	amfInstance := newTestAMF()

	msg := decode.PDUSessionResourceSetupResponse{}

	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandlePDUSessionResourceSetupResponse_UnknownAMFUENGAPID(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	amfID := int64(1379640380095)
	ranID := int64(99)
	msg := decode.PDUSessionResourceSetupResponse{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
	}

	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)

	sender := ran.NGAPSender.(*FakeNGAPSender)
	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandlePDUSessionResourceSetupResponse_OnlyUnknownRANUENGAPID(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	ranID := int64(42)
	msg := decode.PDUSessionResourceSetupResponse{
		RANUENGAPID: &ranID,
	}

	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)

	sender := ran.NGAPSender.(*FakeNGAPSender)
	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandlePDUSessionResourceSetupResponse_HappyPath(t *testing.T) {
	ran := newTestRadio()
	fakeSmf := &FakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)
	amfInstance.Radios[new(sctp.SCTPConn)] = ran

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	ran.RanUEs[1] = ranUe

	transfer := []byte{0xAA, 0xBB}
	amfUeNgapID := int64(10)
	ranUeNgapID := int64(1)

	msg := decode.PDUSessionResourceSetupResponse{
		AMFUENGAPID: &amfUeNgapID,
		RANUENGAPID: &ranUeNgapID,
		SetupItems: []ngapType.PDUSessionResourceSetupItemSURes{
			{
				PDUSessionID:                            ngapType.PDUSessionID{Value: 1},
				PDUSessionResourceSetupResponseTransfer: transfer,
			},
		},
	}

	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)

	if len(fakeSmf.PduResSetupRspCalls) != 1 {
		t.Fatalf("expected 1 PduResSetupRsp call, got %d", len(fakeSmf.PduResSetupRspCalls))
	}

	if fakeSmf.PduResSetupRspCalls[0].SmContextRef != "ref-session-1" {
		t.Errorf("SmContextRef = %q, want %q", fakeSmf.PduResSetupRspCalls[0].SmContextRef, "ref-session-1")
	}
}

func TestHandlePDUSessionResourceSetupResponse_FailedItemForwardedToSmf(t *testing.T) {
	ran := newTestRadio()
	fakeSmf := &FakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)
	amfInstance.Radios[new(sctp.SCTPConn)] = ran

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	ran.RanUEs[1] = ranUe

	transfer := []byte{0xCC, 0xDD}
	amfUeNgapID := int64(10)
	ranUeNgapID := int64(1)

	msg := decode.PDUSessionResourceSetupResponse{
		AMFUENGAPID: &amfUeNgapID,
		RANUENGAPID: &ranUeNgapID,
		FailedToSetupItems: []ngapType.PDUSessionResourceFailedToSetupItemSURes{
			{
				PDUSessionID: ngapType.PDUSessionID{Value: 1},
				PDUSessionResourceSetupUnsuccessfulTransfer: transfer,
			},
		},
	}

	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)

	if len(fakeSmf.PduResSetupFailCalls) != 1 {
		t.Fatalf("expected 1 PduResSetupFail call, got %d", len(fakeSmf.PduResSetupFailCalls))
	}

	if fakeSmf.PduResSetupFailCalls[0].SmContextRef != "ref-session-1" {
		t.Errorf("SmContextRef = %q, want %q", fakeSmf.PduResSetupFailCalls[0].SmContextRef, "ref-session-1")
	}
}
