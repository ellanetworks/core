// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	lib "github.com/ellanetworks/core/ngap"
)

func TestDecodeNGAPMessage_UEContextReleaseRequest(t *testing.T) {
	const message = "ACpAHAAABAAKAAIAGwBVAAIAGwCFAAMAAAEAD0ACBUA="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcUEContextReleaseRequest) {
		t.Errorf("expected ProcedureCode=UEContextReleaseRequest, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcUEContextReleaseRequest) {
		t.Errorf("procedure code = %d, want %d", ngapMsg.ProcedureCode.Value, lib.ProcUEContextReleaseRequest)
	}

	if ngapMsg.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", ngapMsg.Criticality)
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

	amfUeNgapID, ok := item0.Value.(int64)
	if !ok {
		t.Errorf("expected AMF-UE-NGAP-ID value type=int64, got %T", item0.Value)
	}

	if amfUeNgapID != 27 {
		t.Errorf("expected AMF-UE-NGAP-ID=12345, got %d", amfUeNgapID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Value != int64(lib.IDRANUENGAPID) {
		t.Errorf("IE id = %d, want %d", item1.ID.Value, lib.IDRANUENGAPID)
	}

	if item1.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item1.Criticality)
	}

	ranUeNgapID, ok := item1.Value.(int64)
	if !ok {
		t.Errorf("expected RAN-UE-NGAP-ID value type=int64, got %T", item1.Value)
	}

	if ranUeNgapID != 27 {
		t.Errorf("expected RAN-UE-NGAP-ID=27, got %d", ranUeNgapID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Value != int64(lib.IDPDUSessionResourceListCxtRelReq) {
		t.Errorf("IE id = %d, want %d", item2.ID.Value, lib.IDPDUSessionResourceListCxtRelReq)
	}

	if item2.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item2.Criticality)
	}

	pduSessionList, ok := item2.Value.([]PDUSessionResourceListCxtRelReq)
	if !ok {
		t.Fatalf("expected PDUSessionResourceListCxtRelReq value type=[]PDUSessionResourceListCxtRelReq, got %T", item2.Value)
	}

	if len(pduSessionList) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceListCxtRelReq, got %d", len(pduSessionList))
	}

	if pduSessionList[0].PDUSessionID != 1 {
		t.Errorf("expected PDUSessionID=1, got %d", pduSessionList[0].PDUSessionID)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Value != int64(lib.IDCause) {
		t.Errorf("IE id = %d, want %d", item3.ID.Value, lib.IDCause)
	}

	if item3.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item3.Criticality)
	}

	cause, ok := item3.Value.(Cause)
	if !ok {
		t.Fatalf("expected Cause value type=Cause, got %T", item3.Value)
	}

	if cause.Value.Value != int64(lib.CauseRadioNetworkRadioConnectionWithUELost) {
		t.Errorf("expected Cause=radio-connection-with-ue-lost, got %s", cause.Value.Label)
	}

	if cause.Value.Value != int64(lib.CauseRadioNetworkRadioConnectionWithUELost) {
		t.Errorf("expected Cause value=21, got %d", cause.Value.Value)
	}
}
