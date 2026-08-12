// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	lib "github.com/ellanetworks/core/ngap"
)

func TestDecodeNGAPMessage_PDUSessionResourceSetupRequest(t *testing.T) {
	const message = "AB0AgLwAAAQACgACAAEAVQACAAEASgCAmgBAAWF+AnHdg8QCfgBoAQBSLgEBwhEACf8ABjH/AQH/CQYGAMgGAMgpBQEKLQACIgQBECAweQAQASBDAQEJBAMGAMgFAwYAyHsADYAADQQICAgIABACBXglCQhpbnRlcm5ldBIBQCAQIDAvAAAEAIIACgwL68IAMAvrwgAAiwAKAfAhISHGAAAAAQCGAAEAAIgABwABAAAJAQAAbkAKDAvrwgAwC+vCAA=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcPDUSessionResourceSetup) {
		t.Errorf("expected ProcedureCode=PDUSessionResourceSetup, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcPDUSessionResourceSetup) {
		t.Errorf("procedure code = %d, want %d", ngapMsg.ProcedureCode.Value, lib.ProcPDUSessionResourceSetup)
	}

	if ngapMsg.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", ngapMsg.Criticality)
	}

	if len(ngapMsg.Value.IEs) != 4 {
		t.Errorf("expected 4 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Value != int64(lib.IDAMFUENGAPID) {
		t.Errorf("IE id = %d, want %d", item0.ID.Value, lib.IDAMFUENGAPID)
	}

	if item0.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item0.Criticality)
	}

	amfUENGAPID, ok := item0.Value.(int64)
	if !ok {
		t.Fatalf("expected AMF-UE-NGAP-ID to be of type int64, got %T", item0.Value)
	}

	if amfUENGAPID != 1 {
		t.Errorf("expected AMF-UE-NGAP-ID=1, got %d", amfUENGAPID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Value != int64(lib.IDRANUENGAPID) {
		t.Errorf("IE id = %d, want %d", item1.ID.Value, lib.IDRANUENGAPID)
	}

	if item1.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item1.Criticality)
	}

	ranUENGAPID, ok := item1.Value.(int64)
	if !ok {
		t.Fatalf("expected RAN-UE-NGAP-ID to be of type int64, got %T", item1.Value)
	}

	if ranUENGAPID != 1 {
		t.Errorf("expected RAN-UE-NGAP-ID=1, got %d", ranUENGAPID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Value != int64(lib.IDPDUSessionResourceSetupListSUReq) {
		t.Errorf("IE id = %d, want %d", item2.ID.Value, lib.IDPDUSessionResourceSetupListSUReq)
	}

	if item2.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item2.Criticality)
	}

	pduSessionResourceSetupListSUReq, ok := item2.Value.([]PDUSessionResourceSetupSUReq)
	if !ok {
		t.Fatalf("expected PDUSessionResourceSetupListSUReq to be of type []PDUSessionResourceSetupSUReq, got %T", item2.Value)
	}

	if len(pduSessionResourceSetupListSUReq) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceSetupItemSUReq, got %d", len(pduSessionResourceSetupListSUReq))
	}

	pduItem := pduSessionResourceSetupListSUReq[0]

	if pduItem.PDUSessionID != 1 {
		t.Errorf("expected PDUSessionID=1, got %d", pduItem.PDUSessionID)
	}

	if pduItem.PDUSessionResourceSetupRequestTransfer == nil {
		t.Fatalf("expected PDUSessionResourceSetupRequestTransfer, got nil")
	}

	if pduItem.Error != "" {
		t.Errorf("expected no error, got %s", pduItem.Error)
	}

	transfer := pduItem.PDUSessionResourceSetupRequestTransfer

	if transfer.ULNGUUPTNLInformation == nil {
		t.Fatalf("expected UL-NGU-UP-TNLInformation, got nil")
	}

	if transfer.ULNGUUPTNLInformation.GTPTunnel.TransportLayerAddress != "33.33.33.198" {
		t.Errorf("expected TransportLayerAddress=33.33.33.198, got %s", transfer.ULNGUUPTNLInformation.GTPTunnel.TransportLayerAddress)
	}

	if len(transfer.QosFlowSetupRequestList) != 1 {
		t.Fatalf("expected 1 QoS flow, got %d", len(transfer.QosFlowSetupRequestList))
	}

	if transfer.PduSType == nil {
		t.Fatalf("expected PduSType, got nil")
	}

	if transfer.PduSType.Value != int64(lib.PDUSessionTypeIPv4) {
		t.Errorf("expected PduSType=ipv4, got %s", transfer.PduSType.Label)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Value != int64(lib.IDUEAggregateMaximumBitRate) {
		t.Errorf("IE id = %d, want %d", item3.ID.Value, lib.IDUEAggregateMaximumBitRate)
	}

	if item3.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item3.Criticality)
	}

	ueAggregateMaximumBitRate, ok := item3.Value.(UEAggregateMaximumBitRate)
	if !ok {
		t.Fatalf("expected UEAggregateMaximumBitRate to be of type UEAggregateMaximumBitRate, got %T", item3.Value)
	}

	if ueAggregateMaximumBitRate.Uplink != 200000000 {
		t.Errorf("expected Uplink=100000000, got %d", ueAggregateMaximumBitRate.Uplink)
	}

	if ueAggregateMaximumBitRate.Downlink != 200000000 {
		t.Errorf("expected Downlink=200000000, got %d", ueAggregateMaximumBitRate.Downlink)
	}
}
