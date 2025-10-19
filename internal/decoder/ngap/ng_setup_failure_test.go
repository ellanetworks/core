package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
)

func TestDecodeNGAPMessage_NGSetupFailure(t *testing.T) {
	const message = "QBUACAAAAQAPQAGI"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngap, err := ngap.DecodeNGAPMessage(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngap.UnsuccessfulOutcome == nil {
		t.Fatalf("expected UnsuccessfulOutcome, got nil")
	}

	if ngap.UnsuccessfulOutcome.ProcedureCode != "NGSetup" {
		t.Errorf("expected ProcedureCode=NGSetup, got %s", ngap.UnsuccessfulOutcome.ProcedureCode)
	}

	if ngap.UnsuccessfulOutcome.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", ngap.UnsuccessfulOutcome.Criticality)
	}

	if ngap.UnsuccessfulOutcome.Value.NGSetupFailure == nil {
		t.Fatalf("expected NGSetupFailure, got nil")
	}

	if len(ngap.UnsuccessfulOutcome.Value.NGSetupFailure.IEs) != 1 {
		t.Errorf("expected 1 ProtocolIEs, got %d", len(ngap.UnsuccessfulOutcome.Value.NGSetupFailure.IEs))
	}

	item0 := ngap.UnsuccessfulOutcome.Value.NGSetupFailure.IEs[0]

	if item0.ID != "Cause (15)" {
		t.Errorf("expected ID=Cause (15), got %s", item0.ID)
	}

	if item0.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item0.Criticality)
	}

	if item0.Cause == nil {
		t.Fatalf("expected Cause, got nil")
	}

	if *item0.Cause != "UnknownPLMN (4)" {
		t.Errorf("expected Cause=UnknownPLMN (4), got %s", *item0.Cause)
	}
}
