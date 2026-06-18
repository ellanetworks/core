// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "testing"

func TestS1SetupFailureRoundTrip(t *testing.T) {
	ttw := TimeToWaitV10s

	in := &S1SetupFailure{
		Cause:      Cause{Group: CauseGroupMisc, Value: 4}, // misc / unspecified
		TimeToWait: &ttw,
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// An unsuccessfulOutcome PDU begins 40 11 (choice index 2, procedureCode 17).
	if b[0] != 0x40 || b[1] != 0x11 {
		t.Fatalf("envelope prefix = % x, want 40 11", b[:2])
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := pdu.(*UnsuccessfulOutcome); !ok {
		t.Fatalf("got %T, want *UnsuccessfulOutcome", pdu)
	}

	out, err := ParseS1SetupFailure(pdu.value())
	if err != nil {
		t.Fatal(err)
	}

	if out.Cause != in.Cause {
		t.Fatalf("cause = %+v, want %+v", out.Cause, in.Cause)
	}

	if out.TimeToWait == nil || *out.TimeToWait != ttw {
		t.Fatalf("timeToWait = %v", out.TimeToWait)
	}
}

func TestS1SetupFailureCauseOnly(t *testing.T) {
	in := &S1SetupFailure{Cause: Cause{Group: CauseGroupProtocol, Value: 1}}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseS1SetupFailure(pdu.value())
	if err != nil {
		t.Fatal(err)
	}

	if out.Cause != in.Cause || out.TimeToWait != nil || out.CriticalityDiagnostics != nil {
		t.Fatalf("got %+v", out)
	}
}
