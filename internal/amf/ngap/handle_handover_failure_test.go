// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleHandoverFailure_MissingCause(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)
	amfInstance := newTestAMF()
	msg := decode.HandoverFailure{AMFUENGAPID: 1}

	ngap.HandleHandoverFailure(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

// TestHandleHandoverFailure_SourceAmfUeDetached verifies that a handover
// failure is handled gracefully when the source UE's AMF UE context has been
// detached (e.g. due to a concurrent deregistration).
func TestHandleHandoverFailure_SourceAmfUeDetached(t *testing.T) {
	sourceRan := newTestRadio()
	targetRan := newTestRadio()
	sourceSender := sourceRan.NGAPSender.(*FakeNGAPSender)
	targetSender := targetRan.NGAPSender.(*FakeNGAPSender)
	amfInstance := newTestAMF()

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	sourceUe := amf.NewRanUeForTest(sourceRan, 10, 100, logger.AmfLog)
	amfUe.AttachRanUe(sourceUe)

	targetUe := amf.NewRanUeForTest(targetRan, 2, 200, logger.AmfLog)

	err := amf.AttachSourceUeTargetUe(sourceUe, targetUe)
	if err != nil {
		t.Fatalf("AttachSourceUeTargetUe: %v", err)
	}

	targetRan.RanUEs[2] = targetUe

	amfInstance.Radios[new(sctp.SCTPConn)] = sourceRan
	amfInstance.Radios[new(sctp.SCTPConn)] = targetRan

	// Simulate the AMF UE being detached from the source (deregistration race).
	amfUe.DetachRanUe(nil)

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
