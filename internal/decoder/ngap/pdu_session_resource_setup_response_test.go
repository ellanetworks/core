package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/omec-project/ngap/ngapType"
)

func TestDecodeNGAPMessage_PDUSessionResourceSetupResponse(t *testing.T) {
	const message = "IB0AOwAABAAKQAIAAQBVQAIAAQBLQBEAAAENAAPgISEh0QAAAAEAAQB5QBNQAPEQAAAAAQAA8RAAAAHsmi1m"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "SuccessfulOutcome" {
		t.Errorf("expected PDUType=SuccessfulOutcome, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.MessageType != "PDUSessionResourceSetupResponse" {
		t.Errorf("expected MessageType=PDUSessionResourceSetupResponse, got %v", ngapMsg.MessageType)
	}

	if ngapMsg.ProcedureCode.Label != "PDUSessionResourceSetup" {
		t.Errorf("expected ProcedureCode=PDUSessionResourceSetup, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != ngapType.ProcedureCodePDUSessionResourceSetup {
		t.Errorf("expected ProcedureCode value=21, got %d", ngapMsg.ProcedureCode.Value)
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

	if item0.ID.Label != "AMFUENGAPID" {
		t.Errorf("expected ID=AMFUENGAPID, got %s", item0.ID.Label)
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

	if amfUENGAPID != 1 {
		t.Errorf("expected AMFUENGAPID=1, got %d", amfUENGAPID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Label != "RANUENGAPID" {
		t.Errorf("expected ID=RANUENGAPID, got %s", item1.ID.Label)
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

	if ranUENGAPID != 1 {
		t.Errorf("expected RANUENGAPID=1, got %d", ranUENGAPID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Label != "PDUSessionResourceSetupListSURes" {
		t.Errorf("expected ID=PDUSessionResourceSetupListSURes, got %s", item2.ID.Label)
	}

	if item2.ID.Value != ngapType.ProtocolIEIDPDUSessionResourceSetupListSURes {
		t.Errorf("expected ID value=75, got %d", item2.ID.Value)
	}

	if item2.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item2.Criticality)
	}

	if item2.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item2.Criticality.Value)
	}

	pduSessionResourceSetupListSURes, ok := item2.Value.([]ngap.PDUSessionResourceSetupSURes)
	if !ok {
		t.Fatalf("expected PDUSessionResourceSetupListSURes to be of type []PDUSessionResourceSetupSURes, got %T", item2.Value)
	}

	if len(pduSessionResourceSetupListSURes) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceSetupItemSURes, got %d", len(pduSessionResourceSetupListSURes))
	}

	pduItem := pduSessionResourceSetupListSURes[0]

	if pduItem.PDUSessionID != 1 {
		t.Errorf("expected PDUSessionID=1, got %d", pduItem.PDUSessionID)
	}

	if pduItem.PDUSessionResourceSetupResponseTransfer == nil {
		t.Fatalf("expected PDUSessionResourceSetupResponseTransfer, got nil")
	}

	expectedTransfer := "AAPgISEh0QAAAAEAAQ=="
	expectedTransferRaw, err := decodeB64(expectedTransfer)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if string(pduItem.PDUSessionResourceSetupResponseTransfer) != string(expectedTransferRaw) {
		t.Errorf("expected PDUSessionResourceSetupResponseTransfer=%s, got %s", expectedTransfer, pduItem.PDUSessionResourceSetupResponseTransfer)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Label != "UserLocationInformation" {
		t.Errorf("expected ID=UserLocationInformation, got %v", item3.ID)
	}

	if item3.ID.Value != ngapType.ProtocolIEIDUserLocationInformation {
		t.Errorf("expected ID value=121, got %d", item3.ID.Value)
	}

	if item3.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item3.Criticality)
	}

	if item3.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item3.Criticality.Value)
	}
}
