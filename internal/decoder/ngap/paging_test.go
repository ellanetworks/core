// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	lib "github.com/ellanetworks/core/ngap"
)

func TestDecodeNGAPMessage_Paging(t *testing.T) {
	const message = "ABhAGQAAAgBzQAcfwAAAAAABAGdABwAA8RAAAAE="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcPaging) {
		t.Errorf("expected ProcedureCode=Paging, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcPaging) {
		t.Errorf("procedure code = %d, want %d", ngapMsg.ProcedureCode.Value, lib.ProcPaging)
	}

	if ngapMsg.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", ngapMsg.Criticality)
	}

	if len(ngapMsg.Value.IEs) != 2 {
		t.Errorf("expected 2 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Value != int64(lib.IDUEPagingIdentity) {
		t.Errorf("IE id = %d, want %d", item0.ID.Value, lib.IDUEPagingIdentity)
	}

	if item0.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item0.Criticality)
	}

	pagingID, ok := item0.Value.(UEPagingIdentity)
	if !ok {
		t.Fatalf("expected UEPagingIdentity type, got %T", item0.Value)
	}

	if pagingID.FiveGSTMSI.AMFSetID != "fe0" {
		t.Errorf("expected FiveGSType=fe0, got %s", pagingID.FiveGSTMSI.AMFSetID)
	}

	if pagingID.FiveGSTMSI.AMFPointer != "00" {
		t.Errorf("expected AMFPointer=00, got %v", pagingID.FiveGSTMSI.AMFPointer)
	}

	if pagingID.FiveGSTMSI.FiveGTMSI != "00000001" {
		t.Errorf("expected TMSI=00000001, got %s", pagingID.FiveGSTMSI.FiveGTMSI)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Value != int64(lib.IDTAIListForPaging) {
		t.Errorf("IE id = %d, want %d", item1.ID.Value, lib.IDTAIListForPaging)
	}

	if item1.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item1.Criticality)
	}

	_, ok = item1.Value.([]TAI)
	if !ok {
		t.Fatalf("expected TAIListForPaging type, got %T", item1.Value)
	}

	if len(item1.Value.([]TAI)) != 1 {
		t.Errorf("expected 1 TAI, got %d", len(item1.Value.([]TAI)))
	}

	if item1.Value.([]TAI)[0].PLMNID.Mcc != "001" {
		t.Errorf("expected MCC=001, got %s", item1.Value.([]TAI)[0].PLMNID.Mcc)
	}

	if item1.Value.([]TAI)[0].PLMNID.Mnc != "01" {
		t.Errorf("expected MNC=01, got %s", item1.Value.([]TAI)[0].PLMNID.Mnc)
	}

	if item1.Value.([]TAI)[0].TAC != "000001" {
		t.Errorf("expected TAC=000001, got %s", item1.Value.([]TAI)[0].TAC)
	}
}
