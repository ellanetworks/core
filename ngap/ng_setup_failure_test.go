// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"
)

func TestNGSetupFailureGoldenDecode(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenNGSetupFailure))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	uo, ok := pdu.(*UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != ProcNGSetup {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	fail, err := ParseNGSetupFailure(uo.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := Cause{Group: CauseGroupMisc, Value: CauseMiscUnknownPLMNOrSNPN}
	if deref(fail.Cause) != want {
		t.Errorf("cause = %v, want %v", deref(fail.Cause), want)
	}

	if deref(fail.TimeToWait) != TimeToWaitV10s {
		t.Errorf("timeToWait = %d", deref(fail.TimeToWait))
	}
}

func TestNGSetupFailureCauseOnly(t *testing.T) {
	in := &NGSetupFailure{Cause: Ptr(Cause{Group: CauseGroupProtocol, Value: CauseProtocolAbstractSyntaxErrorReject})}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// An unsuccessfulOutcome PDU begins 40 15 (choice index 2, procedureCode 21).
	if b[0] != 0x40 || b[1] != 0x15 {
		t.Fatalf("envelope prefix = % x, want 40 15", b[:2])
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseNGSetupFailure(pdu.value())
	if err != nil {
		t.Fatal(err)
	}

	if deref(out.Cause) != deref(in.Cause) || out.TimeToWait != nil || out.CriticalityDiagnostics != nil {
		t.Fatalf("got %+v", out)
	}
}

// The Cause IE is mandatory but ignore criticality (§9.2.6.3), so a failure
// that omits it is still delivered — with the field left absent.
func TestNGSetupFailureWithoutCauseIsDelivered(t *testing.T) {
	out, err := ParseNGSetupFailure(container(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.Cause != nil {
		t.Errorf("cause = %v, want nil", out.Cause)
	}

	diag := out.Diagnostics()
	if len(diag.IEs) != 1 || diag.IEs[0].ID != idCause || diag.IEs[0].TypeOfError != TypeOfErrorMissing {
		t.Fatalf("diagnostics = %+v", diag.IEs)
	}
}
