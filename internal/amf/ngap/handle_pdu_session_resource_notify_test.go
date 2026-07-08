// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
)

func TestPDUSessionResourceNotify_UnknownAmfUeNgapID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	ngap.HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, decode.PDUSessionResourceNotify{
		RANUENGAPID: 99,
		AMFUENGAPID: 999,
	})

	errInd := assertSingleErrorIndication(t, sender, ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 999, 99)
}

func TestPDUSessionResourceNotify_NilUeContext(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	ngap.HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, decode.PDUSessionResourceNotify{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
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

	ngap.HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, decode.PDUSessionResourceNotify{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
		PDUSessionResourceReleasedItems: []ngapType.PDUSessionResourceReleasedItemNot{
			{
				PDUSessionID:                             ngapType.PDUSessionID{Value: 1},
				PDUSessionResourceNotifyReleasedTransfer: []byte{0x01},
			},
		},
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

	ngap.HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, decode.PDUSessionResourceNotify{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
		PDUSessionResourceReleasedItems: []ngapType.PDUSessionResourceReleasedItemNot{
			{
				PDUSessionID:                             ngapType.PDUSessionID{Value: 5},
				PDUSessionResourceNotifyReleasedTransfer: []byte{0x01},
			},
		},
	})

	if len(fakeSmf.DeactivateSmContextCalls) != 0 {
		t.Fatalf("expected no DeactivateSmContext calls, got %d", len(fakeSmf.DeactivateSmContextCalls))
	}
}

func TestPDUSessionResourceNotify_InvalidPDUSessionID(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	ngap.HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, decode.PDUSessionResourceNotify{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
		PDUSessionResourceReleasedItems: []ngapType.PDUSessionResourceReleasedItemNot{
			{
				PDUSessionID:                             ngapType.PDUSessionID{Value: 0},
				PDUSessionResourceNotifyReleasedTransfer: []byte{0x01},
			},
		},
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

	ngap.HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, decode.PDUSessionResourceNotify{
		RANUENGAPID:   1,
		AMFUENGAPID:   10,
		HasNotifyList: true,
	})

	if len(fakeSmf.DeactivateSmContextCalls) != 0 {
		t.Fatalf("expected no DeactivateSmContext calls, got %d", len(fakeSmf.DeactivateSmContextCalls))
	}
}
