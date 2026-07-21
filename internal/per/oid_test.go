// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import (
	"bytes"
	"testing"
)

func TestOIDContentBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		arcs []uint64
		want []byte
	}{
		{[]uint64{1, 3, 6, 1}, []byte{0x2B, 0x06, 0x01}},                          // 1.3.6.1
		{[]uint64{1, 2, 840, 113549}, []byte{0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D}}, // RSA oid 1.2.840.113549
		{[]uint64{2, 5, 4, 3}, []byte{0x55, 0x04, 0x03}},                          // 2.5.4.3
		{[]uint64{0, 0}, []byte{0x00}},
		{[]uint64{2, 999}, []byte{0x88, 0x37}}, // 2.999 -> 80+919? 40*2+... wait
	}
	for _, c := range cases {
		got := oidContentBytes(c.arcs)
		if !bytes.Equal(got, c.want) {
			t.Errorf("arcs %v: got %x, want %x", c.arcs, got, c.want)
		}
	}
}

func TestOIDRoundtrip(t *testing.T) {
	t.Parallel()

	cases := [][]uint64{
		{1, 3, 6, 1},
		{1, 2, 840, 113549},
		{2, 5, 4, 3},
		{0, 0},
		{1, 3, 6, 1, 4, 1, 311, 21, 8},
	}
	for _, enc := range []Encoding{Aligned, Unaligned} {
		for _, arcs := range cases {
			w := NewWriter()
			if err := EncodeOID(w, enc, arcs); err != nil {
				t.Fatalf("enc=%v arcs=%v: %v", enc, arcs, err)
			}

			buf := w.Bytes()
			r := NewReader(buf)

			got, err := DecodeOID(r, enc)
			if err != nil {
				t.Fatalf("enc=%v arcs=%v decode: %v", enc, arcs, err)
			}

			if len(got) != len(arcs) {
				t.Fatalf("enc=%v arcs=%v: got %v", enc, arcs, got)
			}

			for i := range arcs {
				if got[i] != arcs[i] {
					t.Fatalf("enc=%v arcs=%v: got %v", enc, arcs, got)
				}
			}
		}
	}
}

func TestOIDEncodingExact(t *testing.T) {
	t.Parallel()
	// 1.3.6.1 -> content [0x2B,0x06,0x01], length 3 (semi, form a 0x03), octet-aligned content
	w := NewWriter()
	if err := EncodeOID(w, Aligned, []uint64{1, 3, 6, 1}); err != nil {
		t.Fatal(err)
	}

	got := w.Bytes()

	want := []byte{0x03, 0x2B, 0x06, 0x01}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}
