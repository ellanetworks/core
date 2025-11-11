package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestDecodeNGAPMessage_InitialContextSetupResponse(t *testing.T) {
	const message = "IA4ADwAAAgAKQAIAAgBVQAIAAg=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "SuccessfulOutcome" {
		t.Errorf("expected PDUType=SuccessfulOutcome, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.MessageType != "InitialContextSetupResponse" {
		t.Errorf("expected MessageType=InitialContextSetupResponse, got %v", ngapMsg.MessageType)
	}

	if ngapMsg.ProcedureCode.Label != "InitialContextSetup" {
		t.Errorf("expected ProcedureCode=InitialContextSetup, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != ngapType.ProcedureCodeInitialContextSetup {
		t.Errorf("expected ProcedureCode value=14, got %d", ngapMsg.ProcedureCode.Value)
	}

	if ngapMsg.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", ngapMsg.Criticality)
	}

	if ngapMsg.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", ngapMsg.Criticality.Value)
	}

	if len(ngapMsg.Value.IEs) != 2 {
		t.Errorf("expected 2 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Label != "AMFUENGAPID" {
		t.Errorf("expected ID=AMFUENGAPID, got %v", item0.ID)
	}

	if item0.ID.Value != ngapType.ProtocolIEIDAMFUENGAPID {
		t.Errorf("expected ID value=10, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item0.Criticality.Value)
	}

	amfUENGAPID, ok := item0.Value.(int64)
	if !ok {
		t.Fatalf("expected AMFUENGAPID to be of type int64, got %T", item0.Value)
	}

	if amfUENGAPID != 2 {
		t.Errorf("expected AMFUENGAPID=2, got %d", amfUENGAPID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Label != "RANUENGAPID" {
		t.Errorf("expected ID=RANUENGAPID, got %v", item1.ID)
	}

	if item1.ID.Value != ngapType.ProtocolIEIDRANUENGAPID {
		t.Errorf("expected ID value=85, got %d", item1.ID.Value)
	}

	if item1.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item1.Criticality)
	}

	if item1.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item1.Criticality.Value)
	}

	ranUENGAPID, ok := item1.Value.(int64)
	if !ok {
		t.Fatalf("expected RANUENGAPID to be of type int64, got %T", item1.Value)
	}

	if ranUENGAPID != 2 {
		t.Errorf("expected RANUENGAPID=2, got %d", ranUENGAPID)
	}
}
