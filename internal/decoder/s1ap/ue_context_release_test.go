// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

func TestDecodeUEContextReleaseRequest(t *testing.T) {
	msg := decodeHex(t, "00124016000003000000020008000800034003280002400203a0")

	if msg.PDUType != "InitiatingMessage" || msg.ProcedureCode.Value != int64(s1ap.ProcUEContextReleaseRequest) {
		t.Fatalf("pdu=%q proc=%q", msg.PDUType, msg.ProcedureCode.Label)
	}

	if mustIE(t, msg, s1ap.IDMMEUES1APID).Value != uint32(8) || mustIE(t, msg, s1ap.IDENBUES1APID).Value != uint32(808) {
		t.Fatal("UE id mismatch")
	}

	c := mustIE(t, msg, s1ap.IDCause).Value.(Cause)
	if c.Group.Value != int64(s1ap.CauseGroupRadioNetwork) || c.Value.Value != 29 {
		t.Fatalf("cause = %+v", c)
	}
}

func TestDecodeUEContextReleaseCommand(t *testing.T) {
	msg := decodeHex(t, "0017001200000200630005000b40032b0002400202a0")

	if msg.PDUType != "InitiatingMessage" || msg.ProcedureCode.Value != int64(s1ap.ProcUEContextRelease) {
		t.Fatalf("pdu=%q proc=%q", msg.PDUType, msg.ProcedureCode.Label)
	}

	ids := mustIE(t, msg, s1ap.IDUES1APIDs).Value.(UES1APIDs)
	if ids.MMEUES1APID != 11 || ids.ENBUES1APID != 811 {
		t.Fatalf("UE-S1AP-IDs = %+v", ids)
	}

	c := mustIE(t, msg, s1ap.IDCause).Value.(Cause)
	if c.Group.Value != int64(s1ap.CauseGroupRadioNetwork) || c.Value.Value != int64(s1ap.CauseRadioNetworkRadioConnectionWithUELost) {
		t.Fatalf("cause = %+v", c)
	}
}

func TestDecodeUEContextReleaseComplete(t *testing.T) {
	msg := decodeHex(t, "2017001000000200004002000800084003400328")

	if msg.PDUType != "SuccessfulOutcome" || msg.ProcedureCode.Value != int64(s1ap.ProcUEContextRelease) {
		t.Fatalf("pdu=%q proc=%q", msg.PDUType, msg.ProcedureCode.Label)
	}

	if mustIE(t, msg, s1ap.IDMMEUES1APID).Value != uint32(8) || mustIE(t, msg, s1ap.IDENBUES1APID).Value != uint32(808) {
		t.Fatal("UE id mismatch")
	}
}
