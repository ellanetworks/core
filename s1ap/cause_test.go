// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

func TestCauseKnownVector(t *testing.T) {
	w := per.NewWriter()

	c := CauseProtocolAbstractSyntaxErrorReject.Cause()
	if err := c.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	if want := []byte{0x31}; !bytes.Equal(perBytes(w), want) {
		t.Fatalf("cause = % x, want % x", perBytes(w), want)
	}
}

func TestCauseRoundTrip(t *testing.T) {
	cases := []Cause{
		CauseRadioNetworkUnspecified.Cause(),
		CauseRadioNetworkX2HandoverTriggered.Cause(),
		CauseRadioNetworkRedirectionTowards1xRTT.Cause(),
		CauseRadioNetworkReleaseDueToDiscontinuousCoverage.Cause(),
		CauseTransportUnspecified.Cause(),
		CauseNASUnspecified.Cause(),
		CauseNASIABNotAuthorized.Cause(),
		CauseProtocolUnspecified.Cause(),
		CauseMiscUnknownPLMN.Cause(),
		CauseMisc(8).Cause(),
	}
	for _, c := range cases {
		w := per.NewWriter()

		if err := c.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("%v: encode: %v", c, err)
		}

		b := perBytes(w)

		got, err := unmarshalPERValue[Cause](b)
		if err != nil {
			t.Fatalf("%v: decode: %v", c, err)
		}

		if !got.Equal(c) {
			t.Fatalf("decoded %v, want %v", got, c)
		}

		if again := ieRaw(t, &got); !bytes.Equal(again, b) {
			t.Fatalf("%v: re-encoded % x, want % x", c, again, b)
		}
	}
}

func TestCauseRejectsNoOrTwoAlternatives(t *testing.T) {
	if err := (&Cause{}).MarshalPER(per.NewWriter(), per.Aligned); err == nil {
		t.Fatal("expected an error for a Cause with no alternative")
	}

	two := Cause{NAS: Ptr(CauseNASDetach), Misc: Ptr(CauseMiscUnspecified)}
	if err := two.MarshalPER(per.NewWriter(), per.Aligned); err == nil {
		t.Fatal("expected an error for a Cause with two alternatives")
	}
}

func TestCauseUnknownExtensionAlternativeRoundTrips(t *testing.T) {
	in := []byte{0x80, 0x01, 0x2a}

	got, err := unmarshalPERValue[Cause](in)
	if err != nil {
		t.Fatal(err)
	}

	if got.Ext == nil || got.Ext.Index != 0 || got.Ext.Value != "\x2a" {
		t.Fatalf("Ext = %+v", got.Ext)
	}

	if !got.Diagnose(&ap.Diagnosis[ProtocolIEID]{}) {
		t.Fatal("an unknown Cause alternative must not be comprehended")
	}

	if out := ieRaw(t, &got); !bytes.Equal(out, in) {
		t.Fatalf("re-encoded % x, want % x", out, in)
	}
}

func TestCauseUnknownExtensionValueRoundTrips(t *testing.T) {
	c := CauseTransport(9).Cause()

	b := ieRaw(t, &c)

	got, err := unmarshalPERValue[Cause](b)
	if err != nil {
		t.Fatal(err)
	}

	if got.Transport == nil || *got.Transport != 9 {
		t.Fatalf("decoded %v", got)
	}

	if !got.Diagnose(&ap.Diagnosis[ProtocolIEID]{}) {
		t.Fatal("an unknown CauseTransport value must not be comprehended")
	}

	if name, idx := got.ValueName(); name != "unknown" || idx != 9 {
		t.Fatalf("ValueName = %q, %d", name, idx)
	}
}

func TestCauseEnumeratedValueContinuesPastTheRoot(t *testing.T) {
	if got := CauseNASDetach.Cause().EnumeratedValue(); got != 2 {
		t.Fatalf("root value = %d, want 2", got)
	}

	if got := CauseRadioNetworkInsufficientUECapabilities.Cause().EnumeratedValue(); got != len(enumCauseRadioNetwork.Root)+5 {
		t.Fatalf("extension value = %d, want %d", got, len(enumCauseRadioNetwork.Root)+5)
	}
}

func TestCauseProtocolConstantsMatchTS36413(t *testing.T) {
	cases := []struct {
		value CauseProtocol
		name  string
	}{
		{CauseProtocolTransferSyntaxError, "transfer-syntax-error"},
		{CauseProtocolAbstractSyntaxErrorReject, "abstract-syntax-error-reject"},
		{CauseProtocolAbstractSyntaxErrorIgnoreAndNotify, "abstract-syntax-error-ignore-and-notify"},
		{CauseProtocolMessageNotCompatible, "message-not-compatible-with-receiver-state"},
		{CauseProtocolSemanticError, "semantic-error"},
		{CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage, "abstract-syntax-error-falsely-constructed-message"},
		{CauseProtocolUnspecified, "unspecified"},
	}

	if len(cases) != len(enumCauseProtocol.Root) {
		t.Fatalf("%d named protocol causes, CauseProtocol has %d root values", len(cases), len(enumCauseProtocol.Root))
	}

	for i, c := range cases {
		if int(c.value) != i {
			t.Errorf("%s = %d, want %d", c.name, c.value, i)
		}

		if got, _ := c.value.Cause().ValueName(); got != c.name {
			t.Errorf("value %d named %q, want %q", c.value, got, c.name)
		}
	}
}

func TestCauseNASIABNotAuthorizedIsRel19ExtensionValue(t *testing.T) {
	if got := CauseNASIABNotAuthorized.Name(); got != "iab-not-authorized" {
		t.Fatalf("Name = %q", got)
	}

	c := CauseNASIABNotAuthorized.Cause()

	if want, got := []byte{0x28, 0x20}, ieRaw(t, &c); !bytes.Equal(got, want) {
		t.Fatalf("cause = % x, want % x", got, want)
	}
}

func TestCauseMessageNotCompatibleVector(t *testing.T) {
	w := per.NewWriter()

	c := CauseProtocolMessageNotCompatible.Cause()
	if err := c.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	if want := []byte{0x33}; !bytes.Equal(perBytes(w), want) {
		t.Fatalf("cause = % x, want % x", perBytes(w), want)
	}
}
