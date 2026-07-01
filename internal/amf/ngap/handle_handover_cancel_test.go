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
	"github.com/free5gc/ngap/ngapType"
)

// TestHandleHandoverCancel_UnknownRanUeNgapID verifies that a HandoverCancel
// with a RAN_UE_NGAP_ID that doesn't match any existing UE context is handled
// gracefully — no panic, and an ErrorIndication is sent.
// Regression test.
func TestHandleHandoverCancel_UnknownRanUeNgapID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.NGAPSender.(*fakeNGAPSender)

	msg := decode.HandoverCancel{
		AMFUENGAPID: 1099511627776,
		RANUENGAPID: 99,
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem},
		},
	}

	ngap.HandleHandoverCancel(context.Background(), amfInstance, ran, msg)

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

// TestHandleHandoverCancel_UnknownAmfUeNgapID verifies that a HandoverCancel
// whose AMF UE NGAP ID the AMF never allocated is treated as an unknown local AP
// ID (TS 38.413): an Error Indication carrying the received AP IDs is sent,
// with no acknowledge to the source and no release toward the target.
func TestHandleHandoverCancel_UnknownAmfUeNgapID(t *testing.T) {
	amfInstance := newTestAMF()
	sourceRan := newTestRadio(amfInstance)
	sourceSender := sourceRan.NGAPSender.(*fakeNGAPSender)

	targetRan := newTestRadio(amfInstance)
	targetSender := targetRan.NGAPSender.(*fakeNGAPSender)

	amf.NewRanUeForTest(sourceRan, 1, 10, logger.AmfLog)

	msg := decode.HandoverCancel{
		AMFUENGAPID: 999, // does not match the source UE's AmfUeNgapID (10)
		RANUENGAPID: 1,
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem},
		},
	}

	ngap.HandleHandoverCancel(context.Background(), amfInstance, sourceRan, msg)

	errInd := assertSingleErrorIndication(t, sourceSender, ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID)
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
	sourceSender := sourceRan.NGAPSender.(*fakeNGAPSender)

	targetRan := newTestRadio(amfInstance)
	targetSender := targetRan.NGAPSender.(*fakeNGAPSender)

	sourceUe := amf.NewRanUeForTest(sourceRan, 1, 10, logger.AmfLog)
	targetUe := amf.NewRanUeForTest(targetRan, 2, 20, logger.AmfLog)

	amfUe := amf.NewUeContext()
	amfUe.Log = logger.AmfLog
	amfUe.AttachRanUe(sourceUe)

	if err := amf.AttachSourceUeTargetUe(sourceUe, targetUe); err != nil {
		t.Fatalf("AttachSourceUeTargetUe: %v", err)
	}

	msg := decode.HandoverCancel{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem},
		},
	}

	ngap.HandleHandoverCancel(context.Background(), amfInstance, sourceRan, msg)

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
