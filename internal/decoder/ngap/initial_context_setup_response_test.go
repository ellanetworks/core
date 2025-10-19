package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/omec-project/ngap/ngapType"
)

func TestDecodeNGAPMessage_InitialContextSetupResponse(t *testing.T) {
	const message = "IA4ADwAAAgAKQAIAAgBVQAIAAg=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngap, err := ngap.DecodeNGAPMessage(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngap.SuccessfulOutcome == nil {
		t.Fatalf("expected SuccessfulOutcome, got nil")
	}

	if ngap.SuccessfulOutcome.ProcedureCode.Label != "InitialContextSetup" {
		t.Errorf("expected ProcedureCode=InitialContextSetup, got %v", ngap.SuccessfulOutcome.ProcedureCode)
	}

	if ngap.SuccessfulOutcome.ProcedureCode.Value != int(ngapType.ProcedureCodeInitialContextSetup) {
		t.Errorf("expected ProcedureCode value=14, got %d", ngap.SuccessfulOutcome.ProcedureCode.Value)
	}

	if ngap.SuccessfulOutcome.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", ngap.SuccessfulOutcome.Criticality)
	}

	if ngap.SuccessfulOutcome.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", ngap.SuccessfulOutcome.Criticality.Value)
	}

	if ngap.SuccessfulOutcome.Value.InitialContextSetupResponse == nil {
		t.Fatalf("expected InitialContextSetupResponse, got nil")
	}

	if len(ngap.SuccessfulOutcome.Value.InitialContextSetupResponse.IEs) != 2 {
		t.Errorf("expected 2 ProtocolIEs, got %d", len(ngap.SuccessfulOutcome.Value.InitialContextSetupResponse.IEs))
	}

	item0 := ngap.SuccessfulOutcome.Value.InitialContextSetupResponse.IEs[0]

	if item0.ID != "AMFUENGAPID (10)" {
		t.Errorf("expected ID=AMFUENGAPID (10), got %s", item0.ID)
	}

	if item0.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item0.Criticality.Value)
	}

	if item0.AMFUENGAPID == nil {
		t.Fatalf("expected AMFUENGAPID, got nil")
	}

	if *item0.AMFUENGAPID != 2 {
		t.Errorf("expected AMFUENGAPID=2, got %d", *item0.AMFUENGAPID)
	}

	item1 := ngap.SuccessfulOutcome.Value.InitialContextSetupResponse.IEs[1]

	if item1.ID != "RANUENGAPID (85)" {
		t.Errorf("expected ID=RANUENGAPID (85), got %s", item1.ID)
	}

	if item1.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item1.Criticality)
	}

	if item1.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item1.Criticality.Value)
	}

	if item1.RANUENGAPID == nil {
		t.Fatalf("expected RANUENGAPID, got nil")
	}

	if *item1.RANUENGAPID != 2 {
		t.Errorf("expected RANUENGAPID=2, got %d", *item1.RANUENGAPID)
	}
}
