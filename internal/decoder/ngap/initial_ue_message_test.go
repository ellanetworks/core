// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/decoder/utils"
	lib "github.com/ellanetworks/core/ngap"
)

func TestDecodeNGAPMessage_InitialUEMessage(t *testing.T) {
	raw, err := decodeB64(initialUEMessageCapture)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	expectedSummary := "InitialUEMessage, RAN-UE=1, NAS=REGISTRATION REQUEST"
	if ngapMsg.Summary != expectedSummary {
		t.Errorf("expected Summary=%q, got %q", expectedSummary, ngapMsg.Summary)
	}

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcInitialUEMessage) {
		t.Errorf("expected ProcedureCode=InitialUEMessage, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcInitialUEMessage) {
		t.Errorf("procedure code = %d, want %d", ngapMsg.ProcedureCode.Value, lib.ProcInitialUEMessage)
	}

	if ngapMsg.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", ngapMsg.Criticality)
	}

	if len(ngapMsg.Value.IEs) != 5 {
		t.Errorf("expected 5 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Value != int64(lib.IDRANUENGAPID) {
		t.Errorf("IE id = %d, want %d", item0.ID.Value, lib.IDRANUENGAPID)
	}

	if item0.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item0.Criticality)
	}

	ranUENGAPID, ok := item0.Value.(int64)
	if !ok {
		t.Fatalf("expected RAN-UE-NGAP-ID to be of type int64, got %T", item0.Value)
	}

	if ranUENGAPID != 1 {
		t.Errorf("expected RAN-UE-NGAP-ID=1, got %d", ranUENGAPID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Value != int64(lib.IDNASPDU) {
		t.Errorf("IE id = %d, want %d", item1.ID.Value, lib.IDNASPDU)
	}

	if item1.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item1.Criticality)
	}

	nasPdu, ok := item1.Value.(NASPDU)
	if !ok {
		t.Fatalf("expected NAS-PDU to be of type NAS-PDU, got %T", item1.Value)
	}

	expectedNASPDU := "fgBBeQANAQDxEAAAAABEdGhXJS4E8PDw8A=="

	expectedNASPDUraw, err := decodeB64(expectedNASPDU)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	expectedHex := hex.EncodeToString(expectedNASPDUraw)
	if nasPdu.RawHex != expectedHex {
		t.Errorf("expected RawHex=%s, got %s", expectedHex, nasPdu.RawHex)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Value != int64(lib.IDUserLocationInformation) {
		t.Errorf("IE id = %d, want %d", item2.ID.Value, lib.IDUserLocationInformation)
	}

	if item2.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item2.Criticality)
	}

	userLocationInfo, ok := item2.Value.(UserLocationInformation)
	if !ok {
		t.Fatalf("expected UserLocationInformation to be of type UserLocationInformation, got %T", item2.Value)
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

	if *userLocationInfo.NR.TimeStamp != "2025-10-14T20:47:06Z" {
		t.Errorf("expected TimeStamp=2025-10-14T20:47:06Z, got %s", *userLocationInfo.NR.TimeStamp)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Value != int64(lib.IDRRCEstablishmentCause) {
		t.Errorf("IE id = %d, want %d", item3.ID.Value, lib.IDRRCEstablishmentCause)
	}

	if item3.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item3.Criticality)
	}

	rrcEstabCause, ok := item3.Value.(utils.EnumField)
	if !ok {
		t.Fatalf("expected RRCEstablishmentCause to be of type EnumField, got %T", item3.Value)
	}

	if rrcEstabCause.Value != int64(lib.RRCCauseMOSignalling) {
		t.Errorf("expected RRCEstablishmentCause=mo-Signalling, got %s", rrcEstabCause.Label)
	}

	if rrcEstabCause.Value != int64(lib.RRCCauseMOSignalling) {
		t.Errorf("expected RRCEstablishmentCause value=3, got %d", rrcEstabCause.Value)
	}

	item4 := ngapMsg.Value.IEs[4]

	if item4.ID.Value != int64(lib.IDUEContextRequest) {
		t.Errorf("IE id = %d, want %d", item4.ID.Value, lib.IDUEContextRequest)
	}

	if item4.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item4.Criticality)
	}

	ueContextRequest, ok := item4.Value.(utils.EnumField)
	if !ok {
		t.Fatalf("expected UEContextRequest to be of type EnumField, got %T", item4.Value)
	}

	if ueContextRequest.Value != int64(lib.UEContextRequested) {
		t.Errorf("expected UEContextRequest=Requested, got %v", ueContextRequest.Label)
	}

	if ueContextRequest.Value != int64(lib.UEContextRequested) {
		t.Errorf("expected UEContextRequest value=0, got %d", ueContextRequest.Value)
	}
}

// An InitialUEMessage captured on the 001/01 test PLMN.
const initialUEMessageCapture = "AA9ASAAABQBVAAIAAQAmABoZfgBBeQANAQDxEAAAAABEdGhXJS4E8PDw8AB5ABNQAPEQAAAAAQAA8RAAAAHsmTVKAFpAARgAcEABAA=="
