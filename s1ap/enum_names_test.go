// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/per"
)

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

		{"TriggeringUnsuccessfulOutcome", TriggeringUnsuccessfulOutcome.Name(), "unsuccessfull-outcome"}, //nolint:misspell
		{"TriggeringMessage(9)", TriggeringMessage(9).Name(), "unknown(9)"},

		{"TypeOfErrorNotUnderstood", TypeOfErrorNotUnderstood.Name(), "not-understood"},
		{"TypeOfError(9)", TypeOfError(9).Name(), "unknown(9)"},

		{"CauseNAS", CauseNASDetach.Cause().GroupName(), "nas"},
		{"CauseExt", Cause{Ext: &per.ExtensionAlternative{}}.GroupName(), ""},

		{"PagingDRXv256", PagingDRXv256.Name(), "v256"},
		{"PagingDRX(9)", PagingDRX(9).Name(), "unknown(9)"},

		{"TimeToWaitV10s", TimeToWaitV10s.Name(), "v10s"},
		{"TimeToWait(9)", TimeToWait(9).Name(), "unknown(9)"},

		{"RRCCauseMTAccess", RRCCauseMTAccess.Name(), "mt-Access"},
		{"RRCEstablishmentCause(99)", RRCEstablishmentCause(99).Name(), "unknown(99)"},

		{"HandoverTypeIntraLTE", HandoverTypeIntraLTE.Name(), "intralte"},
		{"HandoverTypeFiveGSToEPS", HandoverTypeFiveGSToEPS.Name(), "fivegs-to-eps"},
		{"HandoverType(99)", HandoverType(99).Name(), "unknown(99)"},

		{"CNDomainPS", CNDomainPS.Name(), "ps"},
		{"CNDomain(9)", CNDomain(9).Name(), "unknown(9)"},

		{"PagingPriorityLevel8", PagingPriorityLevel8.Name(), "priolevel8"},
		{"PagingPriority(99)", PagingPriority(99).Name(), "unknown(99)"},

		{"DirectForwardingPathAvailable", DirectForwardingPathAvailable.Name(), "directPathAvailable"},
		{"DLForwardingProposed", DLForwardingProposed.Name(), "dL-Forwarding-proposed"},
		{"DLForwarding(9)", DLForwarding(9).Name(), "unknown(9)"},
		{"CellSizeVerySmall", CellSizeVerySmall.Name(), "verysmall"},
		{"CellSizeLarge", CellSizeLarge.Name(), "large"},
		{"CellSize(9)", CellSize(9).Name(), "unknown(9)"},
		{"DirectForwardingPathAvailability(9)", DirectForwardingPathAvailability(9).Name(), "unknown(9)"},

		{"UERetentionUesRetained", UERetentionUesRetained.Name(), "ues-retained"},
		{"UERetentionInformation(9)", UERetentionInformation(9).Name(), "unknown(9)"},

		{"ForbiddenInterRATsGERAN", ForbiddenInterRATsGERAN.Name(), "geran"},
		{"ForbiddenInterRATs(9)", ForbiddenInterRATs(9).Name(), "unknown(9)"},

		{"ENBIDShortMacro", ENBID{ShortMacroENBID: Ptr(ShortMacroENBID(1))}.AlternativeName(), "short-macroENB-ID"},
		{"ENBIDExt", ENBID{Ext: &per.ExtensionAlternative{}}.AlternativeName(), ""},
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
