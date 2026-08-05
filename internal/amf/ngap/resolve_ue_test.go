// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/ngap"
)

// assertSingleErrorIndication checks that exactly one Error Indication was sent
// with the given radio-network cause, and returns it.
func assertSingleErrorIndication(t *testing.T, sender *fakeNGAPSender, wantCause int) *ngap.ErrorIndication {
	t.Helper()

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("ErrorIndications sent = %d, want 1", len(sender.SentErrorIndications))
	}

	errInd := sender.SentErrorIndications[0]

	want := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: wantCause}
	if errInd.Cause == nil || *errInd.Cause != want {
		t.Errorf("cause = %v, want radio-network cause %d", errInd.Cause, wantCause)
	}

	return errInd
}

// assertErrorIndicationEchoesIDs checks the Error Indication carries the received
// AP IDs (TS 38.413).
func assertErrorIndicationEchoesIDs(t *testing.T, errInd *ngap.ErrorIndication, wantAmf ngap.AMFUENGAPID, wantRan ngap.RANUENGAPID) {
	t.Helper()

	if errInd.AMFUENGAPID == nil || *errInd.AMFUENGAPID != wantAmf {
		t.Errorf("Error Indication AMF UE NGAP ID = %v, want %d", errInd.AMFUENGAPID, wantAmf)
	}

	if errInd.RANUENGAPID == nil || *errInd.RANUENGAPID != wantRan {
		t.Errorf("Error Indication RAN UE NGAP ID = %v, want %d", errInd.RANUENGAPID, wantRan)
	}
}

// setupCrossRadioScenario creates:
//   - legitimateRan: the radio the UE is actually registered on
//   - attackerRan: a different radio that will try to claim the UE
//   - ueConn: the UE context living on legitimateRan
//   - amfInstance: the AMF with both radios registered
func setupCrossRadioScenario(t *testing.T) (legitimateRan, attackerRan *amf.Radio, ueConn *amf.UeConn, amfInstance *amf.AMF) {
	t.Helper()

	amfInstance = newTestAMF()

	legitimateRan = newTestRadio(amfInstance)
	attackerRan = newTestRadio(amfInstance)

	amfInstance.SetRadioForTest(new(sctp.SCTPConn), legitimateRan)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), attackerRan)

	ueConn = amf.NewUeConnForTest(legitimateRan, 1, 10, logger.AmfLog)

	amfUe := amf.NewUeContext()
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	return legitimateRan, attackerRan, ueConn, amfInstance
}

// TestCrossRadio_PDUSessionResourceSetupResponse verifies that a rogue radio
// cannot claim a UE by sending a PDUSessionResourceSetupResponse with a valid
// AMF-UE-NGAP-ID that belongs to a UE on a different radio.
func TestCrossRadio_PDUSessionResourceSetupResponse(t *testing.T) {
	legitimateRan, attackerRan, ueConn, amfInstance := setupCrossRadioScenario(t)
	attackerSender := attackerRan.Conn.(*fakeNGAPSender)

	HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, attackerRan, &ngap.PDUSessionResourceSetupResponse{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(10)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(1)),
	})

	if len(attackerSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication on attacker radio, got %d", len(attackerSender.SentErrorIndications))
	}

	want := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnknownLocalUENGAPID}
	if cause := attackerSender.SentErrorIndications[0].Cause; cause == nil || *cause != want {
		t.Errorf("cause = %v, want unknown-local-UE-NGAP-ID", cause)
	}

	if ueConn.Radio() != legitimateRan {
		t.Error("UE radio association must not change")
	}
}

// TestCrossRadio_PDUSessionResourceModifyResponse verifies cross-radio
// rejection for PDUSessionResourceModifyResponse.
func TestCrossRadio_PDUSessionResourceModifyResponse(t *testing.T) {
	_, attackerRan, _, amfInstance := setupCrossRadioScenario(t)
	attackerSender := attackerRan.Conn.(*fakeNGAPSender)

	HandlePDUSessionResourceModifyResponse(context.Background(), amfInstance, attackerRan, &ngap.PDUSessionResourceModifyResponse{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(10)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(1)),
	})

	if len(attackerSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(attackerSender.SentErrorIndications))
	}
}

// TestCrossRadio_UEContextReleaseRequest verifies cross-radio
// rejection for UEContextReleaseRequest.
func TestCrossRadio_UEContextReleaseRequest(t *testing.T) {
	_, attackerRan, _, amfInstance := setupCrossRadioScenario(t)
	attackerSender := attackerRan.Conn.(*fakeNGAPSender)

	HandleUEContextReleaseRequest(context.Background(), amfInstance, attackerRan, &ngap.UEContextReleaseRequest{
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
	attackerSender := attackerRan.Conn.(*fakeNGAPSender)

	amfID := ngap.AMFUENGAPID(10)
	ranID := ngap.RANUENGAPID(1)
	HandleUEContextReleaseComplete(context.Background(), amfInstance, attackerRan, &ngap.UEContextReleaseComplete{
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
	attackerSender := attackerRan.Conn.(*fakeNGAPSender)

	amfID := ngap.AMFUENGAPID(10)
	ranID := ngap.RANUENGAPID(1)
	HandleHandoverRequestAcknowledge(context.Background(), amfInstance, attackerRan, &ngap.HandoverRequestAcknowledge{
		AMFUENGAPID:                        &amfID,
		RANUENGAPID:                        &ranID,
		TargetToSourceTransparentContainer: ngap.TargetToSourceTransparentContainer{0x01},
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
	attackerSender := attackerRan.Conn.(*fakeNGAPSender)

	HandleHandoverFailure(context.Background(), amfInstance, attackerRan, &ngap.HandoverFailure{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(10)),
	})

	if len(attackerSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(attackerSender.SentErrorIndications))
	}
}

// TestResolveUE_UnknownAmfUeNgapID verifies that an AMF UE NGAP ID the AMF never
// allocated is reported as an unknown local AP ID (TS 38.413), with the
// received AP IDs echoed back.
func TestResolveUE_UnknownAmfUeNgapID(t *testing.T) {
	legitimateRan, _, _, amfInstance := setupCrossRadioScenario(t)
	sender := legitimateRan.Conn.(*fakeNGAPSender)

	HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, legitimateRan, &ngap.PDUSessionResourceSetupResponse{
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(1)),
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(999)),
	})

	errInd := assertSingleErrorIndication(t, sender, ngap.CauseRadioNetworkUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 999, 1)
}

// TestResolveUE_InconsistentRanUeNgapID verifies that a known AMF UE NGAP ID with
// a RAN UE NGAP ID different from the stored one is reported as an inconsistent
// remote AP ID (TS 38.413), with the received AP IDs echoed back.
func TestResolveUE_InconsistentRanUeNgapID(t *testing.T) {
	legitimateRan, _, _, amfInstance := setupCrossRadioScenario(t)
	sender := legitimateRan.Conn.(*fakeNGAPSender)

	HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, legitimateRan, &ngap.PDUSessionResourceSetupResponse{
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(2)),
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(10)),
	})

	errInd := assertSingleErrorIndication(t, sender, ngap.CauseRadioNetworkInconsistentRemoteUEID)
	assertErrorIndicationEchoesIDs(t, errInd, 10, 2)
}
