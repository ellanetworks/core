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

func TestHandleUEContextModificationResponse_UnknownRanUeNgapID(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	sender := ran.NGAPSender.(*fakeNGAPSender)
	amfInstance := newTestAMF()

	ranID := int64(1)
	msg := decode.UEContextModificationResponse{
		RANUENGAPID: &ranID,
	}

	ngap.HandleUEContextModificationResponse(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication (TS 38.413), got %d", len(sender.SentErrorIndications))
	}
}

func TestHandleUEContextModificationResponse_UEFound(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()
	amfInstance.Radios[new(sctp.SCTPConn)] = ran

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)

	amfUeNgapID := int64(10)
	msg := decode.UEContextModificationResponse{
		AMFUENGAPID: &amfUeNgapID,
	}

	ngap.HandleUEContextModificationResponse(context.Background(), amfInstance, ran, msg)

	// ranUe was already created on 'ran', so Radio() should still be 'ran'.
	if ranUe.Radio() != ran {
		t.Error("expected ranUe.Radio to still be ran")
	}
}
