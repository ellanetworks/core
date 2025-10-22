package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/omec-project/ngap/ngapType"
)

func TestDecodeNGAPMessage_NGSetupResponse(t *testing.T) {
	const message = "IBUALAAABAABAAUBAGFtZgBgAAgAAADxEMr+AABWQAH/AFAACwAA8RAAABAIECAw"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "SuccessfulOutcome" {
		t.Errorf("expected PDUType=SuccessfulOutcome, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.MessageType != "NGSetupResponse" {
		t.Errorf("expected MessageType=NGSetupResponse, got %v", ngapMsg.MessageType)
	}

	if ngapMsg.ProcedureCode.Label != "NGSetup" {
		t.Errorf("expected ProcedureCode=NGSetup, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != ngapType.ProcedureCodeNGSetup {
		t.Errorf("expected ProcedureCode value=1, got %d", ngapMsg.ProcedureCode.Value)
	}

	if ngapMsg.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", ngapMsg.Criticality)
	}

	if ngapMsg.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", ngapMsg.Criticality.Value)
	}

	if len(ngapMsg.Value.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Label != "AMFName" {
		t.Errorf("expected ID=AMFName, got %s", item0.ID.Label)
	}

	if item0.ID.Value != ngapType.ProtocolIEIDAMFName {
		t.Errorf("expected ID value=1, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item0.Criticality.Value)
	}

	amfName, ok := item0.Value.(string)
	if !ok {
		t.Fatalf("expected string, got %T", item0.Value)
	}

	if amfName != "amf" {
		t.Errorf("expected AMFName=amf, got %s", amfName)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Label != "ServedGUAMIList" {
		t.Errorf("expected ID=ServedGUAMIList, got %s", item1.ID.Label)
	}

	if item1.ID.Value != ngapType.ProtocolIEIDServedGUAMIList {
		t.Errorf("expected ID value=96, got %d", item1.ID.Value)
	}

	if item1.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item1.Criticality)
	}

	if item1.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item1.Criticality.Value)
	}

	servedGUAMIList, ok := item1.Value.([]ngap.Guami)
	if !ok {
		t.Fatalf("expected ServedGUAMIList, got %T", item1.Value)
	}

	if servedGUAMIList == nil {
		t.Fatalf("expected ServedGUAMIList, got nil")
	}

	if len(servedGUAMIList) != 1 {
		t.Fatalf("expected 1 GUAMI, got %d", len(servedGUAMIList))
	}

	guami := servedGUAMIList[0]

	if guami.PLMNID.Mcc != "001" {
		t.Errorf("expected PLMNID.Mcc=001, got %s", guami.PLMNID.Mcc)
	}

	if guami.PLMNID.Mnc != "01" {
		t.Errorf("expected PLMNID.Mnc=01, got %s", guami.PLMNID.Mnc)
	}

	if guami.AMFID != "cafe00" {
		t.Errorf("expected AMFID=cafe00, got %s", guami.AMFID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Label != "RelativeAMFCapacity" {
		t.Errorf("expected ID=RelativeAMFCapacity, got %s", item2.ID.Label)
	}

	if item2.ID.Value != ngapType.ProtocolIEIDRelativeAMFCapacity {
		t.Errorf("expected ID value=86, got %d", item2.ID.Value)
	}

	if item2.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item2.Criticality)
	}

	if item2.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item2.Criticality.Value)
	}

	relativeAMFCapacity, ok := item2.Value.(int64)
	if !ok {
		t.Fatalf("expected int64, got %T", item2.Value)
	}

	if relativeAMFCapacity != 255 {
		t.Errorf("expected RelativeAMFCapacity=255, got %d", relativeAMFCapacity)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Label != "PLMNSupportList" {
		t.Errorf("expected ID=PLMNSupportList, got %s", item3.ID.Label)
	}

	if item3.ID.Value != ngapType.ProtocolIEIDPLMNSupportList {
		t.Errorf("expected ID value=80, got %d", item3.ID.Value)
	}

	if item3.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item3.Criticality)
	}

	if item3.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item3.Criticality.Value)
	}

	plmnSupportList, ok := item3.Value.([]ngap.PLMN)
	if !ok {
		t.Fatalf("expected PLMNSupportList, got %T", item3.Value)
	}

	if plmnSupportList == nil {
		t.Fatalf("expected PLMNSupportList, got nil")
	}

	if len(plmnSupportList) != 1 {
		t.Fatalf("expected 1 PLMNSupportItem, got %d", len(plmnSupportList))
	}

	plmnItem := plmnSupportList[0]

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
