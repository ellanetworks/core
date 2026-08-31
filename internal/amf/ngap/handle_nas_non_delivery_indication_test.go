// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
)

func TestNASNonDeliveryIndication_UnknownAmfUeNgapID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	HandleNASNonDeliveryIndication(context.Background(), amfInstance, ran, &ngap.NASNonDeliveryIndication{
		RANUENGAPID: 99,
		AMFUENGAPID: 999,
	})

	errInd := assertSingleErrorIndication(t, sender, ngap.CauseRadioNetworkUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 999, 99)
}

// TS 38.413 §8.6.4
func TestNASNonDeliveryIndication_DoesNotReprocessNAS(t *testing.T) {
	fakeNAS := &fakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	HandleNASNonDeliveryIndication(context.Background(), amfInstance, ran, &ngap.NASNonDeliveryIndication{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
		NASPDU:      ngap.NASPDU{0xDE, 0xAD},
		Cause:       ngap.Ptr(ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnknownLocalUENGAPID}),
	})

	if len(fakeNAS.Calls) != 0 {
		t.Fatalf("NAS handler called %d time(s); NAS Non-Delivery is report-only and must not reprocess the undelivered downlink PDU", len(fakeNAS.Calls))
	}
}
