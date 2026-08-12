// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"testing"

	lib "github.com/ellanetworks/core/ngap"
)

func TestDecodeNGAPMessage_UplinkNASTransport(t *testing.T) {
	const message = "AC5APwAABAAKAAIAAQBVAAIAAQAmABUUfgLpGbfKA34AZwEABS4BANZREgEAeUATUADxEAAAAAEAAPEQAAAB7JlGUQ=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcUplinkNASTransport) {
		t.Errorf("expected ProcedureCode=UplinkNASTransport, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcUplinkNASTransport) {
		t.Errorf("procedure code = %d, want %d", ngapMsg.ProcedureCode.Value, lib.ProcUplinkNASTransport)
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

	if item2.ID.Value != int64(lib.IDNASPDU) {
		t.Errorf("IE id = %d, want %d", item2.ID.Value, lib.IDNASPDU)
	}

	if item2.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item2.Criticality)
	}

	nasPdu, ok := item2.Value.(NASPDU)
	if !ok {
		t.Fatalf("expected NAS-PDU to be of type NAS-PDU, got %T", item2.Value)
	}

	if nasPdu.Protocol != "NAS" {
		t.Errorf("expected Protocol=NAS, got %s", nasPdu.Protocol)
	}

	expectedNASPDU := "fgLpGbfKA34AZwEABS4BANZREgE="

	expectedNASPDUraw, err := decodeB64(expectedNASPDU)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	expectedHex := hex.EncodeToString(expectedNASPDUraw)
	if nasPdu.RawHex != expectedHex {
		t.Errorf("expected RawHex=%s, got %s", expectedHex, nasPdu.RawHex)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Value != int64(lib.IDUserLocationInformation) {
		t.Errorf("IE id = %d, want %d", item3.ID.Value, lib.IDUserLocationInformation)
	}

	if item3.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item3.Criticality)
	}

	userLocationInfo, ok := item3.Value.(UserLocationInformation)
	if !ok {
		t.Fatalf("expected UserLocationInformation to be of type UserLocationInformation, got %T", item3.Value)
	}

	if userLocationInfo.NR == nil {
		t.Fatalf("expected NR, got nil")
	}

	if userLocationInfo.NR.TAI.TAC != "000001" {
		t.Errorf("expected TAC=000001, got %s", userLocationInfo.NR.TAI.TAC)
	}

	if userLocationInfo.NR.TAI.PLMNID.Mcc != "001" {
		t.Errorf("expected PLMNID.Mcc=001, got %s", userLocationInfo.NR.TAI.PLMNID.Mcc)
	}

	if userLocationInfo.NR.TAI.PLMNID.Mnc != "01" {
		t.Errorf("expected PLMNID.Mnc=01, got %s", userLocationInfo.NR.TAI.PLMNID.Mnc)
	}

	if userLocationInfo.NR.TimeStamp == nil {
		t.Fatalf("expected TimeStamp, got nil")
	}

	if *userLocationInfo.NR.TimeStamp != "2025-10-14T21:59:45Z" {
		t.Errorf("expected TimeStamp=2025-10-14T21:59:45Z, got %s", *userLocationInfo.NR.TimeStamp)
	}
}
