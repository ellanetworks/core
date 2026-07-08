// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandoverNotify_UnknownRanUeNgapID(t *testing.T) {
	sender := &fakeNGAPSender{}
	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))
	amfInstance := amf.New(nil, nil, nil)

	msg := decode.HandoverNotify{AMFUENGAPID: 1, RANUENGAPID: 99}

	ngap.HandleHandoverNotify(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}

	errInd := sender.SentErrorIndications[0]
	if errInd.Cause == nil {
		t.Fatal("expected Cause in ErrorIndication, got nil")
	}

	if errInd.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Fatalf("expected RadioNetwork cause, got present=%d", errInd.Cause.Present)
	}

	if errInd.Cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID {
		t.Fatalf("expected UnknownLocalUENGAPID, got %d", errInd.Cause.RadioNetwork.Value)
	}

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(sender.SentUEContextReleaseCommands))
	}
}

func TestHandoverNotify_NilUeContext(t *testing.T) {
	sender := &fakeNGAPSender{}
	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	amf.NewUeConnForTest(ran, 2, 1, logger.AmfLog)

	amfInstance := amf.New(nil, nil, nil)

	msg := decode.HandoverNotify{AMFUENGAPID: 1, RANUENGAPID: 2}

	ngap.HandleHandoverNotify(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(sender.SentUEContextReleaseCommands))
	}
}

func TestHandoverNotify_NoSourceUe(t *testing.T) {
	sender := &fakeNGAPSender{}
	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	amfUe := amf.NewUeContext()

	targetUe := amf.NewUeConnForTest(ran, 2, 1, logger.AmfLog)
	targetUe.AMFForTest().AttachUeConn(amfUe, targetUe)
	// No handover installed, so HandoverSource() is nil.

	amfInstance := amf.New(nil, nil, nil)

	msg := decode.HandoverNotify{AMFUENGAPID: 1, RANUENGAPID: 2}

	ngap.HandleHandoverNotify(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(sender.SentUEContextReleaseCommands))
	}
}

func TestHandoverNotify_HappyPath(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sourceNGAPSender,
	}
	sourceRan.BindAMFForTest(amfInstance)

	amfUe := amf.NewUeContext()

	sourceUe := amf.NewUeConnForTest(sourceRan, 10, 100, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: targetNGAPSender,
	}
	targetRan.BindAMFForTest(amfInstance)

	targetUe := amf.NewUeConnForTest(targetRan, 2, 1, logger.AmfLog)

	err := amf.AttachSourceUeTargetUe(sourceUe, targetUe)
	if err != nil {
		t.Fatalf("failed to attach source/target: %v", err)
	}

	// Handover Notify requires a prepared handover (the acknowledge step ran).
	amfInstance.MarkHandoverPrepared(amfUe, nil)

	msg := decode.HandoverNotify{AMFUENGAPID: 1, RANUENGAPID: 2}

	ngap.HandleHandoverNotify(context.Background(), amfInstance, targetRan, msg)

	if len(sourceNGAPSender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected 1 UEContextReleaseCommand to source RAN, got %d", len(sourceNGAPSender.SentUEContextReleaseCommands))
	}

	cmd := sourceNGAPSender.SentUEContextReleaseCommands[0]

	if cmd.AmfUeNgapID != 100 {
		t.Errorf("expected AmfUeNgapID=100 (source), got %d", cmd.AmfUeNgapID)
	}

	if cmd.RanUeNgapID != 10 {
		t.Errorf("expected RanUeNgapID=10 (source), got %d", cmd.RanUeNgapID)
	}

	if cmd.CausePresent != ngapType.CausePresentRadioNetwork {
		t.Errorf("expected CausePresent=RadioNetwork, got %d", cmd.CausePresent)
	}

	if cmd.Cause != ngapType.CauseRadioNetworkPresentSuccessfulHandover {
		t.Errorf("expected Cause=SuccessfulHandover, got %d", cmd.Cause)
	}

	if sourceUe.ReleaseAction != amf.UeContextReleaseHandover {
		t.Errorf("expected source UE ReleaseAction=UeContextReleaseHandover, got %d", sourceUe.ReleaseAction)
	}

	if amfUe.Conn() != targetUe {
		t.Error("expected UeContext.UeConn to be attached to targetUe")
	}

	if len(targetNGAPSender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(targetNGAPSender.SentErrorIndications))
	}
}

// TestHandoverNotify_ReleasesRejectedSessions verifies that on handover completion
// the sessions the target admitted are completed while a session the target did not
// admit is released, not left leaking in the core (TS 23.501 §5.30.3.5 / TS 23.401
// §5.5.1.2.2), mirroring the MME.
func TestHandoverNotify_ReleasesRejectedSessions(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	fakeSmf := &fakeSmfSbi{}
	amfInstance.Session = fakeSmf

	sourceRan := &amf.Radio{Log: logger.AmfLog, Conn: &fakeNGAPSender{}}
	sourceRan.BindAMFForTest(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{Ref: "ref-1"}
	amfUe.SmContextList[2] = &amf.SmContext{Ref: "ref-2"}

	sourceUe := amf.NewUeConnForTest(sourceRan, 10, 100, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetRan := &amf.Radio{Log: logger.AmfLog, Conn: &fakeNGAPSender{}}
	targetRan.BindAMFForTest(amfInstance)

	targetUe := amf.NewUeConnForTest(targetRan, 2, 1, logger.AmfLog)
	if err := amf.AttachSourceUeTargetUe(sourceUe, targetUe); err != nil {
		t.Fatalf("failed to attach source/target: %v", err)
	}

	// The target admitted session 1 only; session 2 was rejected at the acknowledge.
	amfInstance.MarkHandoverPrepared(amfUe, map[uint8]struct{}{1: {}})

	ngap.HandleHandoverNotify(context.Background(), amfInstance, targetRan, decode.HandoverNotify{AMFUENGAPID: 1, RANUENGAPID: 2})

	if len(fakeSmf.N2HandoverCompleteCalls) != 1 || fakeSmf.N2HandoverCompleteCalls[0] != "ref-1" {
		t.Fatalf("expected only the admitted session ref-1 completed, got %v", fakeSmf.N2HandoverCompleteCalls)
	}

	if len(fakeSmf.ReleaseSmContextCalls) != 1 || fakeSmf.ReleaseSmContextCalls[0] != "ref-2" {
		t.Fatalf("expected the rejected session ref-2 released, got %v", fakeSmf.ReleaseSmContextCalls)
	}

	if _, ok := amfUe.SmContextFindByPDUSessionID(2); ok {
		t.Fatal("the rejected session's SM context must be dropped from the UE")
	}

	if _, ok := amfUe.SmContextFindByPDUSessionID(1); !ok {
		t.Fatal("the admitted session's SM context must be retained")
	}
}

// TestHandoverNotify_FromNonTarget_Dropped verifies a HANDOVER NOTIFY arriving on a
// UeConn that is not the prepared handover target is dropped before any SMF side
// effect or source release (the procedure is between the AMF and the prepared target).
func TestHandoverNotify_FromNonTarget_Dropped(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	fakeSmf := &fakeSmfSbi{}
	amfInstance.Session = fakeSmf

	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{Log: logger.AmfLog, Conn: sourceNGAPSender}
	sourceRan.BindAMFForTest(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{Ref: "ref-1"}

	sourceUe := amf.NewUeConnForTest(sourceRan, 10, 100, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetRan := &amf.Radio{Log: logger.AmfLog, Conn: &fakeNGAPSender{}}
	targetRan.BindAMFForTest(amfInstance)

	targetUe := amf.NewUeConnForTest(targetRan, 2, 1, logger.AmfLog)
	if err := amf.AttachSourceUeTargetUe(sourceUe, targetUe); err != nil {
		t.Fatalf("failed to attach source/target: %v", err)
	}

	amfInstance.MarkHandoverPrepared(amfUe, map[uint8]struct{}{1: {}})

	// An impostor UeConn on the target radio carrying the same AMF UE context but not
	// the prepared target sends the notify.
	impostor := amf.NewUeConnForTest(targetRan, 3, 4, logger.AmfLog)
	impostor.AMFForTest().AttachUeConn(amfUe, impostor)

	ngap.HandleHandoverNotify(context.Background(), amfInstance, targetRan, decode.HandoverNotify{AMFUENGAPID: 4, RANUENGAPID: 3})

	if len(fakeSmf.N2HandoverCompleteCalls) != 0 || len(fakeSmf.ReleaseSmContextCalls) != 0 {
		t.Fatalf("a notify from a non-target must not touch any SM context (complete=%v release=%v)",
			fakeSmf.N2HandoverCompleteCalls, fakeSmf.ReleaseSmContextCalls)
	}

	if len(sourceNGAPSender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("a notify from a non-target must not release the source, got %d", len(sourceNGAPSender.SentUEContextReleaseCommands))
	}

	if !amfInstance.HandoverInProgress(amfUe) {
		t.Fatal("the in-flight handover must be left intact after a spurious notify")
	}
}

func TestHandoverNotify_SmfUpdateFails_StillReleasesSource(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sourceNGAPSender,
	}
	sourceRan.BindAMFForTest(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.SmContextList[1] = &amf.SmContext{Ref: "ref-1"}

	sourceUe := amf.NewUeConnForTest(sourceRan, 10, 100, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: targetNGAPSender,
	}
	targetRan.BindAMFForTest(amfInstance)

	targetUe := amf.NewUeConnForTest(targetRan, 2, 1, logger.AmfLog)

	err := amf.AttachSourceUeTargetUe(sourceUe, targetUe)
	if err != nil {
		t.Fatalf("failed to attach source/target: %v", err)
	}

	// Handover Notify requires a prepared handover (the acknowledge step ran); the
	// admitted session's completion is what fails below.
	amfInstance.MarkHandoverPrepared(amfUe, map[uint8]struct{}{1: {}})

	fakeSmf := &fakeSmfSbi{
		N2HandoverCompleteErr: fmt.Errorf("smf unreachable"),
	}
	amfInstance.Session = fakeSmf

	msg := decode.HandoverNotify{AMFUENGAPID: 1, RANUENGAPID: 2}

	ngap.HandleHandoverNotify(context.Background(), amfInstance, targetRan, msg)

	if len(sourceNGAPSender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected 1 UEContextReleaseCommand to source RAN even when SMF fails, got %d", len(sourceNGAPSender.SentUEContextReleaseCommands))
	}

	if sourceUe.ReleaseAction != amf.UeContextReleaseHandover {
		t.Errorf("expected source UE ReleaseAction=UeContextReleaseHandover even when SMF fails, got %d", sourceUe.ReleaseAction)
	}

	if amfUe.Conn() != targetUe {
		t.Error("expected UeContext.UeConn to be attached to targetUe even when SMF fails")
	}
}
