// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
)

func TestInitialContextSetupResponse_UnknownRanUeNgapID(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMFWithSmf(&FakeSmfSbi{})

	ngap.HandleInitialContextSetupResponse(context.Background(), amfInstance, ran, decode.InitialContextSetupResponse{
		RANUENGAPID: 99,
		AMFUENGAPID: 1,
	})
}

func TestInitialContextSetupResponse_NilAmfUe(t *testing.T) {
	ran := newTestRadio()
	amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)

	amfInstance := newTestAMFWithSmf(&FakeSmfSbi{})

	ngap.HandleInitialContextSetupResponse(context.Background(), amfInstance, ran, decode.InitialContextSetupResponse{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
	})
}

func newTestAMFWithSmfAndDB(smf amf.SmfSbi) *amf.AMF {
	return amf.New(&FakeDBInstance{
		Operator: &db.Operator{
			Mcc: "001",
			Mnc: "01",
		},
	}, nil, smf)
}

func TestInitialContextSetupResponse_SetupItemsForwardedToSmf(t *testing.T) {
	ran := newTestRadio()
	fakeSmf := &FakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	transfer := []byte{0xAA, 0xBB}

	ngap.HandleInitialContextSetupResponse(context.Background(), amfInstance, ran, decode.InitialContextSetupResponse{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
		SetupItems: []ngapType.PDUSessionResourceSetupItemCxtRes{
			{
				PDUSessionID:                            ngapType.PDUSessionID{Value: 1},
				PDUSessionResourceSetupResponseTransfer: transfer,
			},
		},
	})

	if len(fakeSmf.PduResSetupRspCalls) != 1 {
		t.Fatalf("expected 1 PduResSetupRsp call, got %d", len(fakeSmf.PduResSetupRspCalls))
	}

	if fakeSmf.PduResSetupRspCalls[0].SmContextRef != "ref-session-1" {
		t.Errorf("SmContextRef = %q, want %q", fakeSmf.PduResSetupRspCalls[0].SmContextRef, "ref-session-1")
	}

	if !ranUe.RecvdInitialContextSetupResponse {
		t.Error("expected RecvdInitialContextSetupResponse to be true")
	}
}

func TestInitialContextSetupResponse_FailedItemsForwardedToSmf(t *testing.T) {
	ran := newTestRadio()
	fakeSmf := &FakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	transfer := []byte{0xCC, 0xDD}

	ngap.HandleInitialContextSetupResponse(context.Background(), amfInstance, ran, decode.InitialContextSetupResponse{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
		FailedToSetupItems: []ngapType.PDUSessionResourceFailedToSetupItemCxtRes{
			{
				PDUSessionID: ngapType.PDUSessionID{Value: 1},
				PDUSessionResourceSetupUnsuccessfulTransfer: transfer,
			},
		},
	})

	if len(fakeSmf.PduResSetupFailCalls) != 1 {
		t.Fatalf("expected 1 PduResSetupFail call, got %d", len(fakeSmf.PduResSetupFailCalls))
	}

	if fakeSmf.PduResSetupFailCalls[0].SmContextRef != "ref-session-1" {
		t.Errorf("SmContextRef = %q, want %q", fakeSmf.PduResSetupFailCalls[0].SmContextRef, "ref-session-1")
	}
}

func TestInitialContextSetupResponse_SetupItemSmContextNotFound(t *testing.T) {
	ran := newTestRadio()
	fakeSmf := &FakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	ngap.HandleInitialContextSetupResponse(context.Background(), amfInstance, ran, decode.InitialContextSetupResponse{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
		SetupItems: []ngapType.PDUSessionResourceSetupItemCxtRes{
			{
				PDUSessionID:                            ngapType.PDUSessionID{Value: 5},
				PDUSessionResourceSetupResponseTransfer: []byte{0x01},
			},
		},
	})

	if len(fakeSmf.PduResSetupRspCalls) != 0 {
		t.Fatalf("expected no PduResSetupRsp calls, got %d", len(fakeSmf.PduResSetupRspCalls))
	}
}

func TestInitialContextSetupResponse_InvalidPDUSessionID(t *testing.T) {
	ran := newTestRadio()
	fakeSmf := &FakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	ngap.HandleInitialContextSetupResponse(context.Background(), amfInstance, ran, decode.InitialContextSetupResponse{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
		SetupItems: []ngapType.PDUSessionResourceSetupItemCxtRes{
			{
				PDUSessionID:                            ngapType.PDUSessionID{Value: 0},
				PDUSessionResourceSetupResponseTransfer: []byte{0x01},
			},
		},
	})

	if len(fakeSmf.PduResSetupRspCalls) != 0 {
		t.Fatalf("expected no PduResSetupRsp calls, got %d", len(fakeSmf.PduResSetupRspCalls))
	}
}

func TestInitialContextSetupResponse_MixedSetupAndFailedItems(t *testing.T) {
	ran := newTestRadio()
	fakeSmf := &FakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}
	amfUe.SmContextList[2] = &amf.SmContext{
		Ref:    "ref-session-2",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	ngap.HandleInitialContextSetupResponse(context.Background(), amfInstance, ran, decode.InitialContextSetupResponse{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
		SetupItems: []ngapType.PDUSessionResourceSetupItemCxtRes{
			{
				PDUSessionID:                            ngapType.PDUSessionID{Value: 1},
				PDUSessionResourceSetupResponseTransfer: []byte{0xAA},
			},
		},
		FailedToSetupItems: []ngapType.PDUSessionResourceFailedToSetupItemCxtRes{
			{
				PDUSessionID: ngapType.PDUSessionID{Value: 2},
				PDUSessionResourceSetupUnsuccessfulTransfer: []byte{0xBB},
			},
		},
	})

	if len(fakeSmf.PduResSetupRspCalls) != 1 {
		t.Fatalf("expected 1 PduResSetupRsp call, got %d", len(fakeSmf.PduResSetupRspCalls))
	}

	if fakeSmf.PduResSetupRspCalls[0].SmContextRef != "ref-session-1" {
		t.Errorf("setup rsp SmContextRef = %q, want %q", fakeSmf.PduResSetupRspCalls[0].SmContextRef, "ref-session-1")
	}

	if len(fakeSmf.PduResSetupFailCalls) != 1 {
		t.Fatalf("expected 1 PduResSetupFail call, got %d", len(fakeSmf.PduResSetupFailCalls))
	}

	if fakeSmf.PduResSetupFailCalls[0].SmContextRef != "ref-session-2" {
		t.Errorf("setup fail SmContextRef = %q, want %q", fakeSmf.PduResSetupFailCalls[0].SmContextRef, "ref-session-2")
	}

	if !ranUe.RecvdInitialContextSetupResponse {
		t.Error("expected RecvdInitialContextSetupResponse to be true")
	}
}
