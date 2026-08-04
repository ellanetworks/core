// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	libngap "github.com/ellanetworks/core/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleInitialContextSetupFailure_MissingCause(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)
	msg := &libngap.InitialContextSetupFailure{
		AMFUENGAPID: libngap.Ptr(libngap.AMFUENGAPID(1)),
		RANUENGAPID: libngap.Ptr(libngap.RANUENGAPID(1)),
	}

	ngap.HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(sender.SentUEContextReleaseCommands))
	}
}

func TestHandleInitialContextSetupFailure_UnknownAmfUeNgapID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	msg := &libngap.InitialContextSetupFailure{
		AMFUENGAPID: libngap.Ptr(libngap.AMFUENGAPID(999)),
		RANUENGAPID: libngap.Ptr(libngap.RANUENGAPID(99)),
		Cause:       &libngap.Cause{Group: libngap.CauseGroupRadioNetwork, Value: libngap.CauseRadioNetworkUnspecified},
	}

	ngap.HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

	errInd := assertSingleErrorIndication(t, sender, ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 999, 99)
}

func TestHandleInitialContextSetupFailure_NilUeContext(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	msg := &libngap.InitialContextSetupFailure{
		AMFUENGAPID: libngap.Ptr(libngap.AMFUENGAPID(10)),
		RANUENGAPID: libngap.Ptr(libngap.RANUENGAPID(1)),
		Cause:       &libngap.Cause{Group: libngap.CauseGroupRadioNetwork, Value: libngap.CauseRadioNetworkUnspecified},
	}

	ngap.HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)
}

func TestHandleInitialContextSetupFailure_T3550Running(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.ForceRegStepForTest(amf.RegStepContextSetup)

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	conn := amfUe.Conn()
	conn.NASGuardForTest().Arm(time.Hour, 4, func(int32) {}, func() {})

	msg := &libngap.InitialContextSetupFailure{
		AMFUENGAPID: libngap.Ptr(libngap.AMFUENGAPID(10)),
		RANUENGAPID: libngap.Ptr(libngap.RANUENGAPID(1)),
		Cause:       &libngap.Cause{Group: libngap.CauseGroupRadioNetwork, Value: libngap.CauseRadioNetworkUnspecified},
	}

	ngap.HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

	if conn.NASGuardForTest().Active() {
		t.Error("expected T3550 to be nil after failure")
	}

	if amfUe.State() != amf.Deregistered {
		t.Errorf("expected state Deregistered, got %s", amfUe.State())
	}
}

func TestHandleInitialContextSetupFailure_PDUSessionFailureForwardedToSmf(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{
		Ref:    "ref-session-1",
		Snssai: &models.Snssai{Sst: 1},
	}

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	transfer := []byte{0xEE, 0xFF}

	msg := &libngap.InitialContextSetupFailure{
		AMFUENGAPID:              libngap.Ptr(libngap.AMFUENGAPID(10)),
		RANUENGAPID:              libngap.Ptr(libngap.RANUENGAPID(1)),
		Cause:                    &libngap.Cause{Group: libngap.CauseGroupRadioNetwork, Value: libngap.CauseRadioNetworkUnspecified},
		PDUSessionResourceFailed: libngap.PDUSessionResourceFailedToSetupListCxtFail{{PDUSessionID: 1, Transfer: transfer}},
	}

	ngap.HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

	if len(fakeSmf.PduResSetupFailCalls) != 1 {
		t.Fatalf("expected 1 PduResSetupFail call, got %d", len(fakeSmf.PduResSetupFailCalls))
	}

	if fakeSmf.PduResSetupFailCalls[0].SmContextRef != "ref-session-1" {
		t.Errorf("SmContextRef = %q, want %q", fakeSmf.PduResSetupFailCalls[0].SmContextRef, "ref-session-1")
	}
}
