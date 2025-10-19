package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
)

func TestDecodeNGAPMessage_PDUSessionResourceSetupRequest(t *testing.T) {
	const message = "AB0AgLwAAAQACgACAAEAVQACAAEASgCAmgBAAWF+AnHdg8QCfgBoAQBSLgEBwhEACf8ABjH/AQH/CQYGAMgGAMgpBQEKLQACIgQBECAweQAQASBDAQEJBAMGAMgFAwYAyHsADYAADQQICAgIABACBXglCQhpbnRlcm5ldBIBQCAQIDAvAAAEAIIACgwL68IAMAvrwgAAiwAKAfAhISHGAAAAAQCGAAEAAIgABwABAAAJAQAAbkAKDAvrwgAwC+vCAA=="

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

	if ngap.InitiatingMessage.ProcedureCode != "PDUSessionResourceSetup" {
		t.Errorf("expected ProcedureCode=PDUSessionResourceSetup, got %s", ngap.InitiatingMessage.ProcedureCode)
	}

	if ngap.InitiatingMessage.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", ngap.InitiatingMessage.Criticality)
	}

	if ngap.InitiatingMessage.Value.PDUSessionResourceSetupRequest == nil {
		t.Fatalf("expected PDUSessionResourceSetupRequest, got nil")
	}

	if len(ngap.InitiatingMessage.Value.PDUSessionResourceSetupRequest.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngap.InitiatingMessage.Value.PDUSessionResourceSetupRequest.IEs))
	}

	item0 := ngap.InitiatingMessage.Value.PDUSessionResourceSetupRequest.IEs[0]

	if item0.ID != "AMFUENGAPID (10)" {
		t.Errorf("expected ID=AMFUENGAPID (10), got %s", item0.ID)
	}

	if item0.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item0.Criticality)
	}

	if item0.AMFUENGAPID == nil {
		t.Fatalf("expected AMFUENGAPID, got nil")
	}

	if *item0.AMFUENGAPID != 1 {
		t.Errorf("expected AMFUENGAPID=1, got %d", *item0.AMFUENGAPID)
	}

	item1 := ngap.InitiatingMessage.Value.PDUSessionResourceSetupRequest.IEs[1]

	if item1.ID != "RANUENGAPID (85)" {
		t.Errorf("expected ID=RANUENGAPID (85), got %s", item1.ID)
	}

	if item1.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item1.Criticality)
	}

	if item1.RANUENGAPID == nil {
		t.Fatalf("expected RANUENGAPID, got nil")
	}

	if *item1.RANUENGAPID != 1 {
		t.Errorf("expected RANUENGAPID=1, got %d", *item1.RANUENGAPID)
	}

	item2 := ngap.InitiatingMessage.Value.PDUSessionResourceSetupRequest.IEs[2]

	if item2.ID != "PDUSessionResourceSetupListSUReq (74)" {
		t.Errorf("expected ID=PDUSessionResourceSetupListSUReq (74), got %s", item2.ID)
	}

	if item2.Criticality != "Reject (0)" {
		t.Errorf("expected Criticality=Reject (0), got %s", item2.Criticality)
	}

	if item2.PDUSessionResourceSetupListSUReq == nil {
		t.Fatalf("expected PDUSessionResourceSetupListSUReq, got nil")
	}

	if len(item2.PDUSessionResourceSetupListSUReq) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceSetupItemSUReq, got %d", len(item2.PDUSessionResourceSetupListSUReq))
	}

	pduItem := item2.PDUSessionResourceSetupListSUReq[0]

	if pduItem.PDUSessionID != 1 {
		t.Errorf("expected PDUSessionID=1, got %d", pduItem.PDUSessionID)
	}

	if pduItem.PDUSessionResourceSetupRequestTransfer == nil {
		t.Fatalf("expected PDUSessionResourceSetupRequestTransfer, got nil")
	}

	expectedTransfer := "AAAEAIIACgwL68IAMAvrwgAAiwAKAfAhISHGAAAAAQCGAAEAAIgABwABAAAJAQA="
	expectedTransferRaw, err := decodeB64(expectedTransfer)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if string(pduItem.PDUSessionResourceSetupRequestTransfer) != string(expectedTransferRaw) {
		t.Errorf("expected PDUSessionResourceSetupRequestTransfer=%s, got %s", expectedTransfer, pduItem.PDUSessionResourceSetupRequestTransfer)
	}

	item3 := ngap.InitiatingMessage.Value.PDUSessionResourceSetupRequest.IEs[3]

	if item3.ID != "UEAggregateMaximumBitRate (110)" {
		t.Errorf("expected ID=UEAggregateMaximumBitRate (110), got %s", item3.ID)
	}

	if item3.Criticality != "Ignore (1)" {
		t.Errorf("expected Criticality=Ignore (1), got %s", item3.Criticality)
	}

	if item3.UEAggregateMaximumBitRate == nil {
		t.Fatalf("expected UEAggregateMaximumBitRate, got nil")
	}

	if item3.UEAggregateMaximumBitRate.Uplink != 200000000 {
		t.Errorf("expected Uplink=100000000, got %d", item3.UEAggregateMaximumBitRate.Uplink)
	}

	if item3.UEAggregateMaximumBitRate.Downlink != 200000000 {
		t.Errorf("expected Downlink=200000000, got %d", item3.UEAggregateMaximumBitRate.Downlink)
	}

	if item3.UEAggregateMaximumBitRate == nil {
		t.Fatalf("expected UEAggregateMaximumBitRate, got nil")
	}
}
