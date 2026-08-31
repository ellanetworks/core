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

func TestHandleHandoverCancel_UnknownRanUeNgapID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	msg := &ngap.HandoverCancel{
		AMFUENGAPID: 1099511627775,
		RANUENGAPID: 99,
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified},
	}

	HandleHandoverCancel(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}

	errInd := sender.SentErrorIndications[0]

	wantRadioNetworkCause := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnknownLocalUENGAPID}
	if errInd.Cause == nil || *errInd.Cause != wantRadioNetworkCause {
		t.Errorf("cause = %v, want unknown-local-UE-NGAP-ID", errInd.Cause)
	}
}

func TestHandleHandoverCancel_UnknownAmfUeNgapID(t *testing.T) {
	amfInstance := newTestAMF()
	sourceRan := newTestRadio(amfInstance)
	sourceSender := sourceRan.Conn.(*fakeNGAPSender)

	targetRan := newTestRadio(amfInstance)
	targetSender := targetRan.Conn.(*fakeNGAPSender)

	amf.NewUeConnForTest(sourceRan, 1, 10, logger.AmfLog)

	msg := &ngap.HandoverCancel{
		AMFUENGAPID: 999,
		RANUENGAPID: 1,
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified},
	}

	HandleHandoverCancel(context.Background(), amfInstance, sourceRan, msg)

	errInd := assertSingleErrorIndication(t, sourceSender, ngap.CauseRadioNetworkUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 999, 1)

	if len(sourceSender.SentHandoverCancelAcknowledges) != 0 {
		t.Errorf("expected no HandoverCancelAcknowledge, got %d", len(sourceSender.SentHandoverCancelAcknowledges))
	}

	if len(targetSender.SentUEContextReleaseCommands) != 0 {
		t.Errorf("expected no UEContextReleaseCommand on target, got %d", len(targetSender.SentUEContextReleaseCommands))
	}
}

func TestHandleHandoverCancel_HappyPath(t *testing.T) {
	amfInstance := newTestAMF()
	sourceRan := newTestRadio(amfInstance)
	sourceSender := sourceRan.Conn.(*fakeNGAPSender)

	targetRan := newTestRadio(amfInstance)
	targetSender := targetRan.Conn.(*fakeNGAPSender)

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, 10, logger.AmfLog)
	targetUe := amf.NewUeConnForTest(targetRan, 2, 20, logger.AmfLog)

	amfUe := amf.NewUeContext()
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	if err := amf.SetHandoverForTest(sourceUe, targetUe); err != nil {
		t.Fatalf("SetHandoverForTest: %v", err)
	}

	if _, ok := amfInstance.MarkHandoverPrepared(amfUe, nil); !ok {
		t.Fatal("MarkHandoverPrepared")
	}

	msg := &ngap.HandoverCancel{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified},
	}

	HandleHandoverCancel(context.Background(), amfInstance, sourceRan, msg)

	if len(targetSender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected 1 UEContextReleaseCommand on target, got %d", len(targetSender.SentUEContextReleaseCommands))
	}

	if targetUe.ReleaseAction != amf.UeContextReleaseHandover {
		t.Errorf("expected targetUe.ReleaseAction = UeContextReleaseHandover, got %d", targetUe.ReleaseAction)
	}

	if len(sourceSender.SentHandoverCancelAcknowledges) != 1 {
		t.Fatalf("expected 1 HandoverCancelAcknowledge on source, got %d", len(sourceSender.SentHandoverCancelAcknowledges))
	}

	ack := sourceSender.SentHandoverCancelAcknowledges[0]
	if ack.AMFUENGAPID == nil || *ack.AMFUENGAPID != 10 || ack.RANUENGAPID == nil || *ack.RANUENGAPID != 1 {
		t.Errorf("HandoverCancelAcknowledge IDs = (%v, %v), want (10, 1)", ack.AMFUENGAPID, ack.RANUENGAPID)
	}
}

func TestHandleHandoverCancel_Preparing_ReleasesTarget(t *testing.T) {
	amfInstance := newTestAMF()
	sourceRan := newTestRadio(amfInstance)
	sourceSender := sourceRan.Conn.(*fakeNGAPSender)

	targetRan := newTestRadio(amfInstance)
	targetSender := targetRan.Conn.(*fakeNGAPSender)

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, 10, logger.AmfLog)
	targetUe := amf.NewUeConnForTest(targetRan, 2, 20, logger.AmfLog)

	amfUe := amf.NewUeContext()
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	if err := amf.SetHandoverForTest(sourceUe, targetUe); err != nil {
		t.Fatalf("SetHandoverForTest: %v", err)
	}

	HandleHandoverCancel(context.Background(), amfInstance, sourceRan, &ngap.HandoverCancel{AMFUENGAPID: 10, RANUENGAPID: 1})

	if len(targetSender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected 1 UEContextReleaseCommand to the preparing target, got %d", len(targetSender.SentUEContextReleaseCommands))
	}

	if amfInstance.HandoverInProgress(amfUe) {
		t.Error("the handover FSM must be cleared after a preparing cancel")
	}

	if len(sourceSender.SentHandoverCancelAcknowledges) != 1 {
		t.Fatalf("expected 1 HandoverCancelAcknowledge on source, got %d", len(sourceSender.SentHandoverCancelAcknowledges))
	}
}
