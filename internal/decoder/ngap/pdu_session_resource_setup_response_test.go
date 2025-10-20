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

	ngap, err := ngap.DecodeNGAPMessage(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngap.SuccessfulOutcome == nil {
		t.Fatalf("expected SuccessfulOutcome, got nil")
	}

	if ngap.SuccessfulOutcome.ProcedureCode.Label != "PDUSessionResourceSetup" {
		t.Errorf("expected ProcedureCode=PDUSessionResourceSetup, got %v", ngap.SuccessfulOutcome.ProcedureCode)
	}

	if ngap.SuccessfulOutcome.ProcedureCode.Value != int(ngapType.ProcedureCodePDUSessionResourceSetup) {
		t.Errorf("expected ProcedureCode value=21, got %d", ngap.SuccessfulOutcome.ProcedureCode.Value)
	}

	if ngap.SuccessfulOutcome.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", ngap.SuccessfulOutcome.Criticality)
	}

	if ngap.SuccessfulOutcome.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", ngap.SuccessfulOutcome.Criticality.Value)
	}

	if ngap.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse == nil {
		t.Fatalf("expected PDUSessionResourceSetupResponse, got nil")
	}

	if len(ngap.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngap.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse.IEs))
	}

	item0 := ngap.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse.IEs[0]

	if item0.ID.Label != "AMFUENGAPID" {
		t.Errorf("expected ID=AMFUENGAPID, got %s", item0.ID.Label)
	}

	if item0.ID.Value != int(ngapType.ProtocolIEIDAMFUENGAPID) {
		t.Errorf("expected ID value=10, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item0.Criticality.Value)
	}

	if item0.AMFUENGAPID == nil {
		t.Fatalf("expected AMFUENGAPID, got nil")
	}

	if *item0.AMFUENGAPID != 1 {
		t.Errorf("expected AMFUENGAPID=1, got %d", *item0.AMFUENGAPID)
	}

	item1 := ngap.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse.IEs[1]

	if item1.ID.Label != "RANUENGAPID" {
		t.Errorf("expected ID=RANUENGAPID, got %s", item1.ID.Label)
	}

	if item1.ID.Value != int(ngapType.ProtocolIEIDRANUENGAPID) {
		t.Errorf("expected ID value=85, got %d", item1.ID.Value)
	}

	if item1.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item1.Criticality)
	}

	if item1.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item1.Criticality.Value)
	}

	if item1.RANUENGAPID == nil {
		t.Fatalf("expected RANUENGAPID, got nil")
	}

	if *item1.RANUENGAPID != 1 {
		t.Errorf("expected RANUENGAPID=1, got %d", *item1.RANUENGAPID)
	}

	item2 := ngap.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse.IEs[2]

	if item2.ID.Label != "PDUSessionResourceSetupListSURes" {
		t.Errorf("expected ID=PDUSessionResourceSetupListSURes, got %s", item2.ID.Label)
	}

	if item2.ID.Value != int(ngapType.ProtocolIEIDPDUSessionResourceSetupListSURes) {
		t.Errorf("expected ID value=75, got %d", item2.ID.Value)
	}

	if item2.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item2.Criticality)
	}

	if item2.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item2.Criticality.Value)
	}

	if item2.PDUSessionResourceSetupListSURes == nil {
		t.Fatalf("expected PDUSessionResourceSetupListSURes, got nil")
	}

	if len(item2.PDUSessionResourceSetupListSURes) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceSetupItemSURes, got %d", len(item2.PDUSessionResourceSetupListSURes))
	}

	pduItem := item2.PDUSessionResourceSetupListSURes[0]

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

	item3 := ngap.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse.IEs[3]

	if item3.ID.Label != "UserLocationInformation" {
		t.Errorf("expected ID=UserLocationInformation, got %v", item3.ID)
	}

	if item3.ID.Value != int(ngapType.ProtocolIEIDUserLocationInformation) {
		t.Errorf("expected ID value=121, got %d", item3.ID.Value)
	}

	if item3.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item3.Criticality)
	}

	if item3.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item3.Criticality.Value)
	}
}
