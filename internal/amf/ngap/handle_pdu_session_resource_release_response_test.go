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

func TestHandlePDUSessionResourceReleaseResponse_MissingIDs(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)
	amfInstance := newTestAMF()

	msg := decode.PDUSessionResourceReleaseResponse{}

	ngap.HandlePDUSessionResourceReleaseResponse(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandlePDUSessionResourceReleaseResponse_UEFoundWithReleasedSessions(t *testing.T) {
	ran := newTestRadio()
	fakeSmf := &FakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)

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

	amfUeNgapID := int64(10)
	ranUeNgapID := int64(1)
	msg := decode.PDUSessionResourceReleaseResponse{
		AMFUENGAPID: &amfUeNgapID,
		RANUENGAPID: &ranUeNgapID,
		PDUSessionResourceReleasedItems: []ngapType.PDUSessionResourceReleasedItemRelRes{
			{
				PDUSessionID: ngapType.PDUSessionID{Value: 1},
				PDUSessionResourceReleaseResponseTransfer: []byte{0x01},
			},
		},
	}

	ngap.HandlePDUSessionResourceReleaseResponse(context.Background(), amfInstance, ran, msg)

	if len(fakeSmf.PduResRelRspCalls) != 1 {
		t.Fatalf("expected 1 PduResRelRsp call, got %d", len(fakeSmf.PduResRelRspCalls))
	}

	if fakeSmf.PduResRelRspCalls[0] != "ref-session-1" {
		t.Errorf("SmContextRef = %q, want %q", fakeSmf.PduResRelRspCalls[0], "ref-session-1")
	}

	smCtx, ok := amfUe.SmContextFindByPDUSessionID(1)
	if !ok {
		t.Fatal("expected SmContext to still exist")
	}

	if !smCtx.PduSessionInactive {
		t.Error("expected PduSessionInactive to be true")
	}
}
