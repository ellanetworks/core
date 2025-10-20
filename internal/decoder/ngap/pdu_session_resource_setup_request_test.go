package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/omec-project/ngap/ngapType"
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

	if ngap.InitiatingMessage.ProcedureCode.Label != "PDUSessionResourceSetup" {
		t.Errorf("expected ProcedureCode=PDUSessionResourceSetup, got %v", ngap.InitiatingMessage.ProcedureCode)
	}

	if ngap.InitiatingMessage.ProcedureCode.Value != int(ngapType.ProcedureCodePDUSessionResourceSetup) {
		t.Errorf("expected ProcedureCode value=21, got %d", ngap.InitiatingMessage.ProcedureCode.Value)
	}

	if ngap.InitiatingMessage.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", ngap.InitiatingMessage.Criticality)
	}

	if ngap.InitiatingMessage.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", ngap.InitiatingMessage.Criticality.Value)
	}

	if ngap.InitiatingMessage.Value.PDUSessionResourceSetupRequest == nil {
		t.Fatalf("expected PDUSessionResourceSetupRequest, got nil")
	}

	if len(ngap.InitiatingMessage.Value.PDUSessionResourceSetupRequest.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngap.InitiatingMessage.Value.PDUSessionResourceSetupRequest.IEs))
	}

	item0 := ngap.InitiatingMessage.Value.PDUSessionResourceSetupRequest.IEs[0]

	if item0.ID.Label != "AMFUENGAPID" {
		t.Errorf("expected ID=AMFUENGAPID, got %s", item0.ID.Label)
	}

	if item0.ID.Value != int(ngapType.ProtocolIEIDAMFUENGAPID) {
		t.Errorf("expected ID value=10, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item0.Criticality.Value)
	}

	if item0.AMFUENGAPID == nil {
		t.Fatalf("expected AMFUENGAPID, got nil")
	}

	if *item0.AMFUENGAPID != 1 {
		t.Errorf("expected AMFUENGAPID=1, got %d", *item0.AMFUENGAPID)
	}

	item1 := ngap.InitiatingMessage.Value.PDUSessionResourceSetupRequest.IEs[1]

	if item1.ID.Label != "RANUENGAPID" {
		t.Errorf("expected ID=RANUENGAPID, got %v", item1.ID)
	}

	if item1.ID.Value != int(ngapType.ProtocolIEIDRANUENGAPID) {
		t.Errorf("expected ID value=85, got %d", item1.ID.Value)
	}

	if item1.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item1.Criticality)
	}

	if item1.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item1.Criticality.Value)
	}

	ranUENGAPID, ok := item1.Value.(int64)
	if !ok {
		t.Fatalf("expected RANUENGAPID to be of type int64, got %T", item1.Value)
	}

	if ranUENGAPID != 1 {
		t.Errorf("expected RANUENGAPID=1, got %d", ranUENGAPID)
	}

	item2 := ngap.InitiatingMessage.Value.PDUSessionResourceSetupRequest.IEs[2]

	if item2.ID.Label != "PDUSessionResourceSetupListSUReq" {
		t.Errorf("expected ID=PDUSessionResourceSetupListSUReq, got %s", item2.ID.Label)
	}

	if item2.ID.Value != int(ngapType.ProtocolIEIDPDUSessionResourceSetupListSUReq) {
		t.Errorf("expected ID value=74, got %d", item2.ID.Value)
	}

	if item2.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item2.Criticality)
	}

	if item2.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item2.Criticality.Value)
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

	if item3.ID.Label != "UEAggregateMaximumBitRate" {
		t.Errorf("expected ID=UEAggregateMaximumBitRate, got %v", item3.ID)
	}

	if item3.ID.Value != int(ngapType.ProtocolIEIDUEAggregateMaximumBitRate) {
		t.Errorf("expected ID value=110, got %d", item3.ID.Value)
	}

	if item3.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item3.Criticality)
	}

	if item3.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item3.Criticality.Value)
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
