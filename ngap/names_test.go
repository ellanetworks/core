// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "testing"

func TestProcedureCodeString(t *testing.T) {
	for _, tc := range []struct {
		in   ProcedureCode
		want string
	}{
		{ProcNGSetup, "NGSetup (21)"},
		{ProcInitialUEMessage, "InitialUEMessage (15)"},
		{ProcedureCode(200), "ProcedureCode(200)"},
	} {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

func TestProtocolIEIDString(t *testing.T) {
	for _, tc := range []struct {
		in   ProtocolIEID
		want string
	}{
		{IDGlobalRANNodeID, "GlobalRANNodeID (27)"},
		{IDCause, "Cause (15)"},
		{ProtocolIEID(9999), "ProtocolIEID(9999)"},
	} {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

// Criticality Diagnostics reports the procedure's real criticality (§9.3.1.3).
// TS 38.413 splits the 81 procedures 41 reject / 40 ignore, so a hardcoded
// value would be wrong about half the time.
func TestProcedureCriticalityTable(t *testing.T) {
	if got := len(procedureInfos); got != 81 {
		t.Errorf("procedureInfos has %d entries, want 81", got)
	}

	var reject, ignore, outcomes int

	for _, info := range procedureInfos {
		switch info.crit {
		case CriticalityReject:
			reject++
		case CriticalityIgnore:
			ignore++
		case CriticalityNotify:
			t.Error("a procedure has notify criticality; TS 38.413 assigns none")
		}

		if info.outcome {
			outcomes++
		}
	}

	if reject != 41 || ignore != 40 {
		t.Errorf("criticality split = %d reject / %d ignore, want 41 / 40", reject, ignore)
	}

	if outcomes != 18 {
		t.Errorf("%d procedures define an unsuccessful outcome, want 18", outcomes)
	}
}

func TestNGSetupProcedureFacts(t *testing.T) {
	if got := ProcedureCriticality(ProcNGSetup); got != CriticalityReject {
		t.Errorf("NG Setup criticality = %v, want reject", got)
	}

	if !hasUnsuccessfulOutcome(ProcNGSetup) {
		t.Error("NG Setup should define an unsuccessful outcome")
	}

	// A procedure with no unsuccessful outcome falls back to Error Indication.
	if hasUnsuccessfulOutcome(ProcErrorIndication) {
		t.Error("Error Indication should not define an unsuccessful outcome")
	}
}

func TestAbstractSyntaxErrorMessage(t *testing.T) {
	err := &AbstractSyntaxError{
		Procedure: ProcNGSetup,
		Trigger:   TriggeringInitiatingMessage,
		Cause:     Cause{Group: CauseGroupProtocol, Value: CauseProtocolAbstractSyntaxErrorReject},
		IEs: []CriticalityDiagnosticsIEItem{
			{IEID: IDGlobalRANNodeID, IECriticality: CriticalityReject, TypeOfError: TypeOfErrorMissing},
			{IEID: IDSupportedTAList, IECriticality: CriticalityReject, TypeOfError: TypeOfErrorNotUnderstood},
		},
	}

	want := "ngap: NGSetup (21): protocol: abstract-syntax-error-reject (1): " +
		"GlobalRANNodeID (27) (reject, missing), SupportedTAList (102) (reject, not-understood)"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	d := err.ErrorIndicationDiagnostics()
	if d.ProcedureCode == nil || *d.ProcedureCode != ProcNGSetup ||
		d.TriggeringMessage == nil || *d.TriggeringMessage != TriggeringInitiatingMessage ||
		d.ProcedureCriticality == nil || *d.ProcedureCriticality != CriticalityReject ||
		len(d.IEsCriticalityDiagnostics) != 2 {
		t.Errorf("Diagnostics() = %+v", d)
	}
}
