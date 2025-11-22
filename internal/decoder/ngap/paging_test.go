package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestDecodeNGAPMessage_Paging(t *testing.T) {
	const message = "ABhAGQAAAgBzQAcfwAAAAAABAGdABwAA8RAAAAE="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.MessageType != "Paging" {
		t.Errorf("expected MessageType=Paging, got %v", ngapMsg.MessageType)
	}

	if ngapMsg.ProcedureCode.Label != "Paging" {
		t.Errorf("expected ProcedureCode=Paging, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != ngapType.ProcedureCodePaging {
		t.Errorf("expected ProcedureCode value=24, got %d", ngapMsg.ProcedureCode.Value)
	}

	if ngapMsg.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", ngapMsg.Criticality)
	}

	if ngapMsg.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", ngapMsg.Criticality.Value)
	}

	if len(ngapMsg.Value.IEs) != 2 {
		t.Errorf("expected 2 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Label != "UEPagingIdentity" {
		t.Errorf("expected ID=UEPagingIdentity, got %s", item0.ID.Label)
	}

	if item0.ID.Value != ngapType.ProtocolIEIDUEPagingIdentity {
		t.Errorf("expected ID value=53, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item0.Criticality.Value)
	}

	pagingID, ok := item0.Value.(ngap.UEPagingIdentity)
	if !ok {
		t.Fatalf("expected UEPagingIdentity type, got %T", item0.Value)
	}

	if pagingID.FiveGSTMSI.AMFSetID != "fe0" {
		t.Errorf("expected FiveGSType=fe0, got %s", pagingID.FiveGSTMSI.AMFSetID)
	}

	if pagingID.FiveGSTMSI.AMFPointer != "00" {
		t.Errorf("expected AMFPointer=00, got %v", pagingID.FiveGSTMSI.AMFPointer)
	}

	if pagingID.FiveGSTMSI.FiveGTMSI != "00000001" {
		t.Errorf("expected TMSI=00000001, got %s", pagingID.FiveGSTMSI.FiveGTMSI)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Label != "TAIListForPaging" {
		t.Errorf("expected ID=TAIListForPaging, got %s", item1.ID.Label)
	}

	if item1.ID.Value != ngapType.ProtocolIEIDTAIListForPaging {
		t.Errorf("expected ID value=54, got %d", item1.ID.Value)
	}

	if item1.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item1.Criticality)
	}

	if item1.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item1.Criticality.Value)
	}

	_, ok = item1.Value.([]ngap.TAI)
	if !ok {
		t.Fatalf("expected TAIListForPaging type, got %T", item1.Value)
	}

	if len(item1.Value.([]ngap.TAI)) != 1 {
		t.Errorf("expected 1 TAI, got %d", len(item1.Value.([]ngap.TAI)))
	}

	if item1.Value.([]ngap.TAI)[0].PLMNID.Mcc != "001" {
		t.Errorf("expected MCC=001, got %s", item1.Value.([]ngap.TAI)[0].PLMNID.Mcc)
	}

	if item1.Value.([]ngap.TAI)[0].PLMNID.Mnc != "01" {
		t.Errorf("expected MNC=01, got %s", item1.Value.([]ngap.TAI)[0].PLMNID.Mnc)
	}

	if item1.Value.([]ngap.TAI)[0].TAC != "000001" {
		t.Errorf("expected TAC=000001, got %s", item1.Value.([]ngap.TAI)[0].TAC)
	}
}
