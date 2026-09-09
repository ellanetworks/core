// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

func marshalHex(t *testing.T, m interface{ Marshal() ([]byte, error) }) string {
	t.Helper()

	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return hex.EncodeToString(b)
}

func TestDecodeMMEConfigurationUpdate(t *testing.T) {
	msg := decodeHex(t, marshalHex(t, &s1ap.MMEConfigurationUpdate{
		MMEName:             s1ap.Ptr("ella"),
		RelativeMMECapacity: s1ap.Ptr(uint8(0)),
	}))

	if msg.PDUType != "InitiatingMessage" || msg.ProcedureCode.Value != int64(s1ap.ProcMMEConfigurationUpdate) {
		t.Fatalf("pdu=%q proc=%q", msg.PDUType, msg.ProcedureCode.Label)
	}

	if msg.Summary != "MME Configuration Update (capacity 0)" {
		t.Fatalf("summary = %q", msg.Summary)
	}

	if mustIE(t, msg, s1ap.IDMMEname).Value != "ella" {
		t.Fatalf("MMEname = %v", mustIE(t, msg, s1ap.IDMMEname).Value)
	}

	if got := mustIE(t, msg, s1ap.IDRelativeMMECapacity).Value; got != uint8(0) {
		t.Fatalf("RelativeMMECapacity = %v", got)
	}
}

func TestDecodeMMEConfigurationUpdateAcknowledge(t *testing.T) {
	msg := decodeHex(t, marshalHex(t, &s1ap.MMEConfigurationUpdateAcknowledge{}))

	if msg.PDUType != "SuccessfulOutcome" || msg.ProcedureCode.Value != int64(s1ap.ProcMMEConfigurationUpdate) {
		t.Fatalf("pdu=%q proc=%q", msg.PDUType, msg.ProcedureCode.Label)
	}

	if msg.Summary != "MME Configuration Update Acknowledge" {
		t.Fatalf("summary = %q", msg.Summary)
	}
}

func TestDecodeMMEConfigurationUpdateFailure(t *testing.T) {
	msg := decodeHex(t, marshalHex(t, &s1ap.MMEConfigurationUpdateFailure{
		Cause: s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: s1ap.CauseProtocolSemanticError}),
	}))

	if msg.PDUType != "UnsuccessfulOutcome" || msg.ProcedureCode.Value != int64(s1ap.ProcMMEConfigurationUpdate) {
		t.Fatalf("pdu=%q proc=%q", msg.PDUType, msg.ProcedureCode.Label)
	}

	if msg.Summary != "MME Configuration Update Failure" {
		t.Fatalf("summary = %q", msg.Summary)
	}

	if _, ok := findIE(msg.Value.IEs, s1ap.IDCause); !ok {
		t.Fatal("Cause IE missing")
	}
}
