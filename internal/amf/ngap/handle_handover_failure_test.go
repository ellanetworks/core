// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/ngap"
)

func TestHandleHandoverFailure_MissingCause(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	sender := ran.Conn.(*fakeNGAPSender)
	amfInstance := newTestAMF()
	msg := ngap.HandoverFailure{AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(1))}

	HandleHandoverFailure(context.Background(), amfInstance, ran, &msg)

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandleHandoverFailure_SourceUeContextDetached(t *testing.T) {
	amfInstance := newTestAMF()
	sourceRan := newTestRadio(amfInstance)
	targetRan := newTestRadio(amfInstance)
	sourceSender := sourceRan.Conn.(*fakeNGAPSender)
	targetSender := targetRan.Conn.(*fakeNGAPSender)

	amfUe := amf.NewUeContext()

	sourceUe := amf.NewUeConnForTest(sourceRan, 10, 100, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetUe := amf.NewUeConnForTest(targetRan, 2, 200, logger.AmfLog)

	err := amf.SetHandoverForTest(sourceUe, targetUe)
	if err != nil {
		t.Fatalf("SetHandoverForTest: %v", err)
	}

	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), targetRan)

	amfUe.Conn().AMFForTest().ReleaseNasConnection(amfUe, nil)

	msg := ngap.HandoverFailure{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(200)),
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHOFailureInTarget},
	}

	HandleHandoverFailure(context.Background(), amfInstance, targetRan, &msg)

	if len(sourceSender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected 1 HandoverPreparationFailure on source radio, got %d", len(sourceSender.SentHandoverPreparationFailures))
	}

	if len(targetSender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand to the target, got %d", len(targetSender.SentUEContextReleaseCommands))
	}

	if amfInstance.FindUEByAmfUeNgapID(targetRan, 200) != nil {
		t.Fatal("target UE association must be dropped locally on HANDOVER FAILURE")
	}
}

// TS 38.413 §8.4.2.3
func TestHandleHandoverFailure_DropsTargetLocally(t *testing.T) {
	amfInstance := newTestAMF()
	sourceRan := newTestRadio(amfInstance)
	targetRan := newTestRadio(amfInstance)
	sourceSender := sourceRan.Conn.(*fakeNGAPSender)
	targetSender := targetRan.Conn.(*fakeNGAPSender)

	amfUe := amf.NewUeContext()

	sourceUe := amf.NewUeConnForTest(sourceRan, 10, 100, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetUe := amf.NewUeConnForTest(targetRan, 2, 200, logger.AmfLog)

	if err := amf.SetHandoverForTest(sourceUe, targetUe); err != nil {
		t.Fatalf("SetHandoverForTest: %v", err)
	}

	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), targetRan)

	msg := ngap.HandoverFailure{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(200)),
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHOFailureInTarget},
	}

	HandleHandoverFailure(context.Background(), amfInstance, targetRan, &msg)

	if len(sourceSender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected 1 HandoverPreparationFailure on source, got %d", len(sourceSender.SentHandoverPreparationFailures))
	}

	if len(targetSender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand to target, got %d", len(targetSender.SentUEContextReleaseCommands))
	}

	if amfInstance.FindUEByAmfUeNgapID(targetRan, 200) != nil {
		t.Fatal("target UE association must be dropped locally")
	}

	if amfInstance.HandoverInProgress(amfUe) {
		t.Fatal("handover must be cleared after failure")
	}
}

func TestHandleHandoverFailure_NotFromPreparedTarget(t *testing.T) {
	amfInstance := newTestAMF()
	sourceRan := newTestRadio(amfInstance)
	targetRan := newTestRadio(amfInstance)
	sourceSender := sourceRan.Conn.(*fakeNGAPSender)
	targetSender := targetRan.Conn.(*fakeNGAPSender)

	amfUe := amf.NewUeContext()

	sourceUe := amf.NewUeConnForTest(sourceRan, 10, 100, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetUe := amf.NewUeConnForTest(targetRan, 2, 200, logger.AmfLog)

	if err := amf.SetHandoverForTest(sourceUe, targetUe); err != nil {
		t.Fatalf("SetHandoverForTest: %v", err)
	}

	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), targetRan)

	msg := ngap.HandoverFailure{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(100)),
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHOFailureInTarget},
	}

	HandleHandoverFailure(context.Background(), amfInstance, sourceRan, &msg)

	if len(sourceSender.SentHandoverPreparationFailures) != 0 {
		t.Fatalf("expected no HandoverPreparationFailure, got %d", len(sourceSender.SentHandoverPreparationFailures))
	}

	if len(targetSender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(targetSender.SentUEContextReleaseCommands))
	}

	if !amfInstance.HandoverInProgress(amfUe) {
		t.Fatal("expected the handover to remain in progress")
	}
}

// TS 38.413 §9.3.1.3
func TestHandleHandoverFailure_DoesNotRelayTargetDiagnosticsToSource(t *testing.T) {
	amfInstance := newTestAMF()
	sourceRan := newTestRadio(amfInstance)
	targetRan := newTestRadio(amfInstance)
	sourceSender := sourceRan.Conn.(*fakeNGAPSender)

	amfUe := amf.NewUeContext()

	sourceUe := amf.NewUeConnForTest(sourceRan, 10, 100, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetUe := amf.NewUeConnForTest(targetRan, 2, 200, logger.AmfLog)

	if err := amf.SetHandoverForTest(sourceUe, targetUe); err != nil {
		t.Fatalf("SetHandoverForTest: %v", err)
	}

	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), targetRan)

	HandleHandoverFailure(context.Background(), amfInstance, targetRan, &ngap.HandoverFailure{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(200)),
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHOFailureInTarget},
		CriticalityDiagnostics: &ngap.CriticalityDiagnostics{
			IEsCriticalityDiagnostics: []ngap.CriticalityDiagnosticsIEItem{{
				IECriticality: ngap.CriticalityReject,
				IEID:          ngap.ProtocolIEID(101),
				TypeOfError:   ngap.TypeOfErrorNotUnderstood,
			}},
		},
	})

	if len(sourceSender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected 1 HandoverPreparationFailure on source, got %d", len(sourceSender.SentHandoverPreparationFailures))
	}

	if got := sourceSender.SentHandoverPreparationFailures[0].CriticalityDiagnostics; got != nil {
		t.Errorf("failure to source carries the target's diagnostics %+v, want none", got)
	}
}

// TS 38.413 §8.4.1.3
func TestHandleHandoverFailure_RelaysTargetToSourceFailureContainer(t *testing.T) {
	amfInstance := newTestAMF()
	sourceRan := newTestRadio(amfInstance)
	targetRan := newTestRadio(amfInstance)
	sourceSender := sourceRan.Conn.(*fakeNGAPSender)

	amfUe := amf.NewUeContext()

	sourceUe := amf.NewUeConnForTest(sourceRan, 10, 100, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetUe := amf.NewUeConnForTest(targetRan, 2, 200, logger.AmfLog)

	if err := amf.SetHandoverForTest(sourceUe, targetUe); err != nil {
		t.Fatalf("SetHandoverForTest: %v", err)
	}

	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), targetRan)

	container := ngap.TargettoSourceFailureTransparentContainer{0xde, 0xad, 0xbe, 0xef}

	HandleHandoverFailure(context.Background(), amfInstance, targetRan, &ngap.HandoverFailure{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(200)),
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHOFailureInTarget},
		TargettoSourceFailureTransparentContainer: container,
	})

	if len(sourceSender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected 1 HandoverPreparationFailure, got %d", len(sourceSender.SentHandoverPreparationFailures))
	}

	got := sourceSender.SentHandoverPreparationFailures[0].TargettoSourceFailureTransparentContainer
	if !bytes.Equal(got, container) {
		t.Fatalf("relayed container = %x, want %x", got, container)
	}
}

func TestHandleHandoverFailure_NoContainerToRelay(t *testing.T) {
	amfInstance := newTestAMF()
	sourceRan := newTestRadio(amfInstance)
	targetRan := newTestRadio(amfInstance)
	sourceSender := sourceRan.Conn.(*fakeNGAPSender)

	amfUe := amf.NewUeContext()

	sourceUe := amf.NewUeConnForTest(sourceRan, 10, 100, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	targetUe := amf.NewUeConnForTest(targetRan, 2, 200, logger.AmfLog)

	if err := amf.SetHandoverForTest(sourceUe, targetUe); err != nil {
		t.Fatalf("SetHandoverForTest: %v", err)
	}

	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), targetRan)

	HandleHandoverFailure(context.Background(), amfInstance, targetRan, &ngap.HandoverFailure{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(200)),
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHOFailureInTarget},
	})

	if len(sourceSender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected 1 HandoverPreparationFailure, got %d", len(sourceSender.SentHandoverPreparationFailures))
	}

	if got := sourceSender.SentHandoverPreparationFailures[0].TargettoSourceFailureTransparentContainer; got != nil {
		t.Fatalf("relayed container = %x, want none", got)
	}
}
