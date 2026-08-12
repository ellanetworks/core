// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
)

func TestCauseRoundTrip(t *testing.T) {
	for _, c := range []Cause{
		{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified},
		{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkSliceNotSupported},
		{Group: CauseGroupTransport, Value: CauseTransportResourceUnavailable},
		{Group: CauseGroupNAS, Value: CauseNASDeregister},
		{Group: CauseGroupProtocol, Value: CauseProtocolAbstractSyntaxErrorReject},
		{Group: CauseGroupMisc, Value: CauseMiscUnknownPLMNOrSNPN},
		// Extension additions of a group's ENUMERATED continue past its root.
		{Group: CauseGroupRadioNetwork, Value: 0, Extended: true},
		{Group: CauseGroupNAS, Value: 2, Extended: true},
	} {
		w := per.NewWriter()
		if err := c.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("%v: marshal: %v", c, err)
		}

		got, err := unmarshalPERValue[Cause](perBytes(w))
		if err != nil {
			t.Fatalf("%v: unmarshal: %v", c, err)
		}

		if got != c {
			t.Errorf("round trip = %+v, want %+v", got, c)
		}
	}
}

// An out-of-range group cannot be encoded: the CHOICE has five groups plus a
// choice-Extensions alternative this library never selects.
func TestCauseInvalidGroup(t *testing.T) {
	w := per.NewWriter()

	err := Cause{Group: CauseGroup(causeChoiceExtensions)}.MarshalPER(w, per.Aligned)
	if err == nil {
		t.Fatal("marshal accepted the choice-Extensions index as a group")
	}
}

// TS 38.413 §9.3.1.2 closes Cause with choice-Extensions rather than an
// extension marker. A peer selecting it has sent something this version cannot
// represent, so decoding must say so instead of yielding a zero Cause that
// reads as radioNetwork/unspecified.
func TestCauseChoiceExtensionIsRejected(t *testing.T) {
	w := per.NewWriter()
	enc := per.Aligned

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, causeAlternatives-1, causeChoiceExtensions); err != nil {
		t.Fatal(err)
	}

	if err := encodeContainerField(w, enc, ieField{
		id:   IDCause,
		crit: CriticalityIgnore,
		raw:  []byte{0x00},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := unmarshalPERValue[Cause](perBytes(w))
	if err == nil {
		t.Fatalf("decoded choice-Extensions as %+v, want an error", got)
	}

	if !strings.Contains(err.Error(), "unsupported Cause alternative") {
		t.Errorf("error = %q, want it to name the unsupported alternative", err)
	}

	if got != (Cause{}) {
		t.Errorf("cause = %+v, want it left untouched", got)
	}
}

func TestCauseString(t *testing.T) {
	for _, tc := range []struct {
		in   Cause
		want string
	}{
		{Cause{Group: CauseGroupRadioNetwork, Value: 0}, "radioNetwork: unspecified (0)"},
		{Cause{Group: CauseGroupNAS, Value: CauseNASDeregister}, "nas: deregister (2)"},
		// An extension addition's index continues after the root values.
		{Cause{Group: CauseGroupNAS, Value: 0, Extended: true}, "nas: uE-not-in-PLMN-serving-area (4)"},
		{Cause{Group: CauseGroupRadioNetwork, Value: 99}, "radioNetwork: unknown (99)"},
		{Cause{Group: CauseGroup(9), Value: 1}, "group-9: value-1"},
	} {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}
