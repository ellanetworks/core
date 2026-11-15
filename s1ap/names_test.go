// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/per"
)

func TestProcedureCodeString(t *testing.T) {
	for _, tc := range []struct {
		in   ProcedureCode
		want string
	}{
		{ProcS1Setup, "S1Setup (17)"},
		{ProcInitialUEMessage, "InitialUEMessage (12)"},
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
		{IDGlobalENBID, "Global-ENB-ID (59)"},
		{IDCause, "Cause (2)"},
		{ProtocolIEID(9999), "ProtocolIEID(9999)"},
	} {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

func TestCauseString(t *testing.T) {
	for _, tc := range []struct {
		in   Cause
		want string
	}{
		{CauseRadioNetworkUnspecified.Cause(), "radioNetwork: unspecified (0)"},
		{CauseNASDetach.Cause(), "nas: detach (2)"},
		// An extension addition's index continues after the root values.
		{CauseRadioNetworkN26InterfaceNotAvailable.Cause(), "radioNetwork: n26-interface-not-available (40)"},
		{CauseRadioNetwork(99).Cause(), "radioNetwork: unknown (99)"},
		{Cause{Ext: &per.ExtensionAlternative{Index: 1}}, "extension-1"},
	} {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

func TestAbstractSyntaxErrorMessage(t *testing.T) {
	err := &AbstractSyntaxError{
		Procedure: ProcS1Setup,
		Trigger:   TriggeringInitiatingMessage,
		Cause:     CauseProtocolAbstractSyntaxErrorReject.Cause(),
		IEs: []CriticalityDiagnosticsIEItem{
			{IEID: IDGlobalENBID, IECriticality: CriticalityReject, TypeOfError: TypeOfErrorMissing},
			{IEID: IDSupportedTAs, IECriticality: CriticalityReject, TypeOfError: TypeOfErrorNotUnderstood},
		},
	}

	want := "s1ap: S1Setup (17): protocol: abstract-syntax-error-reject (1): " +
		"Global-ENB-ID (59) (reject, missing), SupportedTAs (64) (reject, not-understood)"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	d := err.ErrorIndicationDiagnostics()
	if d.ProcedureCode == nil || *d.ProcedureCode != ProcS1Setup ||
		d.TriggeringMessage == nil || *d.TriggeringMessage != TriggeringInitiatingMessage ||
		d.ProcedureCriticality == nil || *d.ProcedureCriticality != CriticalityReject ||
		len(d.IEsCriticalityDiagnostics) != 2 {
		t.Errorf("Diagnostics() = %+v", d)
	}
}
