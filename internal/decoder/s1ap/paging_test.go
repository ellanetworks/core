// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "testing"

func TestDecodePaging(t *testing.T) {
	msg := decodeHex(t, "000a4027000004005040024b00002b4006001000000001006d400100002e400b00002f40060099f9100001")

	if msg.PDUType != "InitiatingMessage" || msg.ProcedureCode.Label != "Paging" {
		t.Fatalf("pdu=%q proc=%q", msg.PDUType, msg.ProcedureCode.Label)
	}

	idx := mustIE(t, msg, idUEIdentityIndexValue)
	if idx.Value != uint16(300) || idx.ValueType != "integer" {
		t.Fatalf("UEIdentityIndexValue = %v (%s)", idx.Value, idx.ValueType)
	}

	st := mustIE(t, msg, idSTMSI).Value.(STMSI)
	if st.MMEC != 1 || st.MTMSI != 1 {
		t.Fatalf("S-TMSI = %+v", st)
	}

	if mustIE(t, msg, idCNDomain).ValueType != "enum" {
		t.Fatal("CNDomain not an enum")
	}

	tais := mustIE(t, msg, idTAIList).Value.([]TAI)
	if len(tais) != 1 || tais[0].TAC != 1 || tais[0].PLMNID.Mcc != "999" {
		t.Fatalf("TAIList = %+v", tais)
	}
}
