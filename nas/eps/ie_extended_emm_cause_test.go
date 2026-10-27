// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "testing"

func TestExtendedEMMCauseBits(t *testing.T) {
	for _, tc := range []struct {
		nibble uint8
		want   ExtendedEMMCause
	}{
		{0x00, ExtendedEMMCause{}},
		{0x01, ExtendedEMMCause{EUTRANNotAllowed: true}},
		{0x02, ExtendedEMMCause{EPSOptimizationNotSupported: true}},
		{0x04, ExtendedEMMCause{NBIoTNotAllowed: true}},
		{0x07, ExtendedEMMCause{EUTRANNotAllowed: true, EPSOptimizationNotSupported: true, NBIoTNotAllowed: true}},
	} {
		got := ParseExtendedEMMCause(tc.nibble)
		if got != tc.want {
			t.Errorf("ParseExtendedEMMCause(%#x) = %+v, want %+v", tc.nibble, got, tc.want)
		}

		if n := tc.want.Nibble(); n != tc.nibble {
			t.Errorf("%+v.Nibble() = %#x, want %#x", tc.want, n, tc.nibble)
		}
	}
}

func TestExtendedEMMCauseIgnoresSpareBit(t *testing.T) {
	if got := ParseExtendedEMMCause(0x09); got != (ExtendedEMMCause{EUTRANNotAllowed: true}) {
		t.Fatalf("ParseExtendedEMMCause(0x09) = %+v, want only E-UTRAN not allowed", got)
	}

	if n := ParseExtendedEMMCause(0x0F).Nibble(); n != 0x07 {
		t.Fatalf("Nibble() = %#x, want the spare bit cleared (0x07)", n)
	}
}

func TestExtendedEMMCauseString(t *testing.T) {
	got := ExtendedEMMCause{EUTRANNotAllowed: true}.String()
	if want := "EUTRANNotAllowed=true EPSOptimizationNotSupported=false NBIoTNotAllowed=false"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
