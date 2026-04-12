// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

// TestHandleHandoverCancel_UnknownRanUeNgapID verifies that a HandoverCancel
// with a RAN_UE_NGAP_ID that doesn't match any existing UE context is handled
// gracefully — no panic, and an ErrorIndication is sent.
// Regression test inspired by open5gs/open5gs#4378.
func TestHandleHandoverCancel_UnknownRanUeNgapID(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	msg := decode.HandoverCancel{
		AMFUENGAPID: 1099511627776,
		RANUENGAPID: 99,
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem},
		},
	}

	ngap.HandleHandoverCancel(context.Background(), ran, msg)

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}

	errInd := sender.SentErrorIndications[0]
	if errInd.Cause == nil || errInd.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Fatal("expected RadioNetwork cause in ErrorIndication")
	}

	if errInd.Cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID {
		t.Fatalf("expected UnknownLocalUENGAPID, got %d", errInd.Cause.RadioNetwork.Value)
	}
}

func TestHandleHandoverCancel_HappyPath(t *testing.T) {
	sourceRan := newTestRadio()
	sourceSender := sourceRan.NGAPSender.(*FakeNGAPSender)

	targetRan := newTestRadio()
	targetSender := targetRan.NGAPSender.(*FakeNGAPSender)

	sourceUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}

	targetUe := &amf.RanUe{
		RanUeNgapID: 2,
		AmfUeNgapID: 20,
		Radio:       targetRan,
		Log:         logger.AmfLog,
	}

	sourceUe.TargetUe = targetUe
	targetUe.SourceUe = sourceUe
	sourceRan.RanUEs[1] = sourceUe

	msg := decode.HandoverCancel{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem},
		},
	}

	ngap.HandleHandoverCancel(context.Background(), sourceRan, msg)

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
	if ack.AmfUeNgapID != 10 || ack.RanUeNgapID != 1 {
		t.Errorf("HandoverCancelAcknowledge IDs = (%d, %d), want (10, 1)", ack.AmfUeNgapID, ack.RanUeNgapID)
	}
}
