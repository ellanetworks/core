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
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleHandoverFailure_MissingCause(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	sender := ran.Conn.(*fakeNGAPSender)
	amfInstance := newTestAMF()
	msg := decode.HandoverFailure{AMFUENGAPID: 1}

	ngap.HandleHandoverFailure(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

// TestHandleHandoverFailure_SourceUeContextDetached verifies that a handover
// failure is handled gracefully when the source UE's AMF UE context has been
// detached (e.g. due to a concurrent deregistration).
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

	// Simulate the AMF UE being detached from the source (deregistration race).
	amfUe.Conn().AMFForTest().ReleaseNasConnection(amfUe, nil)

	msg := decode.HandoverFailure{
		AMFUENGAPID: 200,
		Cause: &ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem,
			},
		},
	}

	ngap.HandleHandoverFailure(context.Background(), amfInstance, targetRan, msg)

	if len(sourceSender.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected 1 HandoverPreparationFailure on source radio, got %d", len(sourceSender.SentHandoverPreparationFailures))
	}

	// The target rejected preparation, so it holds no context: no UE Context Release
	// Command is sent; the target association is dropped locally (TS 38.413 §8.4.2.3).
	if len(targetSender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand to the target, got %d", len(targetSender.SentUEContextReleaseCommands))
	}

	if amfInstance.FindUEByAmfUeNgapID(targetRan, 200) != nil {
		t.Fatal("target UE association must be dropped locally on HANDOVER FAILURE")
	}
}

// TestHandleHandoverFailure_DropsTargetLocally verifies that on HANDOVER FAILURE the AMF
// fails the source, clears the handover, and drops the target locally with no UE Context
// Release Command (the target holds no context, TS 38.413 §8.4.2.3).
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

	msg := decode.HandoverFailure{
		AMFUENGAPID: 200,
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem},
		},
	}

	ngap.HandleHandoverFailure(context.Background(), amfInstance, targetRan, msg)

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

// TestHandleHandoverFailure_NotFromPreparedTarget verifies a HANDOVER FAILURE that
// arrives on an association other than the prepared target (here the source) is
// ignored and does not tear down the in-flight handover.
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

	// Failure arrives on the SOURCE association (AMF UE NGAP ID 100), not the
	// prepared target (200).
	msg := decode.HandoverFailure{
		AMFUENGAPID: 100,
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem},
		},
	}

	ngap.HandleHandoverFailure(context.Background(), amfInstance, sourceRan, msg)

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
