// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"slices"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestParsedValuesOwnTheirMemory pins the ownership rule the package documents:
// a parsed value never aliases the caller's buffer, so the buffer may be reused
// as soon as Parse returns. The check is on the encoding, since a String method
// can hide the very field that aliased.
//
// UESecurityCapability.Spare is the one that matters most — the bidding-down
// comparison reads it byte-for-byte (TS 33.501 §6.7.2), so a live view of
// attacker octets would defeat it.
func TestParsedValuesOwnTheirMemory(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		parse func([]byte) (encoder, error)
	}{
		{
			"UESecurityCapability",
			[]byte{0xf0, 0xe0, 0xc0, 0x80, 0xaa, 0xbb},
			func(b []byte) (encoder, error) { return ParseUESecurityCapability(b) },
		},
		{
			"GMMCapability",
			[]byte{0x01, 0xaa, 0xbb},
			func(b []byte) (encoder, error) { return ParseGMMCapability(b) },
		},
		{
			"NetworkFeatureSupport",
			[]byte{0x01, 0x02, 0xaa, 0xbb},
			func(b []byte) (encoder, error) { return ParseNetworkFeatureSupport(b) },
		},
		{
			"PSIBitmap",
			[]byte{0x01, 0x02, 0xaa, 0xbb},
			func(b []byte) (encoder, error) { return ParsePSIBitmap(b) },
		},
		{
			"MobileIdentity",
			[]byte{0x07, 0xaa, 0xbb, 0xcc},
			func(b []byte) (encoder, error) { return ParseMobileIdentity(b) },
		},
		{
			"SUCI",
			[]byte{0x01, 0x00, 0xf1, 0x10, 0xf0, 0xff, 0x00, 0x00, 0xaa, 0xbb},
			func(b []byte) (encoder, error) { return ParseSUCI(b) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := slices.Clone(tc.input)

			got, err := tc.parse(buf)
			if err != nil {
				t.Fatalf("parse % x: %v", tc.input, err)
			}

			before, err := got.MarshalBinary()
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			before = slices.Clone(before)

			for i := range buf {
				buf[i] = 0xFF
			}

			after, err := got.MarshalBinary()
			if err != nil {
				t.Fatalf("marshal after overwrite: %v", err)
			}

			if !bytes.Equal(before, after) {
				t.Fatalf("the value aliased the input buffer\n before % x\n after  % x", before, after)
			}
		})
	}
}

// TestMobileIdentityTypeIsDerived confirms the tag cannot disagree with the
// payload: it is computed from whichever variant is set, so a hand-built value
// can no longer report one identity and encode another. The zero value is "no
// identity" and encodes as such.
func TestMobileIdentityTypeIsDerived(t *testing.T) {
	if got := (MobileIdentity{}).Type(); got != IdentityNoIdentity {
		t.Errorf("zero value Type() = %s, want no identity", got)
	}

	raw, err := (MobileIdentity{}).MarshalBinary()
	if err != nil {
		t.Fatalf("the zero value must encode: %v", err)
	}

	back, err := ParseMobileIdentity(raw)
	if err != nil {
		t.Fatal(err)
	}

	if back.Type() != IdentityNoIdentity {
		t.Errorf("zero value round-tripped to %s", back.Type())
	}

	// Every constructor's tag matches its payload.
	guti := GUTIIdentity(GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}})
	if guti.Type() != IdentityGUTI {
		t.Errorf("GUTIIdentity Type() = %s", guti.Type())
	}

	suci := SUCIIdentity(SUCI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}})
	if suci.Type() != IdentitySUCI {
		t.Errorf("SUCIIdentity Type() = %s", suci.Type())
	}

	stmsi := STMSIIdentity(STMSI{})
	if stmsi.Type() != IdentitySTMSI {
		t.Errorf("STMSIIdentity Type() = %s", stmsi.Type())
	}
}
