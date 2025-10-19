package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/omec-project/ngap/ngapType"
)

func TestDecodeNGAPMessage_NGSetupRequest(t *testing.T) {
	const message = "ABUAQQAABAAbAAkAAPEQUAAAAAEAUkAUCIBVRVJBTlNJTS1nbmItMS0xLTEAZgAQAAAAAAEAAPEQAAAQCBAgMAAVQAFA"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngap, err := ngap.DecodeNGAPMessage(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngap.InitiatingMessage == nil {
		t.Fatalf("expected InitiatingMessage, got nil")
	}

	if ngap.InitiatingMessage.ProcedureCode.Label != "NGSetup" {
		t.Errorf("expected ProcedureCode=NGSetup, got %v", ngap.InitiatingMessage.ProcedureCode)
	}

	if ngap.InitiatingMessage.ProcedureCode.Value != int(ngapType.ProcedureCodeNGSetup) {
		t.Errorf("expected ProcedureCode value=1, got %d", ngap.InitiatingMessage.ProcedureCode.Value)
	}

	if ngap.InitiatingMessage.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", ngap.InitiatingMessage.Criticality)
	}

	if ngap.InitiatingMessage.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", ngap.InitiatingMessage.Criticality.Value)
	}

	if ngap.InitiatingMessage.Value.NGSetupRequest == nil {
		t.Fatalf("expected NGSetupRequest, got nil")
	}

	if len(ngap.InitiatingMessage.Value.NGSetupRequest.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngap.InitiatingMessage.Value.NGSetupRequest.IEs))
	}

	item0 := ngap.InitiatingMessage.Value.NGSetupRequest.IEs[0]

	if item0.ID != "GlobalRANNodeID (27)" {
		t.Errorf("expected ID=GlobalRANNodeID (27), got %s", item0.ID)
	}

	if item0.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item0.Criticality.Value)
	}

	if item0.GlobalRANNodeID == nil {
		t.Fatalf("expected GlobalRANNodeID, got nil")
	}

	if item0.GlobalRANNodeID.GlobalGNBID != "00000001" {
		t.Errorf("expected GlobalGNBID=00000001, got %s", item0.GlobalRANNodeID.GlobalGNBID)
	}

	if item0.GlobalRANNodeID.GlobalNgENBID != "" {
		t.Errorf("expected empty globalNgENBID, got %s", item0.GlobalRANNodeID.GlobalNgENBID)
	}

	if item0.GlobalRANNodeID.GlobalN3IWFID != "" {
		t.Errorf("expected empty GlobalN3IWFID, got %s", item0.GlobalRANNodeID.GlobalN3IWFID)
	}

	item1 := ngap.InitiatingMessage.Value.NGSetupRequest.IEs[1]

	if item1.ID != "RANNodeName (82)" {
		t.Errorf("expected ID=RANNodeName (82), got %s", item1.ID)
	}

	if item1.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item1.Criticality)
	}

	if item1.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item1.Criticality.Value)
	}

	if item1.RANNodeName == nil {
		t.Fatalf("expected RANNodeName, got nil")
	}

	if *item1.RANNodeName != "UERANSIM-gnb-1-1-1" {
		t.Errorf("expected RANNodeName=UERANSIM-gnb-1-1-1, got %s", *item1.RANNodeName)
	}

	item2 := ngap.InitiatingMessage.Value.NGSetupRequest.IEs[2]

	if item2.ID != "SupportedTAList (102)" {
		t.Errorf("expected ID=SupportedTAList (102), got %s", item2.ID)
	}

	if item2.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item2.Criticality)
	}

	if item2.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item2.Criticality.Value)
	}

	if item2.SupportedTAList == nil {
		t.Fatalf("expected SupportedTAList, got nil")
	}

	if len(item2.SupportedTAList) != 1 {
		t.Fatalf("expected 1 SupportedTAItem, got %d", len(item2.SupportedTAList))
	}

	supportedTAItem := item2.SupportedTAList[0]

	if supportedTAItem.TAC != "000001" {
		t.Errorf("expected TAC=000001, got %s", supportedTAItem.TAC)
	}

	if len(supportedTAItem.BroadcastPLMNList) != 1 {
		t.Fatalf("expected 1 BroadcastPLMN, got %d", len(supportedTAItem.BroadcastPLMNList))
	}

	if supportedTAItem.BroadcastPLMNList[0].PLMNID.Mcc != "001" {
		t.Errorf("expected PLMNID.Mcc=001, got %s", supportedTAItem.BroadcastPLMNList[0].PLMNID.Mcc)
	}

	if supportedTAItem.BroadcastPLMNList[0].PLMNID.Mnc != "01" {
		t.Errorf("expected PLMNID.Mnc=01, got %s", supportedTAItem.BroadcastPLMNList[0].PLMNID.Mnc)
	}

	if len(supportedTAItem.BroadcastPLMNList[0].SliceSupportList) != 1 {
		t.Fatalf("expected 1 SNSSAI, got %d", len(supportedTAItem.BroadcastPLMNList[0].SliceSupportList))
	}

	snssai := supportedTAItem.BroadcastPLMNList[0].SliceSupportList[0]

	if snssai.SST != 1 {
		t.Errorf("expected SST=1, got %d", snssai.SST)
	}

	if snssai.SD == nil || *snssai.SD != "102030" {
		t.Errorf("expected SD=%s, got %v", "102030", snssai.SD)
	}

	item3 := ngap.InitiatingMessage.Value.NGSetupRequest.IEs[3]

	if item3.ID != "DefaultPagingDRX (21)" {
		t.Errorf("expected ID=DefaultPagingDRX (21), got %s", item3.ID)
	}

	if item3.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item3.Criticality)
	}

	if item3.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item3.Criticality.Value)
	}

	if item3.DefaultPagingDRX == nil {
		t.Fatalf("expected DefaultPagingDRX, got nil")
	}

	expectedDRX := "v128"
	if *item3.DefaultPagingDRX != expectedDRX {
		t.Errorf("expected DefaultPagingDRX=%s, got %s", expectedDRX, *item3.DefaultPagingDRX)
	}
}
