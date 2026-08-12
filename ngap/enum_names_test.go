// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

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
		{"CriticalityNotify", CriticalityNotify.Name(), "notify"},
		{"Criticality(9)", Criticality(9).Name(), ""},

		{"TriggeringInitiatingMessage", TriggeringInitiatingMessage.Name(), "initiating-message"},
		{"TriggeringMessage(9)", TriggeringMessage(9).Name(), ""},

		{"TypeOfErrorMissing", TypeOfErrorMissing.Name(), "missing"},
		{"TypeOfError(9)", TypeOfError(9).Name(), ""},

		{"CauseGroupRadioNetwork", CauseGroupRadioNetwork.Name(), "radioNetwork"},
		{"CauseGroup(9)", CauseGroup(9).Name(), ""},

		{"PagingDRXv128", PagingDRXv128.Name(), "v128"},
		{"PagingDRX(9)", PagingDRX(9).Name(), ""},

		{"TimeToWaitV60s", TimeToWaitV60s.Name(), "v60s"},
		{"TimeToWait(9)", TimeToWait(9).Name(), ""},

		{"RRCCauseMOSignalling", RRCCauseMOSignalling.Name(), "mo-Signalling"},
		{"RRCCauseMCSPriorityAccess", RRCCauseMCSPriorityAccess.Name(), "mcs-PriorityAccess"},
		{"RRCEstablishmentCause(99)", RRCEstablishmentCause(99).Name(), ""},

		{"UEContextRequested", UEContextRequested.Name(), "requested"},
		{"UEContextRequest(9)", UEContextRequest(9).Name(), ""},

		{"UERetentionUesRetained", UERetentionUesRetained.Name(), "ues-retained"},
		{"UERetentionInformation(9)", UERetentionInformation(9).Name(), ""},

		{"PDUSessionTypeIPv4v6", PDUSessionTypeIPv4v6.Name(), "ipv4v6"},
		{"PDUSessionType(9)", PDUSessionType(9).Name(), ""},
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
