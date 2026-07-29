// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"slices"
	"testing"
)

// TestParsedValuesOwnTheirMemory pins the ownership rule the package documents:
// a parsed value never aliases the caller's buffer, so the buffer may be reused
// as soon as Parse returns. The check is on the encoding, since a String method
// can hide the very field that aliased.
//
// UENetworkCapability.Rest is the one that matters most — the MME replays it for
// the UE's bidding-down check (TS 33.401 §7.2.4.4).
func TestParsedValuesOwnTheirMemory(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		parse func([]byte) (encoder, error)
	}{
		{
			"UENetworkCapability",
			[]byte{0xf0, 0x70, 0xc0, 0x40, 0xaa, 0xbb},
			func(b []byte) (encoder, error) { return ParseUENetworkCapability(b) },
		},
		{
			"UESecurityCapability",
			[]byte{0xf0, 0x70, 0xc0, 0x40},
			func(b []byte) (encoder, error) { return ParseUESecurityCapability(b) },
		},
		{
			"MSNetworkCapability",
			[]byte{0xe5, 0xe0, 0x34},
			func(b []byte) (encoder, error) { return ParseMSNetworkCapability(b) },
		},
		{
			"NetworkFeatureSupport",
			[]byte{0x01, 0x02, 0xaa},
			func(b []byte) (encoder, error) { return ParseNetworkFeatureSupport(b) },
		},
		{
			"MobileIdentity",
			[]byte{0xf5, 0xaa, 0xbb, 0xcc, 0xdd},
			func(b []byte) (encoder, error) { return ParseMobileIdentity(b) },
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
