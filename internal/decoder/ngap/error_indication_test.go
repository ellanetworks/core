// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/free5gc/ngap/ngapType"
)

// Captured from a live deployment: an Error Indication naming the UE-NGAP-ID
// pair the receiver could not resolve.
const errorIndicationUnknownLocalUEID = "AAlAFQAAAwAKQAIAAgBVQAIAXAAPQAIDgA=="

func TestDecodeNGAPMessage_ErrorIndication(t *testing.T) {
	raw, err := decodeB64(errorIndicationUnknownLocalUEID)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	if ngapMsg.Value.Error != "" {
		t.Fatalf("expected no decode error, got %q", ngapMsg.Value.Error)
	}

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Label != "ErrorIndication" {
		t.Errorf("expected ProcedureCode=ErrorIndication, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != ngapType.ProcedureCodeErrorIndication {
		t.Errorf("expected ProcedureCode value=%d, got %d", ngapType.ProcedureCodeErrorIndication, ngapMsg.ProcedureCode.Value)
	}

	// Error Indication is ignore at both the message and the IE level
	// (TS 38.413 §9.2.6.4).
	if ngapMsg.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", ngapMsg.Criticality)
	}

	if len(ngapMsg.Value.IEs) != 3 {
		t.Fatalf("expected 3 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Label != "AMFUENGAPID" {
		t.Errorf("expected ID=AMFUENGAPID, got %s", item0.ID.Label)
	}

	if item0.ID.Value != ngapType.ProtocolIEIDAMFUENGAPID {
		t.Errorf("expected ID value=%d, got %d", ngapType.ProtocolIEIDAMFUENGAPID, item0.ID.Value)
	}

	amfUeNgapID, ok := item0.Value.(int64)
	if !ok {
		t.Fatalf("expected AMFUENGAPID value type=int64, got %T", item0.Value)
	}

	if amfUeNgapID != 2 {
		t.Errorf("expected AMFUENGAPID=2, got %d", amfUeNgapID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Label != "RANUENGAPID" {
		t.Errorf("expected ID=RANUENGAPID, got %s", item1.ID.Label)
	}

	if item1.ID.Value != ngapType.ProtocolIEIDRANUENGAPID {
		t.Errorf("expected ID value=%d, got %d", ngapType.ProtocolIEIDRANUENGAPID, item1.ID.Value)
	}

	ranUeNgapID, ok := item1.Value.(int64)
	if !ok {
		t.Fatalf("expected RANUENGAPID value type=int64, got %T", item1.Value)
	}

	if ranUeNgapID != 92 {
		t.Errorf("expected RANUENGAPID=92, got %d", ranUeNgapID)
	}

	// The Cause is the whole point of the message: without it the event says a
	// failure happened and nothing about why.
	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Label != "Cause" {
		t.Errorf("expected ID=Cause, got %s", item2.ID.Label)
	}

	if item2.ID.Value != ngapType.ProtocolIEIDCause {
		t.Errorf("expected ID value=%d, got %d", ngapType.ProtocolIEIDCause, item2.ID.Value)
	}

	cause, ok := item2.Value.(utils.EnumField[uint64])
	if !ok {
		t.Fatalf("expected Cause value type=utils.EnumField[uint64], got %T", item2.Value)
	}

	if cause.Label != "UnknownLocalUENGAPID" {
		t.Errorf("expected Cause=UnknownLocalUENGAPID, got %s", cause.Label)
	}

	if cause.Value != uint64(ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID) {
		t.Errorf("expected Cause value=%d, got %d", ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID, cause.Value)
	}

	if cause.Unknown {
		t.Errorf("expected Cause to be a known enum value")
	}
}

func TestDecodeNGAPMessage_ErrorIndicationSummary(t *testing.T) {
	raw, err := decodeB64(errorIndicationUnknownLocalUEID)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	expected := "ErrorIndication, AMF-UE=2, RAN-UE=92, Cause=UnknownLocalUENGAPID"
	if ngapMsg.Summary != expected {
		t.Errorf("expected Summary=%q, got %q", expected, ngapMsg.Summary)
	}
}
