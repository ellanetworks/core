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

	ngapMsg, err := ngap.DecodeNGAPMessage(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngapMsg.InitiatingMessage == nil {
		t.Fatalf("expected InitiatingMessage, got nil")
	}

	if ngapMsg.InitiatingMessage.ProcedureCode.Label != "NGSetup" {
		t.Errorf("expected ProcedureCode=NGSetup, got %v", ngapMsg.InitiatingMessage.ProcedureCode)
	}

	if ngapMsg.InitiatingMessage.ProcedureCode.Value != int(ngapType.ProcedureCodeNGSetup) {
		t.Errorf("expected ProcedureCode value=1, got %d", ngapMsg.InitiatingMessage.ProcedureCode.Value)
	}

	if ngapMsg.InitiatingMessage.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", ngapMsg.InitiatingMessage.Criticality)
	}

	if ngapMsg.InitiatingMessage.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", ngapMsg.InitiatingMessage.Criticality.Value)
	}

	if ngapMsg.InitiatingMessage.Value.NGSetupRequest == nil {
		t.Fatalf("expected NGSetupRequest, got nil")
	}

	if len(ngapMsg.InitiatingMessage.Value.NGSetupRequest.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngapMsg.InitiatingMessage.Value.NGSetupRequest.IEs))
	}

	item0 := ngapMsg.InitiatingMessage.Value.NGSetupRequest.IEs[0]

	if item0.ID.Label != "GlobalRANNodeID" {
		t.Errorf("expected ID=GlobalRANNodeID, got %v", item0.ID)
	}

	if item0.ID.Value != int(ngapType.ProtocolIEIDGlobalRANNodeID) {
		t.Errorf("expected ID value=27, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item0.Criticality.Value)
	}

	globalRANNodeID, ok := item0.Value.(*ngap.GlobalRANNodeIDIE)
	if !ok {
		t.Fatalf("expected GlobalRANNodeIDIE, got %T", item0.Value)
	}

	if globalRANNodeID.GlobalGNBID != "00000001" {
		t.Errorf("expected GlobalGNBID=00000001, got %s", globalRANNodeID.GlobalGNBID)
	}

	if globalRANNodeID.GlobalNgENBID != "" {
		t.Errorf("expected empty globalNgENBID, got %s", globalRANNodeID.GlobalNgENBID)
	}

	if globalRANNodeID.GlobalN3IWFID != "" {
		t.Errorf("expected empty GlobalN3IWFID, got %s", globalRANNodeID.GlobalN3IWFID)
	}

	item1 := ngapMsg.InitiatingMessage.Value.NGSetupRequest.IEs[1]

	if item1.ID.Label != "RANNodeName" {
		t.Errorf("expected ID=RANNodeName, got %v", item1.ID)
	}

	if item1.ID.Value != int(ngapType.ProtocolIEIDRANNodeName) {
		t.Errorf("expected ID value=82, got %d", item1.ID.Value)
	}

	if item1.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item1.Criticality)
	}

	if item1.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item1.Criticality.Value)
	}

	ranNodeName, ok := item1.Value.(*string)
	if !ok {
		t.Fatalf("expected RANNodeName, got %T", item1.Value)
	}

	if ranNodeName == nil {
		t.Fatalf("expected RANNodeName, got nil")
	}

	if *ranNodeName != "UERANSIM-gnb-1-1-1" {
		t.Errorf("expected RANNodeName=UERANSIM-gnb-1-1-1, got %s", *ranNodeName)
	}

	item2 := ngapMsg.InitiatingMessage.Value.NGSetupRequest.IEs[2]

	if item2.ID.Label != "SupportedTAList" {
		t.Errorf("expected ID=SupportedTAList, got %s", item2.ID.Label)
	}

	if item2.ID.Value != int(ngapType.ProtocolIEIDSupportedTAList) {
		t.Errorf("expected ID value=102, got %d", item2.ID.Value)
	}

	if item2.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item2.Criticality)
	}

	if item2.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item2.Criticality.Value)
	}

	supportedTAList, ok := item2.Value.([]ngap.SupportedTA)
	if !ok {
		t.Fatalf("expected SupportedTAList, got %T", item2.Value)
	}

	if supportedTAList == nil {
		t.Fatalf("expected SupportedTAList, got nil")
	}

	if len(supportedTAList) != 1 {
		t.Fatalf("expected 1 SupportedTAItem, got %d", len(supportedTAList))
	}

	supportedTAItem := supportedTAList[0]

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

	item3 := ngapMsg.InitiatingMessage.Value.NGSetupRequest.IEs[3]

	if item3.ID.Label != "DefaultPagingDRX" {
		t.Errorf("expected ID=DefaultPagingDRX, got %s", item3.ID.Label)
	}

	if item3.ID.Value != int(ngapType.ProtocolIEIDDefaultPagingDRX) {
		t.Errorf("expected ID value=21, got %d", item3.ID.Value)
	}

	if item3.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item3.Criticality)
	}

	if item3.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item3.Criticality.Value)
	}

	defaultPagingDRX, ok := item3.Value.(*ngap.EnumField)
	if !ok {
		t.Fatalf("expected DefaultPagingDRX, got %T", item3.Value)
	}

	if defaultPagingDRX == nil {
		t.Fatalf("expected DefaultPagingDRX, got nil")
	}

	if defaultPagingDRX.Label != "v128" {
		t.Errorf("expected DefaultPagingDRX=v128, got %s", defaultPagingDRX.Label)
	}

	if defaultPagingDRX.Value != int(ngapType.PagingDRXPresentV128) {
		t.Errorf("expected DefaultPagingDRX value=2, got %d", defaultPagingDRX.Value)
	}
}
