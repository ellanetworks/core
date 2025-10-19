package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
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

	if ngap.SuccessfulOutcome.ProcedureCode != "PDUSessionResourceSetup" {
		t.Errorf("expected ProcedureCode=PDUSessionResourceSetup, got %s", ngap.SuccessfulOutcome.ProcedureCode)
	}

	if ngap.SuccessfulOutcome.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", ngap.SuccessfulOutcome.Criticality)
	}

	if ngap.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse == nil {
		t.Fatalf("expected PDUSessionResourceSetupResponse, got nil")
	}

	if len(ngap.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngap.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse.IEs))
	}

	item0 := ngap.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse.IEs[0]

	if item0.ID != "AMFUENGAPID (10)" {
		t.Errorf("expected ID=AMFUENGAPID (10), got %s", item0.ID)
	}

	if item0.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item0.Criticality)
	}

	if item0.AMFUENGAPID == nil {
		t.Fatalf("expected AMFUENGAPID, got nil")
	}

	if *item0.AMFUENGAPID != 1 {
		t.Errorf("expected AMFUENGAPID=1, got %d", *item0.AMFUENGAPID)
	}

	item1 := ngap.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse.IEs[1]

	if item1.ID != "RANUENGAPID (85)" {
		t.Errorf("expected ID=RANUENGAPID (85), got %s", item1.ID)
	}

	if item1.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item1.Criticality)
	}

	if item1.RANUENGAPID == nil {
		t.Fatalf("expected RANUENGAPID, got nil")
	}

	if *item1.RANUENGAPID != 1 {
		t.Errorf("expected RANUENGAPID=1, got %d", *item1.RANUENGAPID)
	}

	item2 := ngap.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse.IEs[2]

	if item2.ID != "PDUSessionResourceSetupListSURes (75)" {
		t.Errorf("expected ID=PDUSessionResourceSetupListSURes (75), got %s", item2.ID)
	}

	if item2.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item2.Criticality)
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

	if item3.ID != "UserLocationInformation (121)" {
		t.Errorf("expected ID=UserLocationInformation (121), got %s", item3.ID)
	}

	if item3.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item3.Criticality)
	}
}
