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
	sender := ran.NGAPSender.(*fakeNGAPSender)
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
	sourceRan := newTestRadio(newTestAMF())
	targetRan := newTestRadio(newTestAMF())
	sourceSender := sourceRan.NGAPSender.(*fakeNGAPSender)
	targetSender := targetRan.NGAPSender.(*fakeNGAPSender)
	amfInstance := newTestAMF()

	amfUe := amf.NewUeContext()
	amfUe.Log = logger.AmfLog

	sourceUe := amf.NewRanUeForTest(sourceRan, 10, 100, logger.AmfLog)
	amfUe.AttachRanUe(sourceUe)

	targetUe := amf.NewRanUeForTest(targetRan, 2, 200, logger.AmfLog)

	err := amf.AttachSourceUeTargetUe(sourceUe, targetUe)
	if err != nil {
		t.Fatalf("AttachSourceUeTargetUe: %v", err)
	}

	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan
	amfInstance.Radios[new(sctp.SCTPConn)] = targetRan

	// Simulate the AMF UE being detached from the source (deregistration race).
	amfUe.ReleaseNasConnection(nil)

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

	if len(targetSender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected 1 UEContextReleaseCommand on target radio, got %d", len(targetSender.SentUEContextReleaseCommands))
	}
}

// TestHandleHandoverFailure_NotFromPreparedTarget verifies a HANDOVER FAILURE that
// arrives on an association other than the prepared target (here the source) is
// ignored and does not tear down the in-flight handover.
func TestHandleHandoverFailure_NotFromPreparedTarget(t *testing.T) {
	sourceRan := newTestRadio(newTestAMF())
	targetRan := newTestRadio(newTestAMF())
	sourceSender := sourceRan.NGAPSender.(*fakeNGAPSender)
	targetSender := targetRan.NGAPSender.(*fakeNGAPSender)
	amfInstance := newTestAMF()

	amfUe := amf.NewUeContext()
	amfUe.Log = logger.AmfLog

	sourceUe := amf.NewRanUeForTest(sourceRan, 10, 100, logger.AmfLog)
	amfUe.AttachRanUe(sourceUe)

	targetUe := amf.NewRanUeForTest(targetRan, 2, 200, logger.AmfLog)

	if err := amf.AttachSourceUeTargetUe(sourceUe, targetUe); err != nil {
		t.Fatalf("AttachSourceUeTargetUe: %v", err)
	}

	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan
	amfInstance.Radios[new(sctp.SCTPConn)] = targetRan

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
