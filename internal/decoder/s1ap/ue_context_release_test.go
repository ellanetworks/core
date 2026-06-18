// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "testing"

func TestDecodeUEContextReleaseRequest(t *testing.T) {
	msg := decodeHex(t, "00124016000003000000020008000800034003280002400203a0")

	if msg.PDUType != "InitiatingMessage" || msg.ProcedureCode.Label != "UEContextReleaseRequest" {
		t.Fatalf("pdu=%q proc=%q", msg.PDUType, msg.ProcedureCode.Label)
	}

	if mustIE(t, msg, idMMEUES1APID).Value != uint32(8) || mustIE(t, msg, idENBUES1APID).Value != uint32(808) {
		t.Fatal("UE id mismatch")
	}

	c := mustIE(t, msg, idCause).Value.(Cause)
	if c.Group.Label != "radioNetwork" || c.Value.Value != 29 || c.Value.Label != "interaction-with-other-procedure" {
		t.Fatalf("cause = %+v", c)
	}
}

func TestDecodeUEContextReleaseCommand(t *testing.T) {
	msg := decodeHex(t, "0017001200000200630005000b40032b0002400202a0")

	if msg.PDUType != "InitiatingMessage" || msg.ProcedureCode.Label != "UEContextRelease" {
		t.Fatalf("pdu=%q proc=%q", msg.PDUType, msg.ProcedureCode.Label)
	}

	ids := mustIE(t, msg, idUES1APIDs).Value.(UES1APIDs)
	if ids.MMEUES1APID != 11 || ids.ENBUES1APID != 811 {
		t.Fatalf("UE-S1AP-IDs = %+v", ids)
	}

	c := mustIE(t, msg, idCause).Value.(Cause)
	if c.Group.Label != "radioNetwork" || c.Value.Value != 21 || c.Value.Label != "radio-connection-with-ue-lost" {
		t.Fatalf("cause = %+v", c)
	}
}

func TestDecodeUEContextReleaseComplete(t *testing.T) {
	msg := decodeHex(t, "2017001000000200004002000800084003400328")

	if msg.PDUType != "SuccessfulOutcome" || msg.ProcedureCode.Label != "UEContextRelease" {
		t.Fatalf("pdu=%q proc=%q", msg.PDUType, msg.ProcedureCode.Label)
	}

	if mustIE(t, msg, idMMEUES1APID).Value != uint32(8) || mustIE(t, msg, idENBUES1APID).Value != uint32(808) {
		t.Fatal("UE id mismatch")
	}
}
