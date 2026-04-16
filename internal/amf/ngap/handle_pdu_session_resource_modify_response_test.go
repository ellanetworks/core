// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
)

func TestPDUSessionResourceModifyResponse_BothIDsNil(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	ngap.HandlePDUSessionResourceModifyResponse(context.Background(), amfInstance, ran, decode.PDUSessionResourceModifyResponse{})
}

func TestPDUSessionResourceModifyResponse_RanUeNgapIDNotFound(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	ranID := int64(99)

	ngap.HandlePDUSessionResourceModifyResponse(context.Background(), amfInstance, ran, decode.PDUSessionResourceModifyResponse{
		RANUENGAPID: &ranID,
	})
}

func TestPDUSessionResourceModifyResponse_CrossRadioRejected(t *testing.T) {
	ran := newTestRadio()
	amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)

	amfInstance := newTestAMFWithSmf(&FakeSmfSbi{})
	amfInstance.Radios[new(sctp.SCTPConn)] = ran

	// A different radio claims the same AMF-UE-NGAP-ID — must be rejected.
	attackerRan := newTestRadio()
	attackerSender := attackerRan.NGAPSender.(*FakeNGAPSender)
	amfID := int64(10)

	ngap.HandlePDUSessionResourceModifyResponse(context.Background(), amfInstance, attackerRan, decode.PDUSessionResourceModifyResponse{
		AMFUENGAPID: &amfID,
	})

	if len(attackerSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication for cross-radio AMF-UE-NGAP-ID, got %d", len(attackerSender.SentErrorIndications))
	}
}
