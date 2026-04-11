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
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleUEContextModificationFailure_MissingAMFUENGAPID(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)
	amfInstance := newTestAMF()
	ranUeNgapID := int64(1)
	msg := decode.UEContextModificationFailure{
		RANUENGAPID: &ranUeNgapID,
	}

	ngap.HandleUEContextModificationFailure(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandleUEContextModificationFailure_UEFound(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	amfInstance.Radios[new(sctp.SCTPConn)] = ran

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[1] = ranUe

	amfUeNgapID := int64(10)
	msg := decode.UEContextModificationFailure{
		AMFUENGAPID: &amfUeNgapID,
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
		},
	}

	ngap.HandleUEContextModificationFailure(context.Background(), amfInstance, ran, msg)

	if ranUe.Radio != ran {
		t.Error("expected ranUe.Radio to be set to ran")
	}
}
