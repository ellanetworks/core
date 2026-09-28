// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

func TestUEContextModificationFailureForgetsTheUEAMBR(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	ueConn := amf.NewUeConnForTest(ran, 1, 1)

	if err := ueConn.SendUEContextModification(context.Background(), models.Ambr{
		Uplink:   models.MustParseBitRate("50 Mbps"),
		Downlink: models.MustParseBitRate("50 Mbps"),
	}); err != nil {
		t.Fatal(err)
	}

	if _, ok := ueConn.HeldUEAMBR(); !ok {
		t.Fatal("the sent UE-AMBR was not recorded")
	}

	HandleUEContextModificationFailure(context.Background(), amfInstance, ran, &ngap.UEContextModificationFailure{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(1)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(1)),
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified},
	})

	if _, ok := ueConn.HeldUEAMBR(); ok {
		t.Fatal("the UE-AMBR the NG-RAN node refused is still recorded as held")
	}

	if sender := ran.Conn.(*fakeNGAPSender); len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestUEContextModificationResponseKeepsTheUEAMBR(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	ueConn := amf.NewUeConnForTest(ran, 1, 1)

	if err := ueConn.SendUEContextModification(context.Background(), models.Ambr{
		Uplink:   models.MustParseBitRate("50 Mbps"),
		Downlink: models.MustParseBitRate("50 Mbps"),
	}); err != nil {
		t.Fatal(err)
	}

	HandleUEContextModificationResponse(context.Background(), amfInstance, ran, &ngap.UEContextModificationResponse{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(1)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(1)),
	})

	if _, ok := ueConn.HeldUEAMBR(); !ok {
		t.Fatal("an accepted UE-AMBR was dropped")
	}
}
