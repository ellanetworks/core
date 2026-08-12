// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "testing"

// Name is what the diagnostic decoders render, so an assigned value names
// itself and an unassigned one reports nothing rather than a neighbour's name.
func TestEnumNames(t *testing.T) {
	for _, tc := range []struct {
		what string
		got  string
		want string
	}{
		{"CriticalityReject", CriticalityReject.Name(), "reject"},
		{"Criticality(9)", Criticality(9).Name(), ""},

		{"TriggeringUnsuccessfulOutcome", TriggeringUnsuccessfulOutcome.Name(), "unsuccessful-outcome"},
		{"TriggeringMessage(9)", TriggeringMessage(9).Name(), ""},

		{"TypeOfErrorNotUnderstood", TypeOfErrorNotUnderstood.Name(), "not-understood"},
		{"TypeOfError(9)", TypeOfError(9).Name(), ""},

		{"CauseGroupNAS", CauseGroupNAS.Name(), "nas"},
		{"CauseGroup(9)", CauseGroup(9).Name(), ""},

		{"PagingDRXv256", PagingDRXv256.Name(), "v256"},
		{"PagingDRX(9)", PagingDRX(9).Name(), ""},

		{"TimeToWaitV10s", TimeToWaitV10s.Name(), "v10s"},
		{"TimeToWait(9)", TimeToWait(9).Name(), ""},

		{"RRCCauseMTAccess", RRCCauseMTAccess.Name(), "mt-Access"},
		{"RRCEstablishmentCause(99)", RRCEstablishmentCause(99).Name(), ""},

		{"HandoverTypeIntraLTE", HandoverTypeIntraLTE.Name(), "intralte"},
		{"HandoverTypeFiveGSToEPS", HandoverTypeFiveGSToEPS.Name(), "fivegs-to-eps"},
		{"HandoverType(99)", HandoverType(99).Name(), ""},

		{"CNDomainPS", CNDomainPS.Name(), "ps"},
		{"CNDomain(9)", CNDomain(9).Name(), ""},

		{"ENBIDShortMacro", ENBIDShortMacro.Name(), "short-macroENB-ID"},
		{"ENBIDKind(9)", ENBIDKind(9).Name(), ""},
	} {
		if tc.got != tc.want {
			t.Errorf("%s.Name() = %q, want %q", tc.what, tc.got, tc.want)
		}
	}
}

// The decorated String forms the errors carry are built on the same tables.
func TestEnumStringUsesTheNames(t *testing.T) {
	if got := CriticalityIgnore.String(); got != CriticalityIgnore.Name() {
		t.Errorf("Criticality String() = %q, Name() = %q", got, CriticalityIgnore.Name())
	}

	if got := TypeOfErrorMissing.String(); got != TypeOfErrorMissing.Name() {
		t.Errorf("TypeOfError String() = %q, Name() = %q", got, TypeOfErrorMissing.Name())
	}

	if got := Criticality(9).String(); got != "Criticality(9)" {
		t.Errorf("unassigned Criticality String() = %q", got)
	}
}
