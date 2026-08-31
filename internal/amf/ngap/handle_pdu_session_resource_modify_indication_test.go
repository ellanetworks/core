// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

func TestPDUSessionResourceModifyIndication_UnknownRanUeNgapID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	HandlePDUSessionResourceModifyIndication(context.Background(), amfInstance, ran, &ngap.PDUSessionResourceModifyIndication{
		RANUENGAPID: ngap.RANUENGAPID(99),
	})

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}

	ei := sender.SentErrorIndications[0]

	wantRadioNetworkCause := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnknownLocalUENGAPID}
	if ei.Cause == nil || *ei.Cause != wantRadioNetworkCause {
		t.Errorf("cause = %v, want unknown-local-UE-NGAP-ID", ei.Cause)
	}
}

func TestPDUSessionResourceModifyIndication_UnknownAmfUeNgapID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	HandlePDUSessionResourceModifyIndication(context.Background(), amfInstance, ran, &ngap.PDUSessionResourceModifyIndication{
		RANUENGAPID: ngap.RANUENGAPID(1),
		AMFUENGAPID: ngap.AMFUENGAPID(99999),
	})

	errInd := assertSingleErrorIndication(t, sender, ngap.CauseRadioNetworkUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 99999, 1)

	if len(sender.SentPDUSessionModifyConfirms) != 0 {
		t.Errorf("expected no Modify Confirm on ID mismatch, got %d", len(sender.SentPDUSessionModifyConfirms))
	}
}

// TS 38.413 §8.2.5.2
func TestPDUSessionResourceModifyIndication_SendsModifyConfirm(t *testing.T) {
	fakeSmf := &fakeSmfSbi{ModifyIndicationResponse: []byte{0x01}}
	amfInstance := newTestAMFWithSmf(fakeSmf)

	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amfUe := newValidUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "imsi-001010000000001-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	msg := &ngap.PDUSessionResourceModifyIndication{
		RANUENGAPID:              ngap.RANUENGAPID(1),
		AMFUENGAPID:              ngap.AMFUENGAPID(10),
		PDUSessionResourceModify: ngap.PDUSessionResourceModifyListModInd{{PDUSessionID: 1, Transfer: []byte{0xaa, 0xbb}}},
	}

	HandlePDUSessionResourceModifyIndication(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}

	if len(fakeSmf.ModifyIndicationCalls) != 1 {
		t.Fatalf("expected 1 SMF modify-indication call, got %d", len(fakeSmf.ModifyIndicationCalls))
	}

	if fakeSmf.ModifyIndicationCalls[0].SmContextRef != "imsi-001010000000001-1" {
		t.Errorf("SmContextRef = %s, want imsi-001010000000001-1", fakeSmf.ModifyIndicationCalls[0].SmContextRef)
	}

	if len(sender.SentPDUSessionModifyConfirms) != 1 {
		t.Fatalf("expected 1 Modify Confirm, got %d", len(sender.SentPDUSessionModifyConfirms))
	}

	confirmed := sender.SentPDUSessionModifyConfirms[0].PDUSessionResourceModify
	if len(confirmed) != 1 || confirmed[0].PDUSessionID != 1 {
		t.Fatalf("expected confirm list naming session 1, got %v", confirmed)
	}
}

// TS 38.413 §8.2.5.2
func TestPDUSessionResourceModifyIndication_SmContextNotFound(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmf(fakeSmf)

	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amfUe := newValidUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	msg := &ngap.PDUSessionResourceModifyIndication{
		RANUENGAPID:              ngap.RANUENGAPID(1),
		AMFUENGAPID:              ngap.AMFUENGAPID(10),
		PDUSessionResourceModify: ngap.PDUSessionResourceModifyListModInd{{PDUSessionID: 1, Transfer: []byte{0xaa}}},
	}

	HandlePDUSessionResourceModifyIndication(context.Background(), amfInstance, ran, msg)

	if len(fakeSmf.ModifyIndicationCalls) != 0 {
		t.Fatalf("expected no SMF call for an unknown session, got %d", len(fakeSmf.ModifyIndicationCalls))
	}

	if len(sender.SentPDUSessionModifyConfirms) != 1 {
		t.Fatalf("expected 1 Modify Confirm carrying the failed session, got %d", len(sender.SentPDUSessionModifyConfirms))
	}
}
