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

func TestHandlePDUSessionResourceReleaseResponse_MissingIDs(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	// Both UE NGAP IDs are mandatory but ignore criticality, so an absent one
	// reaches the handler; without them the AMF cannot address a UE context and
	// reports the fault (TS 38.413 §10.3.5).
	HandlePDUSessionResourceReleaseResponse(context.Background(), amfInstance, ran, &ngap.PDUSessionResourceReleaseResponse{})

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandlePDUSessionResourceReleaseResponse_UEFoundWithReleasedSessions(t *testing.T) {
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

	msg := &ngap.PDUSessionResourceReleaseResponse{
		AMFUENGAPID:                ngap.Ptr(ngap.AMFUENGAPID(10)),
		RANUENGAPID:                ngap.Ptr(ngap.RANUENGAPID(1)),
		PDUSessionResourceReleased: ngap.PDUSessionResourceReleasedListRelRes{{PDUSessionID: 1, Transfer: []byte{0x01}}},
	}

	HandlePDUSessionResourceReleaseResponse(context.Background(), amfInstance, ran, msg)

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
