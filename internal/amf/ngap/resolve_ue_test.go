// SPDX-FileCopyrightText: Ella Networks Inc.
//
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
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

// assertSingleErrorIndication checks that exactly one Error Indication was sent
// with the given radio-network cause, and returns it.
func assertSingleErrorIndication(t *testing.T, sender *FakeNGAPSender, wantCause aper.Enumerated) *ErrorIndication {
	t.Helper()

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("ErrorIndications sent = %d, want 1", len(sender.SentErrorIndications))
	}

	errInd := sender.SentErrorIndications[0]
	if errInd.Cause == nil || errInd.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Fatalf("expected RadioNetwork cause, got %+v", errInd.Cause)
	}

	if errInd.Cause.RadioNetwork.Value != wantCause {
		t.Errorf("cause = %d, want %d", errInd.Cause.RadioNetwork.Value, wantCause)
	}

	return errInd
}

// assertErrorIndicationEchoesIDs checks the Error Indication carries the received
// AP IDs (TS 38.413 §8.7.5.2, §10.6).
func assertErrorIndicationEchoesIDs(t *testing.T, errInd *ErrorIndication, wantAmf, wantRan int64) {
	t.Helper()

	if errInd.AmfUeNgapID == nil || *errInd.AmfUeNgapID != wantAmf {
		t.Errorf("Error Indication AMF UE NGAP ID = %v, want %d", errInd.AmfUeNgapID, wantAmf)
	}

	if errInd.RanUeNgapID == nil || *errInd.RanUeNgapID != wantRan {
		t.Errorf("Error Indication RAN UE NGAP ID = %v, want %d", errInd.RanUeNgapID, wantRan)
	}
}

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

	amfUe := amf.NewUeContext()
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

// TestResolveUE_UnknownAmfUeNgapID verifies that an AMF UE NGAP ID the AMF never
// allocated is reported as an unknown local AP ID (TS 38.413 §10.6), with the
// received AP IDs echoed back.
func TestResolveUE_UnknownAmfUeNgapID(t *testing.T) {
	legitimateRan, _, _, amfInstance := setupCrossRadioScenario(t)
	sender := legitimateRan.NGAPSender.(*FakeNGAPSender)

	ranID := int64(1)
	wrongAmfID := int64(999)
	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, legitimateRan, decode.PDUSessionResourceSetupResponse{
		RANUENGAPID: &ranID,
		AMFUENGAPID: &wrongAmfID,
	})

	errInd := assertSingleErrorIndication(t, sender, ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 999, 1)
}

// TestResolveUE_InconsistentRanUeNgapID verifies that a known AMF UE NGAP ID with
// a RAN UE NGAP ID different from the stored one is reported as an inconsistent
// remote AP ID (TS 38.413 §10.6), with the received AP IDs echoed back.
func TestResolveUE_InconsistentRanUeNgapID(t *testing.T) {
	legitimateRan, _, _, amfInstance := setupCrossRadioScenario(t)
	sender := legitimateRan.NGAPSender.(*FakeNGAPSender)

	amfID := int64(10)
	wrongRanID := int64(2)
	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, legitimateRan, decode.PDUSessionResourceSetupResponse{
		RANUENGAPID: &wrongRanID,
		AMFUENGAPID: &amfID,
	})

	errInd := assertSingleErrorIndication(t, sender, ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 10, 2)
}
