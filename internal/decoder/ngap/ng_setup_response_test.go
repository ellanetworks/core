package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
)

func TestDecodeNGAPMessage_NGSetupResponse(t *testing.T) {
	const message = "IBUALAAABAABAAUBAGFtZgBgAAgAAADxEMr+AABWQAH/AFAACwAA8RAAABAIECAw"

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

	if ngap.SuccessfulOutcome.ProcedureCode != "NGSetup" {
		t.Errorf("expected ProcedureCode=NGSetup, got %s", ngap.SuccessfulOutcome.ProcedureCode)
	}

	if ngap.SuccessfulOutcome.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", ngap.SuccessfulOutcome.Criticality)
	}

	if ngap.SuccessfulOutcome.Value.NGSetupResponse == nil {
		t.Fatalf("expected NGSetupResponse, got nil")
	}

	if len(ngap.SuccessfulOutcome.Value.NGSetupResponse.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngap.SuccessfulOutcome.Value.NGSetupResponse.IEs))
	}

	item0 := ngap.SuccessfulOutcome.Value.NGSetupResponse.IEs[0]

	if item0.ID != "AMFName (1)" {
		t.Errorf("expected ID=AMFName (1), got %s", item0.ID)
	}

	if item0.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item0.Criticality)
	}

	if item0.AMFName == nil {
		t.Fatalf("expected AMFName, got nil")
	}

	if *item0.AMFName != "amf" {
		t.Errorf("expected AMFName=amf, got %s", *item0.AMFName)
	}

	item1 := ngap.SuccessfulOutcome.Value.NGSetupResponse.IEs[1]

	if item1.ID != "ServedGUAMIList (96)" {
		t.Errorf("expected ID=ServedGUAMIList (96), got %s", item1.ID)
	}

	if item1.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item1.Criticality)
	}

	if item1.ServedGUAMIList == nil {
		t.Fatalf("expected ServedGUAMIList, got nil")
	}

	if len(item1.ServedGUAMIList) != 1 {
		t.Fatalf("expected 1 GUAMI, got %d", len(item1.ServedGUAMIList))
	}

	guami := item1.ServedGUAMIList[0]

	if guami.PLMNID.Mcc != "001" {
		t.Errorf("expected PLMNID.Mcc=001, got %s", guami.PLMNID.Mcc)
	}

	if guami.PLMNID.Mnc != "01" {
		t.Errorf("expected PLMNID.Mnc=01, got %s", guami.PLMNID.Mnc)
	}

	if guami.AMFID != "cafe00" {
		t.Errorf("expected AMFID=cafe00, got %s", guami.AMFID)
	}

	item2 := ngap.SuccessfulOutcome.Value.NGSetupResponse.IEs[2]

	if item2.ID != "RelativeAMFCapacity (86)" {
		t.Errorf("expected ID=RelativeAMFCapacity (86), got %s", item2.ID)
	}

	if item2.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item2.Criticality)
	}

	if item2.RelativeAMFCapacity == nil {
		t.Fatalf("expected RelativeAMFCapacity, got nil")
	}

	if *item2.RelativeAMFCapacity != 255 {
		t.Errorf("expected RelativeAMFCapacity=255, got %d", *item2.RelativeAMFCapacity)
	}

	item3 := ngap.SuccessfulOutcome.Value.NGSetupResponse.IEs[3]

	if item3.ID != "PLMNSupportList (80)" {
		t.Errorf("expected ID=PLMNSupportList (80), got %s", item3.ID)
	}

	if item3.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item3.Criticality)
	}

	if item3.PLMNSupportList == nil {
		t.Fatalf("expected PLMNSupportList, got nil")
	}

	if len(item3.PLMNSupportList) != 1 {
		t.Fatalf("expected 1 PLMNSupportItem, got %d", len(item3.PLMNSupportList))
	}

	plmnItem := item3.PLMNSupportList[0]

	if plmnItem.PLMNID.Mcc != "001" {
		t.Errorf("expected Mcc=001, got %s", plmnItem.PLMNID.Mcc)
	}

	if plmnItem.PLMNID.Mnc != "01" {
		t.Errorf("expected Mnc=01, got %s", plmnItem.PLMNID.Mnc)
	}

	if len(plmnItem.SliceSupportList) != 1 {
		t.Fatalf("expected 1 SNSSAI, got %d", len(plmnItem.SliceSupportList))
	}

	snssai := plmnItem.SliceSupportList[0]

	if snssai.SST != 1 {
		t.Errorf("expected SST=1, got %d", snssai.SST)
	}

	if snssai.SD == nil || *snssai.SD != "102030" {
		t.Errorf("expected SD=%s, got %v", "102030", snssai.SD)
	}
}
