package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/omec-project/ngap/ngapType"
)

func TestDecodeNGAPMessage_UEContextReleaseCommand(t *testing.T) {
	const message = "ACkAEQAAAgByAAQAGgAaAA9AAgUA"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Label != "UEContextRelease" {
		t.Errorf("expected ProcedureCode=UEContextRelease, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != ngapType.ProcedureCodeUEContextRelease {
		t.Errorf("expected ProcedureCode value=41, got %d", ngapMsg.ProcedureCode.Value)
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

	if item0.ID.Label != "UENGAPIDs" {
		t.Errorf("expected ID=UENGAPIDs, got %s", item0.ID.Label)
	}

	if item0.ID.Value != ngapType.ProtocolIEIDUENGAPIDs {
		t.Errorf("expected ID value=114, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item0.Criticality.Value)
	}

	ueNgapIDs, ok := item0.Value.(ngap.UENGAPIDs)
	if !ok {
		t.Fatalf("expected UENGAPIDs value type=ngap.UENGAPIDs, got %T", item0.Value)
	}

	if ueNgapIDs.AMFUENGAPID != 0 {
		t.Errorf("expected AMFUENGAPID=0, got %d", ueNgapIDs.AMFUENGAPID)
	}

	if ueNgapIDs.UENGAPIDPair.AMFUENGAPID != 26 {
		t.Errorf("expected UENGAPIDPair.AMFUENGAPID=26, got %d", ueNgapIDs.UENGAPIDPair.AMFUENGAPID)
	}

	if ueNgapIDs.UENGAPIDPair.RANUENGAPID != 26 {
		t.Errorf("expected UENGAPIDPair.RANUENGAPID=26, got %d", ueNgapIDs.UENGAPIDPair.RANUENGAPID)
	}
}
