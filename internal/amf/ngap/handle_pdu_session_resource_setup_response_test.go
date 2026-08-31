// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/ngap"
)

func TestHandlePDUSessionResourceSetupResponse_EmptyMessage(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, &ngap.PDUSessionResourceSetupResponse{})

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandlePDUSessionResourceSetupResponse_UnknownAMFUENGAPID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	msg := &ngap.PDUSessionResourceSetupResponse{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(1099511627775)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(99)),
	}

	HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)

	sender := ran.Conn.(*fakeNGAPSender)
	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication (TS 38.413), got %d", len(sender.SentErrorIndications))
	}
}

func TestHandlePDUSessionResourceSetupResponse_OnlyUnknownRANUENGAPID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	msg := &ngap.PDUSessionResourceSetupResponse{
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(42)),
	}

	HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)

	sender := ran.Conn.(*fakeNGAPSender)
	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandlePDUSessionResourceSetupResponse_HappyPath(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)
	ran := newTestRadio(amfInstance)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), ran)

	amfUe := amf.NewUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	transfer := []byte{0xAA, 0xBB}
	msg := &ngap.PDUSessionResourceSetupResponse{
		AMFUENGAPID:             ngap.Ptr(ngap.AMFUENGAPID(10)),
		RANUENGAPID:             ngap.Ptr(ngap.RANUENGAPID(1)),
		PDUSessionResourceSetup: ngap.PDUSessionResourceSetupListSURes{{PDUSessionID: 1, Transfer: ngap.TransferContainer(transfer)}},
	}

	HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)

	if len(fakeSmf.PduResSetupRspCalls) != 1 {
		t.Fatalf("expected 1 PduResSetupRsp call, got %d", len(fakeSmf.PduResSetupRspCalls))
	}

	if fakeSmf.PduResSetupRspCalls[0].SmContextRef != "ref-session-1" {
		t.Errorf("SmContextRef = %q, want %q", fakeSmf.PduResSetupRspCalls[0].SmContextRef, "ref-session-1")
	}
}

func TestHandlePDUSessionResourceSetupResponse_FailedItemForwardedToSmf(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)
	ran := newTestRadio(amfInstance)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), ran)

	amfUe := amf.NewUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	transfer := []byte{0xCC, 0xDD}
	msg := &ngap.PDUSessionResourceSetupResponse{
		AMFUENGAPID:              ngap.Ptr(ngap.AMFUENGAPID(10)),
		RANUENGAPID:              ngap.Ptr(ngap.RANUENGAPID(1)),
		PDUSessionResourceFailed: ngap.PDUSessionResourceFailedToSetupListSURes{{PDUSessionID: 1, Transfer: ngap.TransferContainer(transfer)}},
	}

	HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)

	if len(fakeSmf.PduResSetupFailCalls) != 1 {
		t.Fatalf("expected 1 PduResSetupFail call, got %d", len(fakeSmf.PduResSetupFailCalls))
	}

	if fakeSmf.PduResSetupFailCalls[0].SmContextRef != "ref-session-1" {
		t.Errorf("SmContextRef = %q, want %q", fakeSmf.PduResSetupFailCalls[0].SmContextRef, "ref-session-1")
	}
}

// TS 38.413 §9.2.1.2
func TestHandlePDUSessionResourceSetupResponse_RecordsUserLocation(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()
	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	plmn := ngap.PLMNIdentity{0x00, 0xf1, 0x10}
	HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, &ngap.PDUSessionResourceSetupResponse{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(10)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(1)),
		UserLocationInformation: &ngap.UserLocationInformation{
			Kind: ngap.UserLocationNR, PLMNIdentity: plmn, CellIdentity: 0x123456789,
			TAI: ngap.TAI{PLMNIdentity: plmn, TAC: 7},
		},
	})

	loc := ueConn.Location.NrLocation
	if loc == nil || loc.Ncgi == nil || loc.Ncgi.NrCellID != "123456789" {
		t.Fatalf("serving cell not recorded: %+v", loc)
	}
}
