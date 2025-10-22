package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/omec-project/ngap/ngapType"
)

func TestDecodeNGAPMessage_NGSetupFailure(t *testing.T) {
	const message = "QBUACAAAAQAPQAGI"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "UnsuccessfulOutcome" {
		t.Errorf("expected PDUType=UnsuccessfulOutcome, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.MessageType != "NGSetupFailure" {
		t.Errorf("expected MessageType=NGSetupFailure, got %v", ngapMsg.MessageType)
	}

	if ngapMsg.ProcedureCode.Label != "NGSetup" {
		t.Errorf("expected ProcedureCode=NGSetup, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != ngapType.ProcedureCodeNGSetup {
		t.Errorf("expected ProcedureCode value=1, got %d", ngapMsg.ProcedureCode.Value)
	}

	if ngapMsg.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", ngapMsg.Criticality)
	}

	if ngapMsg.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", ngapMsg.Criticality.Value)
	}

	if len(ngapMsg.Value.IEs) != 1 {
		t.Errorf("expected 1 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Label != "Cause" {
		t.Errorf("expected ID=Cause, got %v", item0.ID)
	}

	if item0.ID.Value != ngapType.ProtocolIEIDCause {
		t.Errorf("expected ID value=15, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item0.Criticality.Value)
	}

	cause, ok := item0.Value.(utils.EnumField[uint64])
	if !ok {
		t.Fatalf("expected Cause, got %T", item0.Value)
	}

	if cause.Label != "UnknownPLMN" {
		t.Errorf("expected Cause=UnknownPLMN, got %v", cause.Label)
	}

	if cause.Value != uint64(ngapType.CauseMiscPresentUnknownPLMN) {
		t.Errorf("expected Cause value=%d, got %d", ngapType.CauseMiscPresentUnknownPLMN, cause.Value)
	}
}
