// Copyright 2026 Ella Networks

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

func TestPDUSessionResourceNotify_UnknownRanUeNgapID(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	ngap.HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, decode.PDUSessionResourceNotify{
		RANUENGAPID: 99,
	})
}

func TestPDUSessionResourceNotify_NilAmfUe(t *testing.T) {
	ran := newTestRadio()
	amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)

	amfInstance := newTestAMF()

	ngap.HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, decode.PDUSessionResourceNotify{
		RANUENGAPID: 1,
	})
}

func TestPDUSessionResourceNotify_ReleasedSessionDeactivated(t *testing.T) {
	ran := newTestRadio()
	fakeSmf := &FakeSmfSbi{}
	amfInstance := newTestAMFWithSmf(fakeSmf)

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	ngap.HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, decode.PDUSessionResourceNotify{
		RANUENGAPID: 1,
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
	ran := newTestRadio()
	fakeSmf := &FakeSmfSbi{}
	amfInstance := newTestAMFWithSmf(fakeSmf)

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	ngap.HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, decode.PDUSessionResourceNotify{
		RANUENGAPID: 1,
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
	ran := newTestRadio()
	fakeSmf := &FakeSmfSbi{}
	amfInstance := newTestAMFWithSmf(fakeSmf)

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	ngap.HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, decode.PDUSessionResourceNotify{
		RANUENGAPID: 1,
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
	ran := newTestRadio()
	fakeSmf := &FakeSmfSbi{}
	amfInstance := newTestAMFWithSmf(fakeSmf)

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	ngap.HandlePDUSessionResourceNotify(context.Background(), amfInstance, ran, decode.PDUSessionResourceNotify{
		RANUENGAPID:   1,
		HasNotifyList: true,
	})

	if len(fakeSmf.DeactivateSmContextCalls) != 0 {
		t.Fatalf("expected no DeactivateSmContext calls, got %d", len(fakeSmf.DeactivateSmContextCalls))
	}
}
