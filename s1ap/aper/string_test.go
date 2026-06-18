// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

import (
	"bytes"
	"testing"
)

func TestOctetStringKnownVectors(t *testing.T) {
	cases := []struct {
		name   string
		b      []byte
		lb, ub int
		ext    bool
		want   []byte
	}{
		// PLMNidentity: OCTET STRING(SIZE(3)) -> aligned, no length.
		{"plmn fixed 3", []byte{0x12, 0xf3, 0x45}, 3, 3, false, []byte{0x12, 0xf3, 0x45}},
		// Fixed <=2 octets: bit-field, no alignment (starts aligned here so
		// the bytes are identical, but no length prefix is added).
		{"fixed 2", []byte{0x01, 0x02}, 2, 2, false, []byte{0x01, 0x02}},
		// Variable SIZE(1..4): constrained length (2 bits) + align + content.
		{"var 1..4 len2", []byte{0xaa, 0xbb}, 1, 4, false, []byte{0x40, 0xaa, 0xbb}},
		// Unbounded (NAS-PDU): unconstrained length determinant + content.
		{"unbounded len2", []byte{0xde, 0xad}, 0, Unbounded, false, []byte{0x02, 0xde, 0xad}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var w Writer
			if err := w.WriteOctetString(c.b, c.lb, c.ub, c.ext); err != nil {
				t.Fatalf("encode: %v", err)
			}

			if !bytes.Equal(w.Bytes(), c.want) {
				t.Fatalf("bytes = % x, want % x", w.Bytes(), c.want)
			}

			got, err := NewReader(w.Bytes()).ReadOctetString(c.lb, c.ub, c.ext)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			if !bytes.Equal(got, c.b) {
				t.Fatalf("decoded % x, want % x", got, c.b)
			}
		})
	}
}

func TestOctetStringRoundTrip(t *testing.T) {
	payloads := [][]byte{{}, {0x01}, {0x01, 0x02}, {0x01, 0x02, 0x03}, bytes.Repeat([]byte{0x5a}, 200)}

	constraints := []struct{ lb, ub int }{{0, Unbounded}, {0, 255}, {1, 4}}
	for _, p := range payloads {
		for _, c := range constraints {
			if c.ub >= 0 && (len(p) < c.lb || len(p) > c.ub) {
				continue
			}
			// Stray leading bit exercises the unaligned/align paths.
			var w Writer
			w.WriteBit(1)

			if err := w.WriteOctetString(p, c.lb, c.ub, false); err != nil {
				t.Fatalf("len %d [%d,%d]: %v", len(p), c.lb, c.ub, err)
			}

			r := NewReader(w.Bytes())
			if _, err := r.ReadBit(); err != nil {
				t.Fatal(err)
			}

			got, err := r.ReadOctetString(c.lb, c.ub, false)
			if err != nil {
				t.Fatalf("len %d [%d,%d]: %v", len(p), c.lb, c.ub, err)
			}

			if !bytes.Equal(got, p) {
				t.Fatalf("len %d [%d,%d]: decoded % x", len(p), c.lb, c.ub, got)
			}
		}
	}
}

func TestBitStringKnownVectors(t *testing.T) {
	// ENB-ID macroENB-ID: BIT STRING(SIZE(20)) -> aligned, 20 bits.
	var w Writer
	if err := w.WriteBitString([]byte{0x12, 0x34, 0x50}, 20, 20, 20, false); err != nil {
		t.Fatal(err)
	}

	if want := []byte{0x12, 0x34, 0x50}; !bytes.Equal(w.Bytes(), want) {
		t.Fatalf("bytes = % x, want % x", w.Bytes(), want)
	}

	got, n, err := NewReader(w.Bytes()).ReadBitString(20, 20, false)
	if err != nil {
		t.Fatal(err)
	}

	if n != 20 || !bytes.Equal(got, []byte{0x12, 0x34, 0x50}) {
		t.Fatalf("decoded % x (%d bits)", got, n)
	}
}

func TestBitStringRoundTrip(t *testing.T) {
	// (octets, nbits) with unused low bits of the last octet cleared.
	cases := []struct {
		b      []byte
		nbits  int
		lb, ub int
	}{
		{[]byte{0xa0}, 4, 4, 4},                // fixed, bit-field (<=16)
		{[]byte{0xab, 0xcd}, 16, 16, 16},       // fixed, bit-field boundary
		{[]byte{0x12, 0x34, 0x50}, 20, 20, 20}, // fixed, aligned (>16)
		{[]byte{0xff, 0x80}, 9, 1, 160},        // variable, aligned
		{[]byte{0xde, 0xad, 0xbe}, 24, 0, Unbounded},
	}
	for _, c := range cases {
		var w Writer
		w.WriteBit(1) // stray bit to force unaligned start

		if err := w.WriteBitString(c.b, c.nbits, c.lb, c.ub, false); err != nil {
			t.Fatalf("%d bits [%d,%d]: %v", c.nbits, c.lb, c.ub, err)
		}

		r := NewReader(w.Bytes())
		if _, err := r.ReadBit(); err != nil {
			t.Fatal(err)
		}

		got, n, err := r.ReadBitString(c.lb, c.ub, false)
		if err != nil {
			t.Fatalf("%d bits [%d,%d]: %v", c.nbits, c.lb, c.ub, err)
		}

		if n != c.nbits || !bytes.Equal(got, c.b) {
			t.Fatalf("%d bits [%d,%d]: decoded % x (%d bits), want % x", c.nbits, c.lb, c.ub, got, n, c.b)
		}
	}
}
