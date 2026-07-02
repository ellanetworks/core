// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	ran := newTestRadio(newTestAMF())
	sender := ran.Conn.(*fakeNGAPSender)
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

func TestHandleInitialContextSetupFailure_UnknownAmfUeNgapID(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	sender := ran.Conn.(*fakeNGAPSender)
	amfInstance := newTestAMF()

	msg := decode.InitialContextSetupFailure{
		AMFUENGAPID: 999,
		RANUENGAPID: 99,
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
		},
	}

	ngap.HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

	errInd := assertSingleErrorIndication(t, sender, ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 999, 99)
}

func TestHandleInitialContextSetupFailure_NilUeContext(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()

	amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)

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
	ran := newTestRadio(newTestAMF())
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)

	amfUe := amf.NewUeContext()
	amfUe.Log = logger.AmfLog
	amfUe.ForceRegStepForTest(amf.RegStepContextSetup)
	conn := amfUe.NasConn()
	conn.T3550.Arm(time.Hour, 4, func(int32) {}, func() {})

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	msg := decode.InitialContextSetupFailure{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
		},
	}

	ngap.HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

	if conn.T3550.Active() {
		t.Error("expected T3550 to be nil after failure")
	}

	if amfUe.State() != amf.Deregistered {
		t.Errorf("expected state Deregistered, got %s", amfUe.State())
	}
}

func TestHandleInitialContextSetupFailure_PDUSessionFailureForwardedToSmf(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)

	amfUe := amf.NewUeContext()
	amfUe.Log = logger.AmfLog
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

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
