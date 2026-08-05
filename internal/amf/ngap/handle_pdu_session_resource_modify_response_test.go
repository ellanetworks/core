// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/ngap"
)

// Both UE NGAP IDs are mandatory but ignore criticality, so an absent one
// reaches the handler; without them the AMF cannot address a UE context and
// reports the fault (TS 38.413 §10.3.5).
func TestPDUSessionResourceModifyResponse_BothIDsNil(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	HandlePDUSessionResourceModifyResponse(context.Background(), amfInstance, ran, &ngap.PDUSessionResourceModifyResponse{})

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestPDUSessionResourceModifyResponse_RanUeNgapIDNotFound(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()

	HandlePDUSessionResourceModifyResponse(context.Background(), amfInstance, ran, &ngap.PDUSessionResourceModifyResponse{
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(99)),
	})
}

func TestPDUSessionResourceModifyResponse_CrossRadioRejected(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	amfInstance := newTestAMFWithSmf(&fakeSmfSbi{})
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), ran)

	// A different radio claims the same AMF-UE-NGAP-ID — must be rejected.
	attackerRan := newTestRadio(newTestAMF())
	attackerSender := attackerRan.Conn.(*fakeNGAPSender)
	HandlePDUSessionResourceModifyResponse(context.Background(), amfInstance, attackerRan, &ngap.PDUSessionResourceModifyResponse{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(10)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(1)),
	})

	if len(attackerSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication for cross-radio AMF-UE-NGAP-ID, got %d", len(attackerSender.SentErrorIndications))
	}
}
