// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleInitialContextSetupFailure_MissingCause(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)
	amfInstance := newTestAMF()
	msg := decode.InitialContextSetupFailure{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
	}

	ngap.HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(sender.SentUEContextReleaseCommands))
	}
}

func TestHandleInitialContextSetupFailure_UnknownRanUeNgapID(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	msg := decode.InitialContextSetupFailure{
		AMFUENGAPID: 1,
		RANUENGAPID: 99,
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
		},
	}

	ngap.HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)
}

func TestHandleInitialContextSetupFailure_NilAmfUe(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[1] = ranUe

	msg := decode.InitialContextSetupFailure{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
		},
	}

	ngap.HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)
}

func TestHandleInitialContextSetupFailure_T3550Running(t *testing.T) {
	ran := newTestRadio()
	fakeSmf := &FakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog
	amfUe.ForceState(amf.ContextSetup)
	amfUe.T3550 = amf.NewTimer(time.Hour, 4, func(int32) {}, func() {})

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	ran.RanUEs[1] = ranUe

	msg := decode.InitialContextSetupFailure{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
		},
	}

	ngap.HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

	if amfUe.T3550 != nil {
		t.Error("expected T3550 to be nil after failure")
	}

	if amfUe.GetState() != amf.Deregistered {
		t.Errorf("expected state Deregistered, got %s", amfUe.GetState())
	}
}

func TestHandleInitialContextSetupFailure_PDUSessionFailureForwardedToSmf(t *testing.T) {
	ran := newTestRadio()
	fakeSmf := &FakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	ran.RanUEs[1] = ranUe

	transfer := []byte{0xEE, 0xFF}

	msg := decode.InitialContextSetupFailure{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
		},
		PDUSessionResourceFailedToSetupItems: []ngapType.PDUSessionResourceFailedToSetupItemCxtFail{
			{
				PDUSessionID: ngapType.PDUSessionID{Value: 1},
				PDUSessionResourceSetupUnsuccessfulTransfer: transfer,
			},
		},
	}

	ngap.HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

	if len(fakeSmf.PduResSetupFailCalls) != 1 {
		t.Fatalf("expected 1 PduResSetupFail call, got %d", len(fakeSmf.PduResSetupFailCalls))
	}

	if fakeSmf.PduResSetupFailCalls[0].SmContextRef != "ref-session-1" {
		t.Errorf("SmContextRef = %q, want %q", fakeSmf.PduResSetupFailCalls[0].SmContextRef, "ref-session-1")
	}
}
