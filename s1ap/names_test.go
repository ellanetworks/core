// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "testing"

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
		{idGlobalENBID, "GlobalENBID (59)"},
		{idCause, "Cause (2)"},
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
		{Cause{Group: CauseGroupRadioNetwork, Value: 0}, "radioNetwork: unspecified (0)"},
		{Cause{Group: CauseGroupNAS, Value: CauseNASDetach}, "nas: detach (2)"},
		// An extension addition's index continues after the root values.
		{Cause{Group: CauseGroupRadioNetwork, Value: 4, Extended: true}, "radioNetwork: n26-interface-not-available (40)"},
		{Cause{Group: CauseGroupRadioNetwork, Value: 99}, "radioNetwork: unknown (99)"},
		{Cause{Group: CauseGroup(9), Value: 1}, "group-9: value-1"},
	} {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

func TestMissingMandatoryIEsErrorMessage(t *testing.T) {
	err := &MissingMandatoryIEsError{
		Procedure: ProcS1Setup,
		IEs: []MissingIE{
			{ID: idGlobalENBID, Criticality: CriticalityReject},
			{ID: idDefaultPagingDRX, Criticality: CriticalityIgnore},
		},
	}

	want := "s1ap: S1Setup (17) missing mandatory IE(s): GlobalENBID (59) (reject), DefaultPagingDRX (137) (ignore)"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	rejected := err.RejectedIEs()
	if len(rejected) != 1 || rejected[0].ID != idGlobalENBID {
		t.Errorf("RejectedIEs() = %v, want only GlobalENBID", rejected)
	}
}
