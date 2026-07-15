// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func TestWriteBool(t *testing.T) {
	for _, b := range []bool{true, false} {
		var w Writer

		w.WriteBool(b)

		got, err := NewReader(w.Bytes()).ReadBool()
		if err != nil {
			t.Fatal(err)
		}

		if got != b {
			t.Fatalf("decoded %v, want %v", got, b)
		}
	}

	var wt Writer

	wt.WriteBool(true)

	if wt.Bytes()[0] != 0x80 {
		t.Fatalf("true => % x, want 80", wt.Bytes())
	}
}

func TestDecodeErrorFormat(t *testing.T) {
	e := &DecodeError{Offset: 13, Msg: "boom"}

	s := e.Error()
	if !strings.Contains(s, "boom") || !strings.Contains(s, "13") {
		t.Fatalf("Error() = %q, want it to mention the message and offset", s)
	}
}

func TestUnconstrainedEdgeOctets(t *testing.T) {
	values := []int64{
		math.MaxInt64, math.MinInt64,
		1 << 55, -(1 << 55), (1 << 55) - 1,
		1 << 56, -(1 << 56),
	}
	for _, v := range values {
		var w Writer

		if err := w.WriteUnconstrainedInt(v); err != nil {
			t.Fatalf("encode %d: %v", v, err)
		}

		got, err := NewReader(w.Bytes()).ReadUnconstrainedInt()
		if err != nil {
			t.Fatalf("decode %d: %v", v, err)
		}

		if got != v {
			t.Fatalf("decoded %d, want %d", got, v)
		}
	}
}

func TestConstrainedIntDecodeRejectsTwoOctetOutOfRange(t *testing.T) {
	// Range [0,999] uses the two-octet case; the raw value 1000 has no
	// assigned value and must be rejected.
	if _, err := NewReader([]byte{0x03, 0xe8}).ReadConstrainedInt(0, 999); err == nil {
		t.Fatal("expected out-of-range decode error")
	}
}

func TestIntegerLengthRejected(t *testing.T) {
	// A length determinant of 9 octets exceeds the 8-octet integer limit.
	if _, err := NewReader([]byte{0x09}).ReadSemiConstrainedInt(0); err == nil {
		t.Fatal("semi-constrained: expected length error")
	}

	if _, err := NewReader([]byte{0x09}).ReadUnconstrainedInt(); err == nil {
		t.Fatal("unconstrained: expected length error")
	}

	// A zero-octet integer is malformed.
	if _, err := NewReader([]byte{0x00}).ReadSemiConstrainedInt(0); err == nil {
		t.Fatal("semi-constrained: expected zero-length error")
	}
}

func TestOctetStringSizeOutOfRange(t *testing.T) {
	var w Writer

	if err := w.WriteOctetString([]byte{1, 2, 3, 4, 5}, 1, 4, false); err == nil {
		t.Fatal("expected size-out-of-range error for non-extensible string")
	}
}

func TestOctetStringExtensible(t *testing.T) {
	// In-root and out-of-root extensible values both round-trip.
	for _, payload := range [][]byte{{0xaa}, {0xa, 0xb, 0xc, 0xd, 0xe}} {
		var w Writer

		if err := w.WriteOctetString(payload, 1, 4, true); err != nil {
			t.Fatalf("len %d: %v", len(payload), err)
		}

		got, err := NewReader(w.Bytes()).ReadOctetString(1, 4, true)
		if err != nil {
			t.Fatalf("len %d: %v", len(payload), err)
		}

		if !bytes.Equal(got, payload) {
			t.Fatalf("len %d: decoded % x", len(payload), got)
		}
	}
}

func TestWriteBitStringShortBuffer(t *testing.T) {
	var w Writer

	// 20 bits need 3 octets; a 1-octet buffer is rejected.
	if err := w.WriteBitString([]byte{0x00}, 20, 20, 20, false); err == nil {
		t.Fatal("expected short-buffer error")
	}
}

func TestWriteLengthNegative(t *testing.T) {
	var w Writer

	if err := w.WriteLength(-1); err == nil {
		t.Fatal("expected negative-length error")
	}
}

func TestTruncatedInputsError(t *testing.T) {
	// Every decoder on empty input must return an error, never panic.
	if _, err := NewReader(nil).ReadConstrainedInt(0, 4294967295); err == nil {
		t.Fatal("constrained int")
	}

	if _, err := NewReader(nil).ReadLength(); err == nil {
		t.Fatal("length")
	}

	if _, err := NewReader(nil).ReadOpenType(); err == nil {
		t.Fatal("open type")
	}

	if _, _, err := NewReader(nil).ReadBitString(1, 160, true); err == nil {
		t.Fatal("bit string")
	}

	if _, err := NewReader(nil).ReadOctetString(0, Unbounded, false); err == nil {
		t.Fatal("octet string")
	}
}
