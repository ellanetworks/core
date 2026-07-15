// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

import (
	"bytes"
	"testing"
)

func TestConstrainedIntKnownVectors(t *testing.T) {
	cases := []struct {
		name      string
		v, lb, ub int64
		want      []byte
	}{
		// range 1: no bits, so the buffer stays empty.
		{"single value", 7, 7, 7, nil},
		// range 2: a single bit-field bit.
		{"one bit set", 1, 0, 1, []byte{0x80}},
		{"one bit clear", 0, 0, 1, []byte{0x00}},
		// range 256: one aligned octet.
		{"one octet", 5, 0, 255, []byte{0x05}},
		// range 257..65536: two aligned octets.
		{"two octets", 0x0102, 0, 65535, []byte{0x01, 0x02}},
		// range > 64K: octet-count + aligned value. MME-UE-S1AP-ID is
		// INTEGER(0..4294967295); see TS 36.413 / X.691.
		{"mme-ue-id 1", 1, 0, 4294967295, []byte{0x00, 0x01}},
		{"mme-ue-id 256", 256, 0, 4294967295, []byte{0x40, 0x01, 0x00}},
		// ENB-UE-S1AP-ID is INTEGER(0..16777215): range 2^24, byteLen 3.
		{"enb-ue-id 1", 1, 0, 16777215, []byte{0x00, 0x01}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var w Writer
			if err := w.WriteConstrainedInt(c.v, c.lb, c.ub); err != nil {
				t.Fatalf("encode: %v", err)
			}

			if !bytes.Equal(w.Bytes(), c.want) {
				t.Fatalf("bytes = % x, want % x", w.Bytes(), c.want)
			}

			got, err := NewReader(w.Bytes()).ReadConstrainedInt(c.lb, c.ub)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			if got != c.v {
				t.Fatalf("decoded %d, want %d", got, c.v)
			}
		})
	}
}

func TestConstrainedIntRoundTrip(t *testing.T) {
	ranges := []struct{ lb, ub int64 }{
		{0, 1},
		{0, 7},
		{0, 254},
		{0, 255},
		{0, 256},
		{3, 20},
		{0, 65535},
		{0, 65536},
		{0, 16777215},
		{0, 4294967295},
		{-128, 127},
		{100, 1 << 33},
	}
	for _, rg := range ranges {
		for _, v := range []int64{rg.lb, rg.lb + 1, (rg.lb + rg.ub) / 2, rg.ub} {
			// Prefix with a stray bit so the value does not start octet-aligned;
			// this exercises the alignment paths.
			var w Writer
			w.WriteBit(1)

			if err := w.WriteConstrainedInt(v, rg.lb, rg.ub); err != nil {
				t.Fatalf("encode v=%d [%d,%d]: %v", v, rg.lb, rg.ub, err)
			}

			r := NewReader(w.Bytes())
			if _, err := r.ReadBit(); err != nil {
				t.Fatal(err)
			}

			got, err := r.ReadConstrainedInt(rg.lb, rg.ub)
			if err != nil {
				t.Fatalf("decode v=%d [%d,%d]: %v", v, rg.lb, rg.ub, err)
			}

			if got != v {
				t.Fatalf("v=%d [%d,%d]: decoded %d", v, rg.lb, rg.ub, got)
			}
		}
	}
}

func TestConstrainedIntOutOfRange(t *testing.T) {
	var w Writer
	if err := w.WriteConstrainedInt(10, 0, 5); err == nil {
		t.Fatal("expected out-of-range error")
	}
}

func TestConstrainedIntDecodeRejectsOutOfRange(t *testing.T) {
	// range 3 (lb=0, ub=2) uses a 2-bit field; the pattern 0b11 = 3 has no
	// assigned value and must be rejected, not returned as lb+3.
	if _, err := NewReader([]byte{0xc0}).ReadConstrainedInt(0, 2); err == nil {
		t.Fatal("expected out-of-range decode error")
	}
}

func TestSemiConstrainedRoundTrip(t *testing.T) {
	for _, lb := range []int64{0, 1, -5, 1000} {
		for _, d := range []int64{0, 1, 255, 256, 70000, 1 << 40} {
			v := lb + d

			var w Writer
			if err := w.WriteSemiConstrainedInt(v, lb); err != nil {
				t.Fatalf("encode v=%d lb=%d: %v", v, lb, err)
			}

			got, err := NewReader(w.Bytes()).ReadSemiConstrainedInt(lb)
			if err != nil {
				t.Fatalf("decode v=%d lb=%d: %v", v, lb, err)
			}

			if got != v {
				t.Fatalf("v=%d lb=%d: decoded %d", v, lb, got)
			}
		}
	}
}

func TestUnconstrainedRoundTrip(t *testing.T) {
	values := []int64{0, 1, -1, 127, 128, -128, -129, 255, 256, 32767, -32768, 1 << 40, -(1 << 40), 1 << 62, -(1 << 62)}
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

func TestUnconstrainedKnownVector(t *testing.T) {
	var w Writer
	if err := w.WriteUnconstrainedInt(-1); err != nil {
		t.Fatal(err)
	}

	if want := []byte{0x01, 0xff}; !bytes.Equal(w.Bytes(), want) {
		t.Fatalf("bytes = % x, want % x", w.Bytes(), want)
	}
}

func TestNormallySmallRoundTrip(t *testing.T) {
	for _, v := range []uint64{0, 1, 63, 64, 65, 1000, 1 << 20} {
		var w Writer
		if err := w.WriteNormallySmall(v); err != nil {
			t.Fatalf("encode %d: %v", v, err)
		}

		got, err := NewReader(w.Bytes()).ReadNormallySmall()
		if err != nil {
			t.Fatalf("decode %d: %v", v, err)
		}

		if got != v {
			t.Fatalf("decoded %d, want %d", got, v)
		}
	}
}

func TestNormallySmallKnownVector(t *testing.T) {
	var w Writer
	if err := w.WriteNormallySmall(63); err != nil {
		t.Fatal(err)
	}
	// bit '0' + 6-bit value 0b111111 => 0b0111111_0 = 0x7e.
	if want := []byte{0x7e}; !bytes.Equal(w.Bytes(), want) {
		t.Fatalf("bytes = % x, want % x", w.Bytes(), want)
	}
}
