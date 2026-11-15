// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

func TestCauseRoundTrip(t *testing.T) {
	for _, c := range []Cause{
		CauseRadioNetworkUnspecified.Cause(),
		CauseRadioNetworkSliceNotSupported.Cause(),
		CauseTransportResourceUnavailable.Cause(),
		CauseNASDeregister.Cause(),
		CauseProtocolAbstractSyntaxErrorReject.Cause(),
		CauseMiscUnknownPLMNOrSNPN.Cause(),
		CauseRadioNetworkN26InterfaceNotAvailable.Cause(),
		CauseRadioNetworkMultipleAIoTSessionIDInstances.Cause(),
		CauseNASNoAIoTNASResponse.Cause(),
	} {
		w := per.NewWriter()
		if err := c.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("%v: marshal: %v", c, err)
		}

		got, err := unmarshalPERValue[Cause](perBytes(w))
		if err != nil {
			t.Fatalf("%v: unmarshal: %v", c, err)
		}

		if !got.Equal(c) {
			t.Errorf("round trip = %v, want %v", got, c)
		}
	}
}

func TestCauseRejectsZeroOrSeveralAlternatives(t *testing.T) {
	if err := (&Cause{}).MarshalPER(per.NewWriter(), per.Aligned); !errors.Is(err, per.ErrEmpty) {
		t.Errorf("empty cause: err = %v, want per.ErrEmpty", err)
	}

	two := CauseNASDeregister.Cause()
	two.Misc = Ptr(CauseMiscUnspecified)

	if err := two.MarshalPER(per.NewWriter(), per.Aligned); !errors.Is(err, per.ErrChoice) {
		t.Errorf("two alternatives: err = %v, want per.ErrChoice", err)
	}
}

func causeChoiceExtensionWire(t *testing.T, crit Criticality) []byte {
	t.Helper()

	w := per.NewWriter()

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 5, 5); err != nil {
		t.Fatal(err)
	}

	if err := encodeContainerField(w, ieField{ID: 9999, Criticality: crit, Value: apimpl.RawValue([]byte{0x2a})}); err != nil {
		t.Fatal(err)
	}

	return perBytes(w)
}

func TestCauseChoiceExtensionIsPreservedAndDiagnosed(t *testing.T) {
	for _, crit := range []Criticality{CriticalityReject, CriticalityIgnore} {
		wire := causeChoiceExtensionWire(t, crit)

		got, err := unmarshalPERValue[Cause](wire)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}

		if got.ChoiceExtension == nil || got.ChoiceExtension.ID != 9999 || got.ChoiceExtension.Criticality != crit {
			t.Fatalf("ChoiceExtension = %+v, want id 9999 criticality %v", got.ChoiceExtension, crit)
		}

		w := per.NewWriter()
		if err := got.MarshalPER(w, per.Aligned); err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(perBytes(w), wire) {
			t.Errorf("re-encode = % x, want % x", perBytes(w), wire)
		}

		d := ap.Diagnose[ProtocolIEID](&got)

		var found bool

		for _, e := range d.IEs {
			if e.ID == 9999 && e.Criticality == crit {
				found = true
			}
		}

		if !found {
			t.Errorf("diagnostics %+v do not report the unknown choice-Extensions IE on criticality %v", d.IEs, crit)
		}
	}
}

func TestCauseUnknownExtensionValueRoundTrips(t *testing.T) {
	w := per.NewWriter()

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 5, 1); err != nil {
		t.Fatal(err)
	}

	if err := per.EncodeEnumerated(w, per.Aligned, 2, true, 40); err != nil {
		t.Fatal(err)
	}

	wire := perBytes(w)

	got, err := unmarshalPERValue[Cause](wire)
	if err != nil {
		t.Fatal(err)
	}

	if got.Transport == nil || *got.Transport != 40 {
		t.Fatalf("Transport = %v, want 40", got.Transport)
	}

	if got.String() != "transport: unknown(40) (40)" {
		t.Errorf("String() = %q", got.String())
	}

	if !got.Diagnose(&ap.Diagnosis[ProtocolIEID]{}) {
		t.Error("Diagnose() = false for an unknown CauseTransport value")
	}

	w = per.NewWriter()
	if err := got.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(perBytes(w), wire) {
		t.Errorf("re-encode = % x, want % x", perBytes(w), wire)
	}
}

func TestCauseString(t *testing.T) {
	for _, tc := range []struct {
		in   Cause
		want string
	}{
		{CauseRadioNetworkUnspecified.Cause(), "radioNetwork: unspecified (0)"},
		{CauseNASDeregister.Cause(), "nas: deregister (2)"},
		{CauseNASUENotInPLMNServingArea.Cause(), "nas: uE-not-in-PLMN-serving-area (4)"},
		{Cause{}, "empty"},
	} {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

func TestCauseExtensionValuesContinuePastTheRoot(t *testing.T) {
	if got := int(CauseRadioNetworkInsufficientUECapabilities); got != len(enumCauseRadioNetwork.Root)+6 {
		t.Fatalf("insufficient-ue-capabilities = %d, want %d", got, len(enumCauseRadioNetwork.Root)+6)
	}

	if got := CauseRadioNetworkInsufficientUECapabilities.Name(); got != "insufficient-ue-capabilities" {
		t.Errorf("Name() = %q", got)
	}
}
