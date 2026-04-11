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

func TestPDUSessionResourceModifyResponse_AmfUeNgapIDLookup(t *testing.T) {
	ran := newTestRadio()

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[1] = ranUe

	amfInstance := newTestAMFWithSmf(&FakeSmfSbi{})
	amfInstance.Radios[new(sctp.SCTPConn)] = ran

	targetRan := newTestRadio()
	amfID := int64(10)

	ngap.HandlePDUSessionResourceModifyResponse(context.Background(), amfInstance, targetRan, decode.PDUSessionResourceModifyResponse{
		AMFUENGAPID: &amfID,
	})

	if ranUe.Radio != targetRan {
		t.Error("expected ranUe.Radio to be updated to targetRan")
	}
}
