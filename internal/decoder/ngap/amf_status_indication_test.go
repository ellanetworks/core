package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestDecodeNGAPMessage_AMFStatusIndication(t *testing.T) {
	const message = "AAFADwAAAQB4AAgAAADxEMr+AA=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.MessageType != "AMFStatusIndication" {
		t.Errorf("expected MessageType=AMFStatusIndication, got %v", ngapMsg.MessageType)
	}

	if ngapMsg.ProcedureCode.Label != "AMFStatusIndication" {
		t.Errorf("expected ProcedureCode=AMFStatusIndication, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != ngapType.ProcedureCodeAMFStatusIndication {
		t.Errorf("expected ProcedureCode value=1, got %d", ngapMsg.ProcedureCode.Value)
	}

	if ngapMsg.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", ngapMsg.Criticality)
	}

	if ngapMsg.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", ngapMsg.Criticality.Value)
	}

	if len(ngapMsg.Value.IEs) != 1 {
		t.Errorf("expected 1 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Label != "UnavailableGUAMIList" {
		t.Errorf("expected ID=UnavailableGUAMIList, got %s", item0.ID.Label)
	}

	if item0.ID.Value != ngapType.ProtocolIEIDUnavailableGUAMIList {
		t.Errorf("expected ID value=120, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item0.Criticality.Value)
	}

	unavailableGuamiList, ok := item0.Value.([]ngap.Guami)
	if !ok {
		t.Fatalf("expected UnavailableGUAMIList type, got %T", item0.Value)
	}

	if len(unavailableGuamiList) != 1 {
		t.Errorf("expected 1 unavailable GUAMI item, got %d", len(unavailableGuamiList))
	}

	guami := unavailableGuamiList[0]

	if guami.PLMNID.Mcc != "001" {
		t.Errorf("expected MCC=001, got %s", guami.PLMNID.Mcc)
	}

	if guami.PLMNID.Mnc != "01" {
		t.Errorf("expected MNC=01, got %s", guami.PLMNID.Mnc)
	}

	if guami.AMFID != "cafe00" {
		t.Errorf("expected AMFID=cafe00, got %s", guami.AMFID)
	}
}
