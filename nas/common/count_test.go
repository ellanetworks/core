// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import "testing"

func TestCountLayout(t *testing.T) {
	c := MakeCount(0x0102, 0x03)

	if c.Overflow() != 0x0102 {
		t.Errorf("Overflow = %#x, want 0x0102", c.Overflow())
	}

	if c.SQN() != 0x03 {
		t.Errorf("SQN = %#x, want 0x03", c.SQN())
	}

	if c.Value() != 0x00010203 {
		t.Errorf("Value = %#x, want 0x00010203", c.Value())
	}
}

func TestCountValueMasksTo24Bits(t *testing.T) {
	// A count carrying junk in the top 8 bits still yields a 24-bit crypto input.
	c := Count(0xff010203)

	if c.Value() != 0x00010203 {
		t.Errorf("Value = %#x, want 0x00010203", c.Value())
	}
}

func TestCountNextCarriesOverflow(t *testing.T) {
	// Sequence number wraps 0xff -> 0x00 and carries into the overflow counter.
	c := MakeCount(4, 0xff).Next()

	if c.Overflow() != 5 || c.SQN() != 0 {
		t.Fatalf("Next after (4,0xff) = (%d,%#x), want (5,0x00)", c.Overflow(), c.SQN())
	}
}

func TestCountNextWrapsAt24Bits(t *testing.T) {
	// The whole 24-bit count wraps back to zero (TS 24.301 §4.4.3.5 requires a
	// re-key before this, but the type must not leak into the padding bits).
	if got := MakeCount(0xffff, 0xff).Next(); got != 0 {
		t.Fatalf("Next at max 24-bit count = %#x, want 0", uint32(got))
	}
}

func TestReconcileUplinkNoWrap(t *testing.T) {
	// A sequence number above the expected one keeps the overflow counter.
	got := MakeCount(7, 10).reconcileUplink(11)

	if got != MakeCount(7, 11) {
		t.Fatalf("reconcileUplink(11) from (7,10) = (%d,%d), want (7,11)", got.Overflow(), got.SQN())
	}
}

func TestReconcileUplinkWrap(t *testing.T) {
	// A sequence number below the expected one places the message after a wrap.
	got := MakeCount(7, 250).reconcileUplink(2)

	if got != MakeCount(8, 2) {
		t.Fatalf("reconcileUplink(2) from (7,250) = (%d,%d), want (8,2)", got.Overflow(), got.SQN())
	}
}

func TestReconcileUplinkSameSQN(t *testing.T) {
	// The expected sequence number is the expected count.
	got := MakeCount(7, 10).reconcileUplink(10)

	if got != MakeCount(7, 10) {
		t.Fatalf("reconcileUplink(10) from (7,10) = (%d,%d), want (7,10)", got.Overflow(), got.SQN())
	}
}
