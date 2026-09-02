// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"
)

// TS 24.501 §9.11.4.9 spreads the count over two octets, bit 8 of the first
// being the most significant and bit 6 of the second the least.
func TestMaxSupportedPacketFiltersBitLayout(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []byte
		want *uint16
	}{
		{"observed on air", []byte{0x10, 0x00}, ptrUint16(128)},
		{"least significant bit", []byte{0x00, 0x20}, ptrUint16(1)},
		{"spec maximum", []byte{0x80, 0x00}, ptrUint16(1024)},
		{"wrong length", []byte{0x10}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := maxSupportedPacketFilters(tc.in)

			switch {
			case tc.want == nil && got != nil:
				t.Errorf("got %d, want nil", *got)
			case tc.want != nil && got == nil:
				t.Errorf("got nil, want %d", *tc.want)
			case tc.want != nil && *got != *tc.want:
				t.Errorf("got %d, want %d", *got, *tc.want)
			}
		})
	}
}

func ptrUint16(v uint16) *uint16 { return &v }
