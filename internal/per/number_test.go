// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import (
	"bytes"
	"testing"
)

func TestBitsNeeded(t *testing.T) {
	t.Parallel()

	cases := []struct {
		rng  int64
		bits int
	}{
		{1, 0},
		{2, 1},
		{3, 2},
		{4, 2},
		{5, 3},
		{8, 3},
		{9, 4},
		{16, 4},
		{17, 5},
		{256, 8},
		{257, 9},
		{65536, 16},
		{65537, 17},
	}
	for _, c := range cases {
		if got := bitsNeeded(c.rng); got != c.bits {
			t.Errorf("bitsNeeded(%d) = %d, want %d", c.rng, got, c.bits)
		}
	}
}

func TestMinOctetsNonNeg(t *testing.T) {
	t.Parallel()

	cases := []struct {
		v      uint64
		octets int
	}{
		{0, 1}, {1, 1}, {255, 1}, {256, 2}, {65535, 2}, {65536, 3},
	}
	for _, c := range cases {
		if got := minOctetsNonNeg(c.v); got != c.octets {
			t.Errorf("minOctetsNonNeg(%d) = %d, want %d", c.v, got, c.octets)
		}
	}
}

func TestEncodeConstrainedWholeNumberBitField(t *testing.T) {
	t.Parallel()
	// range 10 (4 bits), value 5 -> 0101
	w := NewWriter()
	if err := EncodeConstrainedWholeNumber(w, Aligned, 0, 9, 5); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	got := w.Bytes()

	want := []byte{0b0101_0000}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %08b, want %08b", got, want)
	}
}

func TestEncodeConstrainedWholeNumberRangeOne(t *testing.T) {
	t.Parallel()

	w := NewWriter()
	if err := EncodeConstrainedWholeNumber(w, Aligned, 7, 7, 7); err != nil {
		t.Fatal(err)
	}

	if w.Bits() != 0 {
		t.Fatalf("expected empty field, got %d bits", w.Bits())
	}
}

func TestEncodeConstrainedWholeNumberOneOctet(t *testing.T) {
	t.Parallel()
	// range 256, value 200 -> octet-aligned single octet 0xC8
	w := NewWriter()
	w.WriteBit(true) // start unaligned

	if err := EncodeConstrainedWholeNumber(w, Aligned, 0, 255, 200); err != nil {
		t.Fatal(err)
	}

	got := w.Bytes()
	// 1 pad bit + align to byte + 0xC8
	want := []byte{0b1000_0000, 0xC8}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

func TestEncodeConstrainedWholeNumberTwoOctet(t *testing.T) {
	t.Parallel()
	// range 65536, value 300 -> octet-aligned two octets 0x012C
	w := NewWriter()
	if err := EncodeConstrainedWholeNumber(w, Aligned, 0, 65535, 300); err != nil {
		t.Fatal(err)
	}

	got := w.Bytes()

	want := []byte{0x01, 0x2C}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

func TestEncodeConstrainedWholeNumberUnaligned(t *testing.T) {
	t.Parallel()
	// unaligned range 257 -> 9 bits, value 5 -> 000000101
	w := NewWriter()
	if err := EncodeConstrainedWholeNumber(w, Unaligned, 0, 256, 5); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	got := w.Bytes()

	want := []byte{0b0000_0010, 0b1000_0000}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %08b, want %08b", got, want)
	}
}

func TestConstrainedWholeNumberRoundtrip(t *testing.T) {
	t.Parallel()

	for _, enc := range []Encoding{Aligned, Unaligned} {
		for _, rng := range []int64{2, 10, 255, 256, 257, 65536} {
			for _, n := range []int64{0, rng / 2, rng - 1} {
				w := NewWriter()
				if err := EncodeConstrainedWholeNumber(w, enc, 0, rng-1, n); err != nil {
					t.Errorf("enc=%v rng=%d n=%d: %v", enc, rng, n, err)
					continue
				}

				w.AlignToByte()
				r := NewReader(w.Bytes())

				got, err := DecodeConstrainedWholeNumber(r, enc, 0, rng-1)
				if err != nil || got != n {
					t.Errorf("enc=%v rng=%d n=%d: got %d err %v", enc, rng, n, got, err)
				}
			}
		}
	}
}

func TestEncodeConstrainedWholeNumberOverflow(t *testing.T) {
	t.Parallel()

	w := NewWriter()
	if err := EncodeConstrainedWholeNumber(w, Aligned, 0, 9, 10); err != ErrOverflow {
		t.Fatalf("err = %v, want ErrOverflow", err)
	}
}

func TestNormallySmallSmall(t *testing.T) {
	t.Parallel()
	// n=5 -> 0 + 000101
	w := NewWriter()
	if err := EncodeNormallySmall(w, Aligned, 5); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	got := w.Bytes()

	want := []byte{0b0_000101_0}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %08b, want %08b", got, want)
	}

	r := NewReader(got)

	n, err := DecodeNormallySmall(r, Aligned)
	if err != nil || n != 5 {
		t.Fatalf("decode: n=%d err=%v", n, err)
	}
}

func TestNormallySmallLarge(t *testing.T) {
	t.Parallel()
	// n=100 -> 1 + semi-constrained (lb=0): length 1 (0x01) + 0x64
	w := NewWriter()
	if err := EncodeNormallySmall(w, Aligned, 100); err != nil {
		t.Fatal(err)
	}

	got := w.Bytes()

	want := []byte{0b1_0000000, 0x01, 0x64}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}

	r := NewReader(got)

	n, err := DecodeNormallySmall(r, Aligned)
	if err != nil || n != 100 {
		t.Fatalf("decode: n=%d err=%v", n, err)
	}
}
