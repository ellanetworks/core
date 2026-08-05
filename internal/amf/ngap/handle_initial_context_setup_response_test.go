// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

func TestInitialContextSetupResponse_UnknownAmfUeNgapID(t *testing.T) {
	amfInstance := newTestAMFWithSmf(&fakeSmfSbi{})
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	HandleInitialContextSetupResponse(context.Background(), amfInstance, ran, &ngap.InitialContextSetupResponse{
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(99)),
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(999)),
	})

	errInd := assertSingleErrorIndication(t, sender, ngap.CauseRadioNetworkUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 999, 99)
}

func TestInitialContextSetupResponse_NilUeContext(t *testing.T) {
	amfInstance := newTestAMFWithSmf(&fakeSmfSbi{})
	ran := newTestRadio(amfInstance)
	amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	HandleInitialContextSetupResponse(context.Background(), amfInstance, ran, &ngap.InitialContextSetupResponse{
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(1)),
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(10)),
	})
}

func newTestAMFWithSmfAndDB(smf amf.SmfSbi) *amf.AMF {
	return amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc: "001",
			Mnc: "01",
		},
	}, nil, smf)
}

func TestInitialContextSetupResponse_SetupItemsForwardedToSmf(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	transfer := []byte{0xAA, 0xBB}

	HandleInitialContextSetupResponse(context.Background(), amfInstance, ran, &ngap.InitialContextSetupResponse{
		RANUENGAPID:             ngap.Ptr(ngap.RANUENGAPID(1)),
		AMFUENGAPID:             ngap.Ptr(ngap.AMFUENGAPID(10)),
		PDUSessionResourceSetup: ngap.PDUSessionResourceSetupListCxtRes{{PDUSessionID: 1, Transfer: transfer}},
	})

	if len(fakeSmf.PduResSetupRspCalls) != 1 {
		t.Fatalf("expected 1 PduResSetupRsp call, got %d", len(fakeSmf.PduResSetupRspCalls))
	}

	if fakeSmf.PduResSetupRspCalls[0].SmContextRef != "ref-session-1" {
		t.Errorf("SmContextRef = %q, want %q", fakeSmf.PduResSetupRspCalls[0].SmContextRef, "ref-session-1")
	}

	if ueConn.ICS() != amf.ICSCompleted {
		t.Error("expected ueConn.ICS == ICSCompleted")
	}
}

func TestInitialContextSetupResponse_FailedItemsForwardedToSmf(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	transfer := []byte{0xCC, 0xDD}

	HandleInitialContextSetupResponse(context.Background(), amfInstance, ran, &ngap.InitialContextSetupResponse{
		RANUENGAPID:              ngap.Ptr(ngap.RANUENGAPID(1)),
		AMFUENGAPID:              ngap.Ptr(ngap.AMFUENGAPID(10)),
		PDUSessionResourceFailed: ngap.PDUSessionResourceFailedToSetupListCxtRes{{PDUSessionID: 1, Transfer: transfer}},
	})

	if len(fakeSmf.PduResSetupFailCalls) != 1 {
		t.Fatalf("expected 1 PduResSetupFail call, got %d", len(fakeSmf.PduResSetupFailCalls))
	}

	if fakeSmf.PduResSetupFailCalls[0].SmContextRef != "ref-session-1" {
		t.Errorf("SmContextRef = %q, want %q", fakeSmf.PduResSetupFailCalls[0].SmContextRef, "ref-session-1")
	}
}

func TestInitialContextSetupResponse_SetupItemSmContextNotFound(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	HandleInitialContextSetupResponse(context.Background(), amfInstance, ran, &ngap.InitialContextSetupResponse{
		RANUENGAPID:             ngap.Ptr(ngap.RANUENGAPID(1)),
		AMFUENGAPID:             ngap.Ptr(ngap.AMFUENGAPID(10)),
		PDUSessionResourceSetup: ngap.PDUSessionResourceSetupListCxtRes{{PDUSessionID: 5, Transfer: []byte{0x01}}},
	})

	if len(fakeSmf.PduResSetupRspCalls) != 0 {
		t.Fatalf("expected no PduResSetupRsp calls, got %d", len(fakeSmf.PduResSetupRspCalls))
	}
}

func TestInitialContextSetupResponse_InvalidPDUSessionID(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	HandleInitialContextSetupResponse(context.Background(), amfInstance, ran, &ngap.InitialContextSetupResponse{
		RANUENGAPID:             ngap.Ptr(ngap.RANUENGAPID(1)),
		AMFUENGAPID:             ngap.Ptr(ngap.AMFUENGAPID(10)),
		PDUSessionResourceSetup: ngap.PDUSessionResourceSetupListCxtRes{{PDUSessionID: 0, Transfer: []byte{0x01}}},
	})

	if len(fakeSmf.PduResSetupRspCalls) != 0 {
		t.Fatalf("expected no PduResSetupRsp calls, got %d", len(fakeSmf.PduResSetupRspCalls))
	}
}

func TestInitialContextSetupResponse_MixedSetupAndFailedItems(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}
	amfUe.SmContextList[2] = &amf.SmContext{
		Ref:    "ref-session-2",
		Snssai: &models.Snssai{Sst: 1},
	}

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	HandleInitialContextSetupResponse(context.Background(), amfInstance, ran, &ngap.InitialContextSetupResponse{
		RANUENGAPID:              ngap.Ptr(ngap.RANUENGAPID(1)),
		AMFUENGAPID:              ngap.Ptr(ngap.AMFUENGAPID(10)),
		PDUSessionResourceSetup:  ngap.PDUSessionResourceSetupListCxtRes{{PDUSessionID: 1, Transfer: []byte{0xAA}}},
		PDUSessionResourceFailed: ngap.PDUSessionResourceFailedToSetupListCxtRes{{PDUSessionID: 2, Transfer: []byte{0xBB}}},
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

	if ueConn.ICS() != amf.ICSCompleted {
		t.Error("expected ueConn.ICS == ICSCompleted")
	}
}
