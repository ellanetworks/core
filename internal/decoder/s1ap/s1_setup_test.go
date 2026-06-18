// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

func TestDecodeS1SetupRequest(t *testing.T) {
	msg := decodeHex(t, "0011002d000004003b00080099f910000019b0003c400a0380737273656e623031004000070000004099f9100089400140")

	if msg.PDUType != "InitiatingMessage" || msg.ProcedureCode.Label != "S1Setup" {
		t.Fatalf("pdu=%q proc=%q", msg.PDUType, msg.ProcedureCode.Label)
	}

	if msg.Summary != "S1 Setup Request (srsenb01)" {
		t.Fatalf("summary = %q", msg.Summary)
	}

	g := mustIE(t, msg, idGlobalENBID).Value.(GlobalENBID)
	if g.PLMNID.Mcc != "999" || g.PLMNID.Mnc != "01" || g.ENBID.Value != 411 || g.ENBID.Kind.Label != "macro" {
		t.Fatalf("Global-ENB-ID = %+v", g)
	}

	if mustIE(t, msg, idENBname).Value != "srsenb01" {
		t.Fatalf("eNBname = %v", mustIE(t, msg, idENBname).Value)
	}

	tas := mustIE(t, msg, idSupportedTAs).Value.([]SupportedTA)
	if len(tas) != 1 || tas[0].TAC != 1 || tas[0].BroadcastPLMNs[0].Mcc != "999" {
		t.Fatalf("SupportedTAs = %+v", tas)
	}

	if mustIE(t, msg, idDefaultPagingDRX).ValueType != "enum" {
		t.Fatal("DefaultPagingDRX not an enum")
	}
}

func TestDecodeS1SetupResponse(t *testing.T) {
	msg := decodeHex(t, "20110021000003003d40060180656c6c610069000b000099f91000000001000100574001ff")

	if msg.PDUType != "SuccessfulOutcome" || msg.ProcedureCode.Label != "S1Setup" {
		t.Fatalf("pdu=%q proc=%q", msg.PDUType, msg.ProcedureCode.Label)
	}

	if mustIE(t, msg, idMMEname).Value != "ella" {
		t.Fatalf("MMEname = %v", mustIE(t, msg, idMMEname).Value)
	}

	list := mustIE(t, msg, idServedGUMMEIs).Value.([]ServedGUMMEI)
	if len(list) != 1 || list[0].ServedPLMNs[0].Mcc != "999" || list[0].ServedGroupIDs[0] != 1 || list[0].ServedMMECodes[0] != 1 {
		t.Fatalf("ServedGUMMEIs = %+v", list)
	}

	if mustIE(t, msg, idRelativeMMECapacity).Value != uint8(255) {
		t.Fatalf("RelativeMMECapacity = %v", mustIE(t, msg, idRelativeMMECapacity).Value)
	}
}

// S1 Setup Failure is the MME's reject response; the deployment never emits one
// (it accepts every eNB), so the example is built with the codec.
func TestDecodeS1SetupFailure(t *testing.T) {
	ttw := s1ap.TimeToWaitV10s
	fail := &s1ap.S1SetupFailure{
		Cause:      s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: 3},
		TimeToWait: &ttw,
	}

	raw, err := fail.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	msg := DecodeS1APMessage(raw)

	if msg.PDUType != "UnsuccessfulOutcome" || msg.ProcedureCode.Label != "S1Setup" {
		t.Fatalf("pdu=%q proc=%q", msg.PDUType, msg.ProcedureCode.Label)
	}

	c := mustIE(t, msg, idCause).Value.(Cause)
	if c.Group.Label != "misc" || c.Value.Value != 3 || c.Value.Label != "om-intervention" {
		t.Fatalf("cause = %+v", c)
	}

	if mustIE(t, msg, idTimeToWait).ValueType != "enum" {
		t.Fatal("TimeToWait not an enum")
	}
}
