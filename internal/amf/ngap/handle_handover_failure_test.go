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
	amfInstance := newTestAMF()
	msg := decode.HandoverFailure{AMFUENGAPID: 1}

	assertNoPanic(t, "HandleHandoverFailure(missing cause)", func() {
		ngap.HandleHandoverFailure(context.Background(), amfInstance, ran, msg)
	})
}

// TestHandleHandoverFailure_SourceAmfUeDetached verifies that a handover
// failure is handled gracefully when the source UE's AMF UE context has been
// detached (e.g. due to a concurrent deregistration).
func TestHandleHandoverFailure_SourceAmfUeDetached(t *testing.T) {
	sourceRan := newTestRadio()
	targetRan := newTestRadio()
	amfInstance := newTestAMF()

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	sourceUe := &amf.RanUe{
		RanUeNgapID: 10,
		AmfUeNgapID: 100,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(sourceUe)
	sourceRan.RanUEs[10] = sourceUe

	targetUe := &amf.RanUe{
		RanUeNgapID: 2,
		AmfUeNgapID: 200,
		Radio:       targetRan,
		Log:         logger.AmfLog,
	}

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

	assertNoPanic(t, "HandleHandoverFailure(source AmfUe detached)", func() {
		ngap.HandleHandoverFailure(context.Background(), amfInstance, targetRan, msg)
	})
}
