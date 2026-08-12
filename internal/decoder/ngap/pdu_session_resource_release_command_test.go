// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"testing"

	lib "github.com/ellanetworks/core/ngap"
)

func TestDecodeNGAPMessage_PDUSessionResourceReleaseCommand(t *testing.T) {
	raw, err := decodeB64(pduSessionResourceReleaseCommandCapture)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcPDUSessionResourceRelease) {
		t.Errorf("expected ProcedureCode=PDUSessionResourceRelease, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcPDUSessionResourceRelease) {
		t.Errorf("procedure code = %d, want %d", ngapMsg.ProcedureCode.Value, lib.ProcPDUSessionResourceRelease)
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

	amfUeNgapID, ok := item0.Value.(int64)
	if !ok {
		t.Fatalf("expected AMF-UE-NGAP-ID value type=int64, got %T", item0.Value)
	}

	if amfUeNgapID != 148 {
		t.Errorf("expected AMF-UE-NGAP-ID=148, got %d", amfUeNgapID)
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
		t.Fatalf("expected RAN-UE-NGAP-ID value type=int64, got %T", item1.Value)
	}

	if ranUeNgapID != 1 {
		t.Errorf("expected RAN-UE-NGAP-ID=1, got %d", ranUeNgapID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Value != int64(lib.IDNASPDU) {
		t.Errorf("IE id = %d, want %d", item2.ID.Value, lib.IDNASPDU)
	}

	if item2.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item2.Criticality)
	}

	nasPdu, ok := item2.Value.(NASPDU)
	if !ok {
		t.Fatalf("expected NAS-PDU value type=NAS-PDU, got %T", item2.Value)
	}

	expectedNASPDU := "fgKGQUZ3A34AaAEABS4BBNMAEgE="

	expectedNASPDUraw, err := decodeB64(expectedNASPDU)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	expectedHex := hex.EncodeToString(expectedNASPDUraw)
	if nasPdu.RawHex != expectedHex {
		t.Errorf("expected RawHex=%s, got %s", expectedHex, nasPdu.RawHex)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Value != int64(lib.IDPDUSessionResourceToReleaseListRelCmd) {
		t.Errorf("IE id = %d, want %d", item3.ID.Value, lib.IDPDUSessionResourceToReleaseListRelCmd)
	}

	if item3.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item3.Criticality)
	}

	pduSessionList, ok := item3.Value.([]PDUSessionResourceToReleaseListRelCmd)
	if !ok {
		t.Fatalf("expected PDUSessionResourceToReleaseListRelCmd value type=[]PDUSessionResourceToReleaseListRelCmd, got %T", item3.Value)
	}

	if len(pduSessionList) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceToReleaseListRelCmd, got %d", len(pduSessionList))
	}

	if pduSessionList[0].PDUSessionID != 1 {
		t.Errorf("expected PDUSessionID=1, got %d", pduSessionList[0].PDUSessionID)
	}

	if pduSessionList[0].Error != "" {
		t.Fatalf("unexpected error decoding release command transfer: %s", pduSessionList[0].Error)
	}

	if pduSessionList[0].PDUSessionResourceReleaseCommandTransfer == nil {
		t.Fatalf("expected decoded release command transfer, got nil")
	}

	cause := pduSessionList[0].PDUSessionResourceReleaseCommandTransfer.Cause
	if cause.Value.Value == 0 && cause.Value.Label == "" {
		t.Errorf("expected non-empty cause, got zero value")
	}
}

// A PDUSessionResourceReleaseCommand captured on the 001/01 test PLMN.
const pduSessionResourceReleaseCommandCapture = "ABwAMQAABAAKAAIAlABVAAIAAQAmQBUUfgKGQUZ3A34AaAEABS4BBNMAEgEATwAFAAABARA="
