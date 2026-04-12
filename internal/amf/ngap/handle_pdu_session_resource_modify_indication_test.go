// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestPDUSessionResourceModifyIndication_UnknownRanUeNgapID(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	ngap.HandlePDUSessionResourceModifyIndication(context.Background(), ran, decode.PDUSessionResourceModifyIndication{
		RANUENGAPID: 99,
	})

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}

	ei := sender.SentErrorIndications[0]
	if ei.Cause == nil || ei.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Fatal("expected RadioNetwork cause")
	}

	if ei.Cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID {
		t.Fatalf("expected UnknownLocalUENGAPID, got %d", ei.Cause.RadioNetwork.Value)
	}
}

func TestPDUSessionResourceModifyIndication_SendsModifyConfirm(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[1] = ranUe

	ngap.HandlePDUSessionResourceModifyIndication(context.Background(), ran, decode.PDUSessionResourceModifyIndication{
		RANUENGAPID: 1,
	})

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}

	if len(sender.SentPDUSessionModifyConfirms) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceModifyConfirm, got %d", len(sender.SentPDUSessionModifyConfirms))
	}

	confirm := sender.SentPDUSessionModifyConfirms[0]
	if confirm.AmfUeNgapID != 10 {
		t.Errorf("AmfUeNgapID = %d, want 10", confirm.AmfUeNgapID)
	}

	if confirm.RanUeNgapID != 1 {
		t.Errorf("RanUeNgapID = %d, want 1", confirm.RanUeNgapID)
	}
}
