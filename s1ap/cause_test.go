// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
)

func TestCauseKnownVector(t *testing.T) {
	// protocol (group 3 of 5) / abstract-syntax-error-reject (value 1 of 7):
	// choice ext 0 + idx 011, enum ext 0 + idx 001 => 0 011 0 001 = 0x31.
	w := per.NewWriter()

	if err := (Cause{Group: CauseGroupProtocol, Value: 1}).MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	if want := []byte{0x31}; !bytes.Equal(perBytes(w), want) {
		t.Fatalf("cause = % x, want % x", perBytes(w), want)
	}
}

func TestCauseRoundTrip(t *testing.T) {
	cases := []Cause{
		{CauseGroupRadioNetwork, 0, false},
		{CauseGroupRadioNetwork, 35, false}, // last root value
		{CauseGroupRadioNetwork, 0, true},   // an extension value
		{CauseGroupTransport, 1, false},
		{CauseGroupNAS, 3, false},
		{CauseGroupProtocol, 6, false},
		{CauseGroupMisc, 5, false},
		{CauseGroupMisc, 2, true},
	}
	for _, c := range cases {
		w := per.NewWriter()

		if err := c.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("%+v: encode: %v", c, err)
		}

		got, err := unmarshalPERValue[Cause](perBytes(w))
		if err != nil {
			t.Fatalf("%+v: decode: %v", c, err)
		}

		if got != c {
			t.Fatalf("decoded %+v, want %+v", got, c)
		}
	}
}

func TestCauseInvalidGroup(t *testing.T) {
	w := per.NewWriter()

	if err := (Cause{Group: 9, Value: 0}).MarshalPER(w, per.Aligned); err == nil {
		t.Fatal("expected error for invalid cause group")
	}
}
