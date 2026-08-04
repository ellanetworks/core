// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	libngap "github.com/ellanetworks/core/ngap"
)

func TestHandlePDUSessionResourceSetupResponse_EmptyMessage(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	// Both UE NGAP IDs are mandatory but ignore criticality, so an absent one
	// reaches the handler; without them the AMF cannot address a UE context and
	// reports the fault (TS 38.413 §10.3.5).
	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, &libngap.PDUSessionResourceSetupResponse{})

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandlePDUSessionResourceSetupResponse_UnknownAMFUENGAPID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	msg := &libngap.PDUSessionResourceSetupResponse{
		AMFUENGAPID: libngap.Ptr(libngap.AMFUENGAPID(1099511627775)),
		RANUENGAPID: libngap.Ptr(libngap.RANUENGAPID(99)),
	}

	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)

	sender := ran.Conn.(*fakeNGAPSender)
	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication (TS 38.413), got %d", len(sender.SentErrorIndications))
	}
}

func TestHandlePDUSessionResourceSetupResponse_OnlyUnknownRANUENGAPID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	msg := &libngap.PDUSessionResourceSetupResponse{
		RANUENGAPID: libngap.Ptr(libngap.RANUENGAPID(42)),
	}

	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)

	sender := ran.Conn.(*fakeNGAPSender)
	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication (TS 38.413), got %d", len(sender.SentErrorIndications))
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
	msg := &libngap.PDUSessionResourceSetupResponse{
		AMFUENGAPID:             libngap.Ptr(libngap.AMFUENGAPID(10)),
		RANUENGAPID:             libngap.Ptr(libngap.RANUENGAPID(1)),
		PDUSessionResourceSetup: libngap.PDUSessionResourceSetupListSURes{{PDUSessionID: 1, Transfer: libngap.TransferContainer(transfer)}},
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
	msg := &libngap.PDUSessionResourceSetupResponse{
		AMFUENGAPID:              libngap.Ptr(libngap.AMFUENGAPID(10)),
		RANUENGAPID:              libngap.Ptr(libngap.RANUENGAPID(1)),
		PDUSessionResourceFailed: libngap.PDUSessionResourceFailedToSetupListSURes{{PDUSessionID: 1, Transfer: libngap.TransferContainer(transfer)}},
	}

	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)

	if len(fakeSmf.PduResSetupFailCalls) != 1 {
		t.Fatalf("expected 1 PduResSetupFail call, got %d", len(fakeSmf.PduResSetupFailCalls))
	}

	if fakeSmf.PduResSetupFailCalls[0].SmContextRef != "ref-session-1" {
		t.Errorf("SmContextRef = %q, want %q", fakeSmf.PduResSetupFailCalls[0].SmContextRef, "ref-session-1")
	}
}

// The NG-RAN node may report the UE's serving cell with the setup outcome; the
// AMF records it so a later location query is answered from where the UE is
// (TS 38.413 §9.2.1.2).
func TestHandlePDUSessionResourceSetupResponse_RecordsUserLocation(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()
	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	plmn := libngap.PLMNIdentity{0x00, 0xf1, 0x10}
	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, &libngap.PDUSessionResourceSetupResponse{
		AMFUENGAPID: libngap.Ptr(libngap.AMFUENGAPID(10)),
		RANUENGAPID: libngap.Ptr(libngap.RANUENGAPID(1)),
		UserLocationInformation: &libngap.UserLocationInformation{
			Kind: libngap.UserLocationNR, PLMNIdentity: plmn, CellIdentity: 0x123456789,
			TAI: libngap.TAI{PLMNIdentity: plmn, TAC: 7},
		},
	})

	loc := ueConn.Location.NrLocation
	if loc == nil || loc.Ncgi == nil || loc.Ncgi.NrCellID != "123456789" {
		t.Fatalf("serving cell not recorded: %+v", loc)
	}
}
