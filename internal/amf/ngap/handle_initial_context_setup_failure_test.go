// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

func TestHandleInitialContextSetupFailure_MissingCause(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)
	msg := &ngap.InitialContextSetupFailure{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(1)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(1)),
	}

	HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(sender.SentUEContextReleaseCommands))
	}
}

func TestHandleInitialContextSetupFailure_UnknownAmfUeNgapID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	msg := &ngap.InitialContextSetupFailure{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(999)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(99)),
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified},
	}

	HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

	errInd := assertSingleErrorIndication(t, sender, ngap.CauseRadioNetworkUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 999, 99)
}

func TestHandleInitialContextSetupFailure_NilUeContext(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	msg := &ngap.InitialContextSetupFailure{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(10)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(1)),
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified},
	}

	HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("a resolvable connection with no UE context must be dropped silently, got %d error indications", len(sender.SentErrorIndications))
	}
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

	msg := &ngap.InitialContextSetupFailure{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(10)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(1)),
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified},
	}

	HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

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

	msg := &ngap.InitialContextSetupFailure{
		AMFUENGAPID:              ngap.Ptr(ngap.AMFUENGAPID(10)),
		RANUENGAPID:              ngap.Ptr(ngap.RANUENGAPID(1)),
		Cause:                    &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified},
		PDUSessionResourceFailed: ngap.PDUSessionResourceFailedToSetupListCxtFail{{PDUSessionID: 1, Transfer: transfer}},
	}

	HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

	if len(fakeSmf.PduResSetupFailCalls) != 1 {
		t.Fatalf("expected 1 PduResSetupFail call, got %d", len(fakeSmf.PduResSetupFailCalls))
	}

	if fakeSmf.PduResSetupFailCalls[0].SmContextRef != "ref-session-1" {
		t.Errorf("SmContextRef = %q, want %q", fakeSmf.PduResSetupFailCalls[0].SmContextRef, "ref-session-1")
	}
}

func TestHandleInitialContextSetupFailure_ReleasesNGRANStateForEverySession(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{Ref: "ref-session-1", Snssai: &models.Snssai{Sst: 1}}
	amfUe.SmContextList[2] = &amf.SmContext{Ref: "ref-session-2", Snssai: &models.Snssai{Sst: 1}}

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	if got := ueConn.ClaimN2Setup(amf.N2SetupInitialContext, []uint8{1, 2}); len(got) != 2 {
		t.Fatalf("claimed %v, want both PDU sessions", got)
	}

	if !ueConn.ClaimICS() {
		t.Fatal("could not claim the initial context setup")
	}

	msg := &ngap.InitialContextSetupFailure{
		AMFUENGAPID:              ngap.Ptr(ngap.AMFUENGAPID(10)),
		RANUENGAPID:              ngap.Ptr(ngap.RANUENGAPID(1)),
		Cause:                    &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified},
		PDUSessionResourceFailed: ngap.PDUSessionResourceFailedToSetupListCxtFail{{PDUSessionID: 1, Transfer: []byte{0xEE}}},
	}

	HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

	for _, id := range []uint8{1, 2} {
		if !ueConn.ClaimN2SetupSession(amf.N2SetupInitialContext, id) {
			t.Errorf("PDU session %d is still claimed after the NG-RAN node failed to establish the UE context", id)
		}
	}

	if !ueConn.ClaimICS() {
		t.Error("the initial context setup claim was not released, so a retry cannot re-attempt it")
	}
}

func TestHandleInitialContextSetupFailure_ReleasesNGRANStateWithoutAFailedList(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(fakeSmf)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{Ref: "ref-session-1", Snssai: &models.Snssai{Sst: 1}}

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	if !ueConn.ClaimN2SetupSession(amf.N2SetupInitialContext, 1) {
		t.Fatal("could not claim the PDU session")
	}

	msg := &ngap.InitialContextSetupFailure{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(10)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(1)),
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified},
	}

	HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)

	if !ueConn.ClaimN2SetupSession(amf.N2SetupInitialContext, 1) {
		t.Error("a failure carrying no PDU session list must still clear the NG-RAN state")
	}
}
