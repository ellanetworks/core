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
	"github.com/ellanetworks/core/internal/sctp"
)

func TestPDUSessionResourceModifyResponse_BothIDsNil(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()

	ngap.HandlePDUSessionResourceModifyResponse(context.Background(), amfInstance, ran, decode.PDUSessionResourceModifyResponse{})
}

func TestPDUSessionResourceModifyResponse_RanUeNgapIDNotFound(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()

	ranID := int64(99)

	ngap.HandlePDUSessionResourceModifyResponse(context.Background(), amfInstance, ran, decode.PDUSessionResourceModifyResponse{
		RANUENGAPID: &ranID,
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
	amfID := int64(10)

	ngap.HandlePDUSessionResourceModifyResponse(context.Background(), amfInstance, attackerRan, decode.PDUSessionResourceModifyResponse{
		AMFUENGAPID: &amfID,
	})

	if len(attackerSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication for cross-radio AMF-UE-NGAP-ID, got %d", len(attackerSender.SentErrorIndications))
	}
}
