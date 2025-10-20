package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/omec-project/ngap/ngapType"
)

func TestDecodeNGAPMessage_NGSetupFailure(t *testing.T) {
	const message = "QBUACAAAAQAPQAGI"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg, err := ngap.DecodeNGAPMessage(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngapMsg.UnsuccessfulOutcome == nil {
		t.Fatalf("expected UnsuccessfulOutcome, got nil")
	}

	if ngapMsg.UnsuccessfulOutcome.ProcedureCode.Label != "NGSetup" {
		t.Errorf("expected ProcedureCode=NGSetup, got %v", ngapMsg.UnsuccessfulOutcome.ProcedureCode)
	}

	if ngapMsg.UnsuccessfulOutcome.ProcedureCode.Value != int(ngapType.ProcedureCodeNGSetup) {
		t.Errorf("expected ProcedureCode value=1, got %d", ngapMsg.UnsuccessfulOutcome.ProcedureCode.Value)
	}

	if ngapMsg.UnsuccessfulOutcome.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", ngapMsg.UnsuccessfulOutcome.Criticality)
	}

	if ngapMsg.UnsuccessfulOutcome.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", ngapMsg.UnsuccessfulOutcome.Criticality.Value)
	}

	if ngapMsg.UnsuccessfulOutcome.Value.NGSetupFailure == nil {
		t.Fatalf("expected NGSetupFailure, got nil")
	}

	if len(ngapMsg.UnsuccessfulOutcome.Value.NGSetupFailure.IEs) != 1 {
		t.Errorf("expected 1 ProtocolIEs, got %d", len(ngapMsg.UnsuccessfulOutcome.Value.NGSetupFailure.IEs))
	}

	item0 := ngapMsg.UnsuccessfulOutcome.Value.NGSetupFailure.IEs[0]

	if item0.ID.Label != "Cause" {
		t.Errorf("expected ID=Cause, got %v", item0.ID)
	}

	if item0.ID.Value != int(ngapType.ProtocolIEIDCause) {
		t.Errorf("expected ID value=15, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item0.Criticality.Value)
	}

	cause, ok := item0.Value.(ngap.EnumField)
	if !ok {
		t.Fatalf("expected Cause, got %T", item0.Value)
	}

	if cause.Label != "UnknownPLMN" {
		t.Errorf("expected Cause=UnknownPLMN, got %v", cause.Label)
	}

	if cause.Value != int(ngapType.CauseMiscPresentUnknownPLMN) {
		t.Errorf("expected Cause value=%d, got %d", ngapType.CauseMiscPresentUnknownPLMN, cause.Value)
	}
}
