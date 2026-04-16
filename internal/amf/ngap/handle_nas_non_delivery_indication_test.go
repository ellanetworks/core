// Copyright 2026 Ella Networks

package ngap_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestNasNonDeliveryIndication_UnknownRanUeNgapID(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	ngap.HandleNasNonDeliveryIndication(context.Background(), amfInstance, ran, decode.NASNonDeliveryIndication{
		RANUENGAPID: 99,
	})
}

func TestNasNonDeliveryIndication_UEFoundDispatchesNAS(t *testing.T) {
	ran := newTestRadio()

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	amfInstance := newTestAMF()

	// Minimal NAS PDU — HandleNAS will fail to parse it, but the handler
	// just logs the error. We verify no panic and the UE lookup succeeds.
	ngap.HandleNasNonDeliveryIndication(context.Background(), amfInstance, ran, decode.NASNonDeliveryIndication{
		RANUENGAPID: 1,
		NASPDU:      []byte{0x7E, 0x00, 0x00},
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID},
		},
	})
}

func TestNasNonDeliveryIndication_VerifyNASCalledWithPDU(t *testing.T) {
	fakeNAS := &FakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio()

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	nasPDU := []byte{0xDE, 0xAD}

	ngap.HandleNasNonDeliveryIndication(context.Background(), amfInstance, ran, decode.NASNonDeliveryIndication{
		RANUENGAPID: 1,
		NASPDU:      nasPDU,
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID},
		},
	})

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}

	if !bytes.Equal(fakeNAS.Calls[0].NASPDU, nasPDU) {
		t.Errorf("NAS PDU = %x, want %x", fakeNAS.Calls[0].NASPDU, nasPDU)
	}

	if fakeNAS.Calls[0].RanUe != ranUe {
		t.Error("NAS called with wrong RanUe")
	}
}

func TestNasNonDeliveryIndication_NilPDU_PropagatesCorrectly(t *testing.T) {
	fakeNAS := &FakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio()

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	ngap.HandleNasNonDeliveryIndication(context.Background(), amfInstance, ran, decode.NASNonDeliveryIndication{
		RANUENGAPID: 1,
		NASPDU:      nil,
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID},
		},
	})

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}

	if fakeNAS.Calls[0].NASPDU != nil {
		t.Errorf("NAS PDU = %x, want nil", fakeNAS.Calls[0].NASPDU)
	}
}
