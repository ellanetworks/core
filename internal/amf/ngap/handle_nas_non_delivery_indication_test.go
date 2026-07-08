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
	"github.com/free5gc/ngap/ngapType"
)

func TestNasNonDeliveryIndication_UnknownAmfUeNgapID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	ngap.HandleNasNonDeliveryIndication(context.Background(), amfInstance, ran, decode.NASNonDeliveryIndication{
		RANUENGAPID: 99,
		AMFUENGAPID: 999,
	})

	errInd := assertSingleErrorIndication(t, sender, ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 999, 99)
}

func TestNasNonDeliveryIndication_UEFound_ReportOnly(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	// TS 38.413 §8.6.4: report-only — the handler resolves the UE and records
	// liveness; the (undelivered downlink) NAS-PDU is not acted on. Verify no panic.
	ngap.HandleNasNonDeliveryIndication(context.Background(), amfInstance, ran, decode.NASNonDeliveryIndication{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
		NASPDU:      []byte{0x7E, 0x00, 0x00},
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID},
		},
	})
}

// The NAS-PDU IE is the downlink message the RAN could not deliver; feeding it back
// into the uplink NAS path would fail the downlink/uplink integrity check and perturb
// the uplink NAS count (TS 38.413 §8.6.4). The handler must not invoke the NAS layer.
func TestNasNonDeliveryIndication_DoesNotReprocessNAS(t *testing.T) {
	fakeNAS := &fakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	ngap.HandleNasNonDeliveryIndication(context.Background(), amfInstance, ran, decode.NASNonDeliveryIndication{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
		NASPDU:      []byte{0xDE, 0xAD},
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID},
		},
	})

	if len(fakeNAS.Calls) != 0 {
		t.Fatalf("NAS handler called %d time(s); NAS Non-Delivery is report-only and must not reprocess the undelivered downlink PDU", len(fakeNAS.Calls))
	}
}
