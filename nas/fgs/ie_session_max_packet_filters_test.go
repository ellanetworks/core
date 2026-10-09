// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"testing"
)

// TS 24.501 §9.11.4.9 spreads the count over two octets, bit 8 of the first
// being the most significant and bit 6 of the second the least.
func TestMaxPacketFiltersBitLayout(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []byte
		want uint16
	}{
		{"observed on air", []byte{0x10, 0x00}, 128},
		{"least significant bit", []byte{0x00, 0x20}, 1},
		{"spec maximum", []byte{0x80, 0x00}, 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMaxPacketFilters(tc.in)
			if err != nil || got != tc.want {
				t.Fatalf("got %d (%v), want %d", got, err, tc.want)
			}

			if enc := maxPacketFiltersValue(got); !bytes.Equal(enc, tc.in) {
				t.Fatalf("re-encoded as %x, want %x", enc, tc.in)
			}
		})
	}

	if _, err := parseMaxPacketFilters([]byte{0x10}); err == nil {
		t.Fatal("a one-octet value parsed")
	}
}
