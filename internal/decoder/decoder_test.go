package decoder_test

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/decoder"
)

func decodeB64(s string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}

	return nil, fmt.Errorf("not valid base64")
}

func TestDecode_NGSetupRequest(t *testing.T) {
	const message = "ABUAQQAABAAbAAkAAPEQUAAAAAEAUkAUCIBVRVJBTlNJTS1nbmItMS0xLTEAZgAQAAAAAAEAAPEQAAAQCBAgMAAVQAFA"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngap, err := decoder.DecodeNetworkLog(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngap.ProcedureCode != "NGSetup" {
		t.Errorf("expected ProcedureCode=NGSetup, got %s", ngap.ProcedureCode)
	}

	if ngap.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", ngap.Criticality)
	}

	if ngap.InitiatingMessage == nil {
		t.Fatalf("expected InitiatingMessage, got nil")
	}

	if ngap.InitiatingMessage.NGSetupRequest == nil {
		t.Fatalf("expected NGSetupRequest, got nil")
	}

	if len(ngap.InitiatingMessage.NGSetupRequest.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngap.InitiatingMessage.NGSetupRequest.IEs))
	}

	item0 := ngap.InitiatingMessage.NGSetupRequest.IEs[0]

	if item0.ID != "GlobalRANNodeID (27)" {
		t.Errorf("expected ID=GlobalRANNodeID (27), got %s", item0.ID)
	}

	if item0.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item0.Criticality)
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

	item1 := ngap.InitiatingMessage.NGSetupRequest.IEs[1]

	if item1.ID != "RANNodeName (82)" {
		t.Errorf("expected ID=RANNodeName (82), got %s", item1.ID)
	}

	if item1.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item1.Criticality)
	}

	if item1.RANNodeName == nil {
		t.Fatalf("expected RANNodeName, got nil")
	}

	if *item1.RANNodeName != "UERANSIM-gnb-1-1-1" {
		t.Errorf("expected RANNodeName=UERANSIM-gnb-1-1-1, got %s", *item1.RANNodeName)
	}

	item2 := ngap.InitiatingMessage.NGSetupRequest.IEs[2]

	if item2.ID != "SupportedTAList (102)" {
		t.Errorf("expected ID=SupportedTAList (102), got %s", item2.ID)
	}

	if item2.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item2.Criticality)
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

	if supportedTAItem.BroadcastPLMNList[0].PLMNIdentity != "00f110" {
		t.Errorf("expected PLMNIdentity=00f110, got %s", supportedTAItem.BroadcastPLMNList[0].PLMNIdentity)
	}

	if len(supportedTAItem.BroadcastPLMNList[0].TAISliceSupportList) != 1 {
		t.Fatalf("expected 1 SNSSAI, got %d", len(supportedTAItem.BroadcastPLMNList[0].TAISliceSupportList))
	}

	snssai := supportedTAItem.BroadcastPLMNList[0].TAISliceSupportList[0]

	if snssai.SST != 1 {
		t.Errorf("expected SST=1, got %d", snssai.SST)
	}

	if snssai.SD == nil || *snssai.SD != "102030" {
		t.Errorf("expected SD=%s, got %v", "102030", snssai.SD)
	}

	item3 := ngap.InitiatingMessage.NGSetupRequest.IEs[3]

	if item3.ID != "DefaultPagingDRX (21)" {
		t.Errorf("expected ID=DefaultPagingDRX (21), got %s", item3.ID)
	}

	if item3.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item3.Criticality)
	}

	if item3.DefaultPagingDRX == nil {
		t.Fatalf("expected DefaultPagingDRX, got nil")
	}

	expectedDRX := "v128"
	if *item3.DefaultPagingDRX != expectedDRX {
		t.Errorf("expected DefaultPagingDRX=%s, got %s", expectedDRX, *item3.DefaultPagingDRX)
	}
}
