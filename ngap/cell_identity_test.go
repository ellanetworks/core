// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "testing"

func nodeID(value uint32, bits int) GlobalRANNodeID {
	return GlobalRANNodeID{Kind: RANNodeIDGNB, Value: value, Bits: bits}
}

// TS 23.003 §19.6: the gNB ID occupies the leftmost bits of the 36-bit NCI and
// the cell id the rest. The shift is the node id's own width — the fact a
// caller cannot see, and the one this API exists to keep out of callers.
func TestNRCellIdentityComposition(t *testing.T) {
	tests := []struct {
		gnbID  uint32
		bits   int
		cellID uint64
		want   uint64
	}{
		{0x000102, 24, 0, 0x102000},
		{0x000102, 24, 0xfff, 0x102fff},
		{0xabcdef, 24, 0, 0xabcdef000},
		{0x1a2b3, 20, 1, 0x1a2b30001},
		{0x3fffff, 22, 0, 0xfffffc000},
	}

	for _, tt := range tests {
		got, err := nodeID(tt.gnbID, tt.bits).NRCellIdentity(tt.cellID)
		if err != nil {
			t.Fatalf("gnbID %#x/%d: %v", tt.gnbID, tt.bits, err)
		}

		if got != tt.want {
			t.Errorf("gnbID %#x/%d cell %#x = %#x, want %#x", tt.gnbID, tt.bits, tt.cellID, got, tt.want)
		}

		cell, ok := nodeID(tt.gnbID, tt.bits).SplitNRCellIdentity(got)
		if !ok || cell != tt.cellID {
			t.Errorf("split %#x = (%#x, %v), want (%#x, true)", got, cell, ok, tt.cellID)
		}
	}
}

func TestEUTRACellIdentityComposition(t *testing.T) {
	got, err := GlobalRANNodeID{Kind: RANNodeIDMacroNgENB, Value: 0x00010, Bits: 20}.EUTRACellIdentity(1)
	if err != nil {
		t.Fatal(err)
	}

	if got != 0x1001 {
		t.Errorf("cell identity = %#x, want 0x1001", got)
	}
}

// A node id that fills or overflows the cell identity leaves nothing to
// address a cell with, and a cell id wider than the space left would silently
// land in the node's bits.
func TestCellIdentityRejectsOverflow(t *testing.T) {
	if _, err := nodeID(1, NRCellIdentityBits).NRCellIdentity(0); err == nil {
		t.Error("a node id as wide as the cell identity was accepted")
	}

	if _, err := nodeID(1, 0).NRCellIdentity(0); err == nil {
		t.Error("a zero-width node id was accepted")
	}

	// 24-bit node leaves 12 bits for the cell.
	if _, err := nodeID(0x102, 24).NRCellIdentity(1 << 12); err == nil {
		t.Error("a cell id wider than the space left was accepted")
	}

	if _, err := nodeID(0xffffff, 20).NRCellIdentity(0); err == nil {
		t.Error("a node id exceeding its own bit width was accepted")
	}
}

func TestGlobalRANNodeIDHex(t *testing.T) {
	tests := []struct {
		id   GlobalRANNodeID
		want string
	}{
		{nodeID(0x000102, 24), "000102"},
		{nodeID(0x1a2b3, 20), "1a2b3"},
		{nodeID(0x3fffff, 22), "fffffc"},
	}

	for _, tt := range tests {
		if got := tt.id.Hex(); got != tt.want {
			t.Errorf("Hex() = %q, want %q", got, tt.want)
		}
	}
}
