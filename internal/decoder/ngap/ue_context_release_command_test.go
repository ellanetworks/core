// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	lib "github.com/ellanetworks/core/ngap"
)

func TestDecodeNGAPMessage_UEContextReleaseCommand(t *testing.T) {
	const message = "ACkAEQAAAgByAAQAGgAaAA9AAgUA"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcUEContextRelease) {
		t.Errorf("expected ProcedureCode=UEContextRelease, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcUEContextRelease) {
		t.Errorf("procedure code = %d, want %d", ngapMsg.ProcedureCode.Value, lib.ProcUEContextRelease)
	}

	if ngapMsg.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", ngapMsg.Criticality)
	}

	if len(ngapMsg.Value.IEs) != 2 {
		t.Errorf("expected 2 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Value != int64(lib.IDUENGAPIDs) {
		t.Errorf("IE id = %d, want %d", item0.ID.Value, lib.IDUENGAPIDs)
	}

	if item0.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item0.Criticality)
	}

	ueNgapIDs, ok := item0.Value.(UENGAPIDs)
	if !ok {
		t.Fatalf("expected UE-NGAP-IDs value type=UE-NGAP-IDs, got %T", item0.Value)
	}

	if ueNgapIDs.AMFUENGAPID != 0 {
		t.Errorf("expected AMF-UE-NGAP-ID=0, got %d", ueNgapIDs.AMFUENGAPID)
	}

	if ueNgapIDs.UENGAPIDPair.AMFUENGAPID != 26 {
		t.Errorf("expected UENGAPIDPair.AMF-UE-NGAP-ID=26, got %d", ueNgapIDs.UENGAPIDPair.AMFUENGAPID)
	}

	if ueNgapIDs.UENGAPIDPair.RANUENGAPID != 26 {
		t.Errorf("expected UENGAPIDPair.RAN-UE-NGAP-ID=26, got %d", ueNgapIDs.UENGAPIDPair.RANUENGAPID)
	}
}
