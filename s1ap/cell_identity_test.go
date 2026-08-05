// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/per"
)

// TS 36.413 §9.2.1.37: the eNB ID occupies the leftmost bits of the 28-bit ECI
// and the cell id the rest. The shift is the eNB id's own width, which its kind
// fixes — the fact a caller cannot see, and the one this API keeps out of them.
func TestCellIdentityComposition(t *testing.T) {
	tests := []struct {
		kind   ENBIDKind
		enbID  uint32
		cellID uint32
		want   uint32
	}{
		{ENBIDMacro, 0x1a2b3, 1, 0x1a2b301}, // 20-bit macro, 8-bit cell
		{ENBIDMacro, 0x00010, 0, 0x1000},
		{ENBIDHome, 0xabcdef1, 0, 0xabcdef1}, // 28-bit home eNB: the ECI is the eNB id itself
	}

	for _, tt := range tests {
		id := ENBID{Kind: tt.kind, Value: tt.enbID}

		got, err := id.CellIdentity(tt.cellID)
		if err != nil {
			t.Fatalf("kind %d id %#x: %v", tt.kind, tt.enbID, err)
		}

		if got != tt.want {
			t.Errorf("kind %d id %#x cell %#x = %#x, want %#x", tt.kind, tt.enbID, tt.cellID, got, tt.want)
		}

		cell, ok := id.SplitCellIdentity(got)
		if !ok || cell != tt.cellID {
			t.Errorf("split %#x = (%#x, %v), want (%#x, true)", got, cell, ok, tt.cellID)
		}
	}
}

func TestCellIdentityRejectsOverflow(t *testing.T) {
	macro := ENBID{Kind: ENBIDMacro, Value: 0x1a2b3}

	// A 20-bit macro id leaves 8 bits for the cell.
	if _, err := macro.CellIdentity(1 << 8); err == nil {
		t.Error("a cell id wider than the space left was accepted")
	}

	if _, err := (ENBID{Kind: ENBIDMacro, Value: 0xffffff}).CellIdentity(0); err == nil {
		t.Error("an eNB id exceeding its own bit width was accepted")
	}

	if _, err := (ENBID{Kind: 99, Value: 1}).CellIdentity(0); err == nil {
		t.Error("an unknown eNB id kind was accepted")
	}

	// A home eNB ID is itself the 28-bit cell identity, so only cell 0 exists.
	if _, err := (ENBID{Kind: ENBIDHome, Value: 0xabcdef1}).CellIdentity(1); err == nil {
		t.Error("a non-zero cell id on a home eNB was accepted")
	}
}

func TestENBIDHex(t *testing.T) {
	if got := (ENBID{Kind: ENBIDMacro, Value: 0x1a2b3}).Hex(); got != "1a2b3" {
		t.Errorf("Hex() = %q, want 1a2b3", got)
	}
}

// CellIdentity ::= BIT STRING (SIZE (28)) (TS 36.413 §9.2.1.38), so a wider
// value has no encoding: it is refused instead of losing its high bits.
func TestEUTRANCGIRejectsOverWideCellIdentity(t *testing.T) {
	plmn := PLMNIdentity{0x00, 0xf1, 0x10}

	if err := (EUTRANCGI{PLMNIdentity: plmn, CellID: 0xfffffff}).MarshalPER(per.NewWriter(), per.Aligned); err != nil {
		t.Fatalf("the widest 28-bit cell identity must encode: %v", err)
	}

	if err := (EUTRANCGI{PLMNIdentity: plmn, CellID: 1 << 28}).MarshalPER(per.NewWriter(), per.Aligned); err == nil {
		t.Fatal("a 29-bit cell identity encoded, want an error")
	}
}
