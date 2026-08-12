// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	lib "github.com/ellanetworks/core/ngap"
)

func TestDecodeNGAPMessage_AMFStatusIndication(t *testing.T) {
	const message = "AAFADwAAAQB4AAgAAADxEMr+AA=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcAMFStatusIndication) {
		t.Errorf("expected ProcedureCode=AMFStatusIndication, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcAMFStatusIndication) {
		t.Errorf("procedure code = %d, want %d", ngapMsg.ProcedureCode.Value, lib.ProcAMFStatusIndication)
	}

	if ngapMsg.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", ngapMsg.Criticality)
	}

	if len(ngapMsg.Value.IEs) != 1 {
		t.Errorf("expected 1 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Value != int64(lib.IDUnavailableGUAMIList) {
		t.Errorf("IE id = %d, want %d", item0.ID.Value, lib.IDUnavailableGUAMIList)
	}

	if item0.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item0.Criticality)
	}

	unavailableGuamiList, ok := item0.Value.([]Guami)
	if !ok {
		t.Fatalf("expected UnavailableGUAMIList type, got %T", item0.Value)
	}

	if len(unavailableGuamiList) != 1 {
		t.Errorf("expected 1 unavailable GUAMI item, got %d", len(unavailableGuamiList))
	}

	guami := unavailableGuamiList[0]

	if guami.PLMNID.Mcc != "001" {
		t.Errorf("expected MCC=001, got %s", guami.PLMNID.Mcc)
	}

	if guami.PLMNID.Mnc != "01" {
		t.Errorf("expected MNC=01, got %s", guami.PLMNID.Mnc)
	}

	if guami.AMFRegionID != "ca" {
		t.Errorf("expected AMFRegionID=ca, got %s", guami.AMFRegionID)
	}

	if guami.AMFSetID != "fe0" {
		t.Errorf("expected AMFSetID=fe0, got %s", guami.AMFSetID)
	}

	if guami.AMFPointer != "00" {
		t.Errorf("expected AMFPointer=00, got %s", guami.AMFPointer)
	}
}
