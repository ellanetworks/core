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
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleUEContextModificationFailure_UnknownRanUeNgapID(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	sender := ran.Conn.(*fakeNGAPSender)
	amfInstance := newTestAMF()
	ranUeNgapID := int64(1)
	msg := decode.UEContextModificationFailure{
		RANUENGAPID: &ranUeNgapID,
	}

	ngap.HandleUEContextModificationFailure(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication (TS 38.413), got %d", len(sender.SentErrorIndications))
	}
}

func TestHandleUEContextModificationFailure_UEFound(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), ran)

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	amfUeNgapID := int64(10)
	msg := decode.UEContextModificationFailure{
		AMFUENGAPID: &amfUeNgapID,
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
		},
	}

	ngap.HandleUEContextModificationFailure(context.Background(), amfInstance, ran, msg)

	// ueConn was already created on 'ran', so Radio() should still be 'ran'.
	if ueConn.Radio() != ran {
		t.Error("expected ueConn.Radio to still be ran")
	}
}
