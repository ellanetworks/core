// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
	libngap "github.com/ellanetworks/core/ngap"
)

// Both UE NGAP IDs are mandatory but ignore criticality, so an absent one
// reaches the handler; without them the AMF cannot address a UE context and
// reports the fault (TS 38.413 §10.3.5).
func TestPDUSessionResourceModifyResponse_BothIDsNil(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	ngap.HandlePDUSessionResourceModifyResponse(context.Background(), amfInstance, ran, &libngap.PDUSessionResourceModifyResponse{})

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestPDUSessionResourceModifyResponse_RanUeNgapIDNotFound(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()

	ngap.HandlePDUSessionResourceModifyResponse(context.Background(), amfInstance, ran, &libngap.PDUSessionResourceModifyResponse{
		RANUENGAPID: libngap.Ptr(libngap.RANUENGAPID(99)),
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
	ngap.HandlePDUSessionResourceModifyResponse(context.Background(), amfInstance, attackerRan, &libngap.PDUSessionResourceModifyResponse{
		AMFUENGAPID: libngap.Ptr(libngap.AMFUENGAPID(10)),
		RANUENGAPID: libngap.Ptr(libngap.RANUENGAPID(1)),
	})

	if len(attackerSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication for cross-radio AMF-UE-NGAP-ID, got %d", len(attackerSender.SentErrorIndications))
	}
}
