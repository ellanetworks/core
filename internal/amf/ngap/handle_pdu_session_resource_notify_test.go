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

func TestPDUSessionResourceNotify_UnknownAmfUeNgapID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, &ngap.PDUSessionResourceNotify{
		RANUENGAPID: ngap.RANUENGAPID(99),
		AMFUENGAPID: ngap.AMFUENGAPID(999),
	})

	errInd := assertSingleErrorIndication(t, sender, ngap.CauseRadioNetworkUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 999, 99)
}

func TestPDUSessionResourceNotify_NilUeContext(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, &ngap.PDUSessionResourceNotify{
		RANUENGAPID: ngap.RANUENGAPID(1),
		AMFUENGAPID: ngap.AMFUENGAPID(10),
	})
}

func TestPDUSessionResourceNotify_ReleasedSessionDeactivated(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, &ngap.PDUSessionResourceNotify{
		RANUENGAPID:                ngap.RANUENGAPID(1),
		AMFUENGAPID:                ngap.AMFUENGAPID(10),
		PDUSessionResourceReleased: ngap.PDUSessionResourceReleasedListNot{{PDUSessionID: 1, Transfer: []byte{0x01}}},
	})

	if len(fakeSmf.DeactivateSmContextCalls) != 1 {
		t.Fatalf("expected 1 DeactivateSmContext call, got %d", len(fakeSmf.DeactivateSmContextCalls))
	}

	if fakeSmf.DeactivateSmContextCalls[0] != "ref-session-1" {
		t.Errorf("DeactivateSmContext ref = %q, want %q", fakeSmf.DeactivateSmContextCalls[0], "ref-session-1")
	}

	sc := amfUe.SmContextList[1]
	if sc == nil {
		t.Fatal("SmContext was removed instead of marked inactive")
	}

	if !sc.PduSessionInactive {
		t.Error("expected SmContext to be marked inactive")
	}
}

func TestPDUSessionResourceNotify_ReleasedSessionSmContextNotFound(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, &ngap.PDUSessionResourceNotify{
		RANUENGAPID:                ngap.RANUENGAPID(1),
		AMFUENGAPID:                ngap.AMFUENGAPID(10),
		PDUSessionResourceReleased: ngap.PDUSessionResourceReleasedListNot{{PDUSessionID: 5, Transfer: []byte{0x01}}},
	})

	if len(fakeSmf.DeactivateSmContextCalls) != 0 {
		t.Fatalf("expected no DeactivateSmContext calls, got %d", len(fakeSmf.DeactivateSmContextCalls))
	}
}

// PDU Session ID 0 is a legal INTEGER (0..255) value that no session in this
// UE's context uses, so the released item names nothing to deactivate. The
// handler skips it and carries on with the rest of the list.
func TestPDUSessionResourceNotify_ReleasedSessionIDNotInContext(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, &ngap.PDUSessionResourceNotify{
		RANUENGAPID:                ngap.RANUENGAPID(1),
		AMFUENGAPID:                ngap.AMFUENGAPID(10),
		PDUSessionResourceReleased: ngap.PDUSessionResourceReleasedListNot{{PDUSessionID: 0, Transfer: []byte{0x01}}},
	})

	if len(fakeSmf.DeactivateSmContextCalls) != 0 {
		t.Fatalf("expected no DeactivateSmContext calls, got %d", len(fakeSmf.DeactivateSmContextCalls))
	}
}

func TestPDUSessionResourceNotify_NotifyListLogsWarning(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, &ngap.PDUSessionResourceNotify{
		RANUENGAPID:              1,
		AMFUENGAPID:              10,
		PDUSessionResourceNotify: ngap.PDUSessionResourceNotifyList{{PDUSessionID: 1, Transfer: ngap.TransferContainer{0x01}}},
	})

	if len(fakeSmf.DeactivateSmContextCalls) != 0 {
		t.Fatalf("expected no DeactivateSmContext calls, got %d", len(fakeSmf.DeactivateSmContextCalls))
	}
}
