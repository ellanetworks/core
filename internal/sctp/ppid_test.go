// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"encoding/binary"
	"testing"
)

// TestPPIDWireOrder verifies that PPIDWireOrder encodes a logical PPID so that
// the verbatim-written sinfo_ppid lands big-endian on the wire (TS 36.412 §7,
// TS 38.412 §7), and that the conversion is symmetric.
func TestPPIDWireOrder(t *testing.T) {
	for _, ppid := range []uint32{18, 60} {
		wire := PPIDWireOrder(ppid)

		// The socket layer writes sinfo_ppid verbatim, so the in-memory bytes of
		// wire are the wire bytes; they must decode big-endian to the logical PPID.
		var b [4]byte
		binary.NativeEndian.PutUint32(b[:], wire)

		if got := binary.BigEndian.Uint32(b[:]); got != ppid {
			t.Errorf("PPIDWireOrder(%d): wire decodes big-endian to %d, want %d", ppid, got, ppid)
		}

		if back := PPIDWireOrder(wire); back != ppid {
			t.Errorf("PPIDWireOrder not symmetric for %d: got %d", ppid, back)
		}
	}
}
