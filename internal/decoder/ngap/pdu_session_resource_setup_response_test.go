// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	lib "github.com/ellanetworks/core/ngap"
)

func TestDecodeNGAPMessage_PDUSessionResourceSetupResponse(t *testing.T) {
	raw, err := decodeB64(pduSessionResourceSetupResponseCapture)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "SuccessfulOutcome" {
		t.Errorf("expected PDUType=SuccessfulOutcome, got %v", ngapMsg.PDUType)
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

	if item0.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item0.Criticality)
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

	if item1.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item1.Criticality)
	}

	ranUENGAPID, ok := item1.Value.(int64)
	if !ok {
		t.Fatalf("expected RAN-UE-NGAP-ID to be of type int64, got %T", item1.Value)
	}

	if ranUENGAPID != 1 {
		t.Errorf("expected RAN-UE-NGAP-ID=1, got %d", ranUENGAPID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Value != int64(lib.IDPDUSessionResourceSetupListSURes) {
		t.Errorf("IE id = %d, want %d", item2.ID.Value, lib.IDPDUSessionResourceSetupListSURes)
	}

	if item2.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item2.Criticality)
	}

	pduSessionResourceSetupListSURes, ok := item2.Value.([]PDUSessionResourceSetupSURes)
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

	if pduItem.Error != "" {
		t.Fatalf("unexpected error decoding response transfer: %s", pduItem.Error)
	}

	if pduItem.PDUSessionResourceSetupResponseTransfer == nil {
		t.Fatalf("expected PDUSessionResourceSetupResponseTransfer, got nil")
	}

	transfer := pduItem.PDUSessionResourceSetupResponseTransfer

	if transfer.DLQosFlowPerTNLInformation.GTPTunnel.TransportLayerAddress != "33.33.33.209" {
		t.Errorf("expected TransportLayerAddress=33.33.33.209, got %s", transfer.DLQosFlowPerTNLInformation.GTPTunnel.TransportLayerAddress)
	}

	if transfer.DLQosFlowPerTNLInformation.GTPTunnel.GTPTEID != 1 {
		t.Errorf("expected GTPTEID=1, got %d", transfer.DLQosFlowPerTNLInformation.GTPTunnel.GTPTEID)
	}

	if len(transfer.DLQosFlowPerTNLInformation.AssociatedQosFlows) != 1 {
		t.Fatalf("expected 1 AssociatedQosFlow, got %d", len(transfer.DLQosFlowPerTNLInformation.AssociatedQosFlows))
	}

	if transfer.DLQosFlowPerTNLInformation.AssociatedQosFlows[0].QosFlowIdentifier != 1 {
		t.Errorf("expected QosFlowIdentifier=1, got %d", transfer.DLQosFlowPerTNLInformation.AssociatedQosFlows[0].QosFlowIdentifier)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Value != int64(lib.IDUserLocationInformation) {
		t.Errorf("IE id = %d, want %d", item3.ID.Value, lib.IDUserLocationInformation)
	}

	if item3.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item3.Criticality)
	}

	if item3.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item3.Criticality)
	}
}

// A PDUSessionResourceSetupResponse captured on the 001/01 test PLMN.
const pduSessionResourceSetupResponseCapture = "IB0AOwAABAAKQAIAAQBVQAIAAQBLQBEAAAENAAPgISEh0QAAAAEAAQB5QBNQAPEQAAAAAQAA8RAAAAHsmi1m"
