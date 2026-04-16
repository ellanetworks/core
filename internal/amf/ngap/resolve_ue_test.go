// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

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

// setupCrossRadioScenario creates:
//   - legitimateRan: the radio the UE is actually registered on
//   - attackerRan: a different radio that will try to claim the UE
//   - ranUe: the UE context living on legitimateRan
//   - amfInstance: the AMF with both radios registered
func setupCrossRadioScenario(t *testing.T) (legitimateRan, attackerRan *amf.Radio, ranUe *amf.RanUe, amfInstance *amf.AMF) {
	t.Helper()

	legitimateRan = newTestRadio()
	attackerRan = newTestRadio()

	amfInstance = newTestAMF()
	amfInstance.Radios[new(sctp.SCTPConn)] = legitimateRan
	amfInstance.Radios[new(sctp.SCTPConn)] = attackerRan

	ranUe = amf.NewRanUeForTest(legitimateRan, 1, 10, logger.AmfLog)

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog
	amfUe.AttachRanUe(ranUe)

	return legitimateRan, attackerRan, ranUe, amfInstance
}

// TestCrossRadio_PDUSessionResourceSetupResponse verifies that a rogue radio
// cannot claim a UE by sending a PDUSessionResourceSetupResponse with a valid
// AMF-UE-NGAP-ID that belongs to a UE on a different radio.
func TestCrossRadio_PDUSessionResourceSetupResponse(t *testing.T) {
	legitimateRan, attackerRan, ranUe, amfInstance := setupCrossRadioScenario(t)
	attackerSender := attackerRan.NGAPSender.(*FakeNGAPSender)

	amfID := int64(10)
	ranID := int64(1)
	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, attackerRan, decode.PDUSessionResourceSetupResponse{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
	})

	if len(attackerSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication on attacker radio, got %d", len(attackerSender.SentErrorIndications))
	}

	if attackerSender.SentErrorIndications[0].Cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID {
		t.Errorf("expected UnknownLocalUENGAPID cause, got %d", attackerSender.SentErrorIndications[0].Cause.RadioNetwork.Value)
	}

	if ranUe.Radio() != legitimateRan {
		t.Error("UE radio association must not change")
	}
}

// TestCrossRadio_PDUSessionResourceModifyResponse verifies cross-radio
// rejection for PDUSessionResourceModifyResponse.
func TestCrossRadio_PDUSessionResourceModifyResponse(t *testing.T) {
	_, attackerRan, _, amfInstance := setupCrossRadioScenario(t)
	attackerSender := attackerRan.NGAPSender.(*FakeNGAPSender)

	amfID := int64(10)
	ngap.HandlePDUSessionResourceModifyResponse(context.Background(), amfInstance, attackerRan, decode.PDUSessionResourceModifyResponse{
		AMFUENGAPID: &amfID,
	})

	if len(attackerSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(attackerSender.SentErrorIndications))
	}
}

// TestCrossRadio_UEContextModificationResponse verifies cross-radio
// rejection for UEContextModificationResponse.
func TestCrossRadio_UEContextModificationResponse(t *testing.T) {
	_, attackerRan, _, amfInstance := setupCrossRadioScenario(t)
	attackerSender := attackerRan.NGAPSender.(*FakeNGAPSender)

	amfID := int64(10)
	ngap.HandleUEContextModificationResponse(context.Background(), amfInstance, attackerRan, decode.UEContextModificationResponse{
		AMFUENGAPID: &amfID,
	})

	if len(attackerSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(attackerSender.SentErrorIndications))
	}
}

// TestCrossRadio_UEContextModificationFailure verifies cross-radio
// rejection for UEContextModificationFailure.
func TestCrossRadio_UEContextModificationFailure(t *testing.T) {
	_, attackerRan, _, amfInstance := setupCrossRadioScenario(t)
	attackerSender := attackerRan.NGAPSender.(*FakeNGAPSender)

	amfID := int64(10)
	ngap.HandleUEContextModificationFailure(context.Background(), amfInstance, attackerRan, decode.UEContextModificationFailure{
		AMFUENGAPID: &amfID,
	})

	if len(attackerSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(attackerSender.SentErrorIndications))
	}
}

// TestCrossRadio_UEContextReleaseRequest verifies cross-radio
// rejection for UEContextReleaseRequest.
func TestCrossRadio_UEContextReleaseRequest(t *testing.T) {
	_, attackerRan, _, amfInstance := setupCrossRadioScenario(t)
	attackerSender := attackerRan.NGAPSender.(*FakeNGAPSender)

	ngap.HandleUEContextReleaseRequest(context.Background(), amfInstance, attackerRan, decode.UEContextReleaseRequest{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
	})

	if len(attackerSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(attackerSender.SentErrorIndications))
	}

	if len(attackerSender.SentUEContextReleaseCommands) != 0 {
		t.Error("attacker radio must not receive UEContextReleaseCommand for victim UE")
	}
}

// TestCrossRadio_UEContextReleaseComplete verifies cross-radio
// rejection for UEContextReleaseComplete.
func TestCrossRadio_UEContextReleaseComplete(t *testing.T) {
	_, attackerRan, _, amfInstance := setupCrossRadioScenario(t)
	attackerSender := attackerRan.NGAPSender.(*FakeNGAPSender)

	amfID := int64(10)
	ranID := int64(1)
	ngap.HandleUEContextReleaseComplete(context.Background(), amfInstance, attackerRan, decode.UEContextReleaseComplete{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
	})

	if len(attackerSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(attackerSender.SentErrorIndications))
	}
}

// TestCrossRadio_HandoverRequestAcknowledge verifies cross-radio
// rejection for HandoverRequestAcknowledge.
func TestCrossRadio_HandoverRequestAcknowledge(t *testing.T) {
	_, attackerRan, _, amfInstance := setupCrossRadioScenario(t)
	attackerSender := attackerRan.NGAPSender.(*FakeNGAPSender)

	amfID := int64(10)
	ranID := int64(1)
	ngap.HandleHandoverRequestAcknowledge(context.Background(), amfInstance, attackerRan, decode.HandoverRequestAcknowledge{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
		TargetToSourceTransparentContainer: ngapType.TargetToSourceTransparentContainer{
			Value: []byte{0x01},
		},
	})

	if len(attackerSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(attackerSender.SentErrorIndications))
	}

	if len(attackerSender.SentHandoverCommands) != 0 {
		t.Error("attacker radio must not receive HandoverCommand")
	}
}

// TestCrossRadio_HandoverFailure verifies cross-radio
// rejection for HandoverFailure.
func TestCrossRadio_HandoverFailure(t *testing.T) {
	_, attackerRan, _, amfInstance := setupCrossRadioScenario(t)
	attackerSender := attackerRan.NGAPSender.(*FakeNGAPSender)

	ngap.HandleHandoverFailure(context.Background(), amfInstance, attackerRan, decode.HandoverFailure{
		AMFUENGAPID: 10,
	})

	if len(attackerSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(attackerSender.SentErrorIndications))
	}
}

// TestCrossRadio_InconsistentAmfUeNgapID verifies that when the RAN-UE-NGAP-ID
// matches but the AMF-UE-NGAP-ID does not, an InconsistentRemoteUENGAPID error
// is sent back.
func TestCrossRadio_InconsistentAmfUeNgapID(t *testing.T) {
	legitimateRan, _, _, amfInstance := setupCrossRadioScenario(t)
	sender := legitimateRan.NGAPSender.(*FakeNGAPSender)

	// RAN-UE-NGAP-ID=1 exists on legitimateRan with AMF-UE-NGAP-ID=10,
	// but we claim AMF-UE-NGAP-ID=999.
	ranID := int64(1)
	wrongAmfID := int64(999)
	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, legitimateRan, decode.PDUSessionResourceSetupResponse{
		RANUENGAPID: &ranID,
		AMFUENGAPID: &wrongAmfID,
	})

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}

	if sender.SentErrorIndications[0].Cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID {
		t.Errorf("expected InconsistentRemoteUENGAPID, got %d", sender.SentErrorIndications[0].Cause.RadioNetwork.Value)
	}
}
