// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import "testing"

func TestEnumNames(t *testing.T) {
	names := EnumNames{"ZERO", "", "TWO"}

	for v, want := range map[uint32]string{0: "ZERO", 1: "Type(1)", 2: "TWO", 3: "Type(3)"} {
		if got := names.Name("Type", v); got != want {
			t.Errorf("Name(%d) = %q, want %q", v, got, want)
		}

		if got := names.Has(v); got != (v == 0 || v == 2) {
			t.Errorf("Has(%d) = %v", v, got)
		}
	}
}

func TestBitNames(t *testing.T) {
	for v, want := range map[uint32]string{0: "0", 0x1: "A", 0x3: "A|B", 0x5: "A|0x4"} {
		if got := BitNames(v, "A", "B"); got != want {
			t.Errorf("BitNames(%#x) = %q, want %q", v, got, want)
		}
	}
}
