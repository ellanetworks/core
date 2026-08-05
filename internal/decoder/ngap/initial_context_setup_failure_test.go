// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	lib "github.com/ellanetworks/core/ngap"
)

// Captured InitialContextSetupFailure PDU with cause radioNetwork=SliceNotSupported (39).
// Hex dump:
//
//	40 0e 00 15 00 00 03 00 0a 40 02 00 09 00 55 40
//	02 00 09 00 0f 40 02 09 c0
func TestDecodeNGAPMessage_InitialContextSetupFailure(t *testing.T) {
	const message = "QA4AFQAAAwAKQAIACQBVQAIACQAPQAIJwA=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "UnsuccessfulOutcome" {
		t.Errorf("expected PDUType=UnsuccessfulOutcome, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Label != "InitialContextSetup" {
		t.Errorf("expected ProcedureCode=InitialContextSetup, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcInitialContextSetup) {
		t.Errorf("expected ProcedureCode value=%d, got %d", lib.ProcInitialContextSetup, ngapMsg.ProcedureCode.Value)
	}

	if ngapMsg.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", ngapMsg.Criticality)
	}

	if ngapMsg.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", ngapMsg.Criticality.Value)
	}

	if ngapMsg.Value.Error != "" {
		t.Fatalf("expected no decode error, got %q", ngapMsg.Value.Error)
	}

	if len(ngapMsg.Value.IEs) != 3 {
		t.Fatalf("expected 3 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	amfUEID := ngapMsg.Value.IEs[0]
	if amfUEID.ID.Value != int64(idAMFUENGAPID) {
		t.Errorf("expected first IE=AMF-UE-NGAP-ID (%d), got %d", idAMFUENGAPID, amfUEID.ID.Value)
	}

	if v, ok := amfUEID.Value.(int64); !ok || v != 9 {
		t.Errorf("expected AMF-UE-NGAP-ID=9, got %T %v", amfUEID.Value, amfUEID.Value)
	}

	ranUEID := ngapMsg.Value.IEs[1]
	if ranUEID.ID.Value != int64(idRANUENGAPID) {
		t.Errorf("expected second IE=RAN-UE-NGAP-ID (%d), got %d", idRANUENGAPID, ranUEID.ID.Value)
	}

	if v, ok := ranUEID.Value.(int64); !ok || v != 9 {
		t.Errorf("expected RAN-UE-NGAP-ID=9, got %T %v", ranUEID.Value, ranUEID.Value)
	}

	causeIE := ngapMsg.Value.IEs[2]
	if causeIE.ID.Label != "Cause" {
		t.Errorf("expected third IE=Cause, got %v", causeIE.ID)
	}

	cause, ok := causeIE.Value.(Cause)
	if !ok {
		t.Fatalf("expected Cause, got %T", causeIE.Value)
	}

	if cause.Value.Label != "slice-not-supported" {
		t.Errorf("expected Cause=slice-not-supported, got %v", cause.Value.Label)
	}

	if cause.Value.Value != int64(lib.CauseRadioNetworkSliceNotSupported) {
		t.Errorf("expected Cause value=%d, got %d", lib.CauseRadioNetworkSliceNotSupported, cause.Value.Value)
	}
}
