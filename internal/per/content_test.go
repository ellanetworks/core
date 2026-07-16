package per

import (
	"bytes"
	"testing"
)

func TestBitStringFixedSmall(t *testing.T) {
	t.Parallel()
	// BIT STRING (SIZE (8)) = 0xB6 -> 8-bit bit-field, no length, no align
	w := NewWriter()

	data := []byte{0xB6}
	if err := EncodeBitString(w, Aligned, 8, 8, true, true, false, data, 8); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	got := w.Bytes()

	want := []byte{0xB6}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

func TestBitStringFixedLarge(t *testing.T) {
	t.Parallel()
	// BIT STRING (SIZE (24)) -> octet-aligned, no length
	w := NewWriter()
	w.WriteBit(true) // unaligned start

	data := []byte{0xAB, 0xCD, 0xEF}
	if err := EncodeBitString(w, Aligned, 24, 24, true, true, false, data, 24); err != nil {
		t.Fatal(err)
	}

	got := w.Bytes()
	// 1 pad bit + align (7 zeros) -> 0x80, then 0xAB 0xCD 0xEF
	want := []byte{0x80, 0xAB, 0xCD, 0xEF}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

func TestBitStringUnconstrained(t *testing.T) {
	t.Parallel()
	// BIT STRING (no constraint) = 12 bits 0xABC -> length 12 (0x0C) + octet-aligned 12 bits
	w := NewWriter()

	data := []byte{0xAB, 0xC0}
	if err := EncodeBitString(w, Aligned, 0, 0, false, false, false, data, 12); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	got := w.Bytes()
	// length 0x0C (1 octet), then 12 bits 0xABC octet-aligned, then 4 pad bits -> 3 octets
	want := []byte{0x0C, 0xAB, 0xC0}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}

	r := NewReader(got)

	d, nbits, err := DecodeBitString(r, Aligned, 0, 0, false, false, false)
	if err != nil {
		t.Fatal(err)
	}

	if nbits != 12 || !bytes.Equal(d, []byte{0xAB, 0xC0}) {
		t.Fatalf("decode: nbits=%d data=%x", nbits, d)
	}
}

func TestOctetStringFixedSmall(t *testing.T) {
	t.Parallel()
	// OCTET STRING (SIZE (2)) = 0xAB 0xCD -> 16-bit bit-field, no align
	w := NewWriter()
	w.WriteBit(true) // unaligned start

	data := []byte{0xAB, 0xCD}
	if err := EncodeOctetString(w, Aligned, 2, 2, true, true, false, data); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	got := w.Bytes()
	// 1 + 16 bits = 17 bits: 11010101 11100110 10000000
	want := []byte{0xD5, 0xE6, 0x80}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}

	r := NewReader(got)
	if _, err := r.ReadBit(); err != nil {
		t.Fatal(err)
	}

	d, err := DecodeOctetString(r, Aligned, 2, 2, true, true, false)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(d, data) {
		t.Fatalf("decode: %x", d)
	}
}

func TestOctetStringFixedLarge(t *testing.T) {
	t.Parallel()
	// OCTET STRING (SIZE (4)) -> octet-aligned, no length
	w := NewWriter()
	w.WriteBit(true)

	data := []byte{0x01, 0x02, 0x03, 0x04}
	if err := EncodeOctetString(w, Aligned, 4, 4, true, true, false, data); err != nil {
		t.Fatal(err)
	}

	got := w.Bytes()

	want := []byte{0x80, 0x01, 0x02, 0x03, 0x04}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

func TestOctetStringUnconstrained(t *testing.T) {
	t.Parallel()
	// OCTET STRING (no constraint) = 5 bytes -> length 0x05 + 5 bytes
	w := NewWriter()

	data := []byte{1, 2, 3, 4, 5}
	if err := EncodeOctetString(w, Aligned, 0, 0, false, false, false, data); err != nil {
		t.Fatal(err)
	}

	got := w.Bytes()

	want := []byte{0x05, 1, 2, 3, 4, 5}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}

	r := NewReader(got)

	d, err := DecodeOctetString(r, Aligned, 0, 0, false, false, false)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(d, data) {
		t.Fatalf("decode: %x", d)
	}
}

func TestOctetStringConstrainedLength(t *testing.T) {
	t.Parallel()
	// OCTET STRING (SIZE (0..10)) = 3 bytes -> constrained length range 11 -> 4 bits, value 3 -> 0011
	w := NewWriter()

	data := []byte{0xAA, 0xBB, 0xCC}
	if err := EncodeOctetString(w, Aligned, 0, 10, true, true, false, data); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	got := w.Bytes()
	// 4 bits 0011 + align (4 zeros) -> 0x30, then octet-aligned 3 bytes
	want := []byte{0x30, 0xAA, 0xBB, 0xCC}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}

	r := NewReader(got)

	d, err := DecodeOctetString(r, Aligned, 0, 10, true, true, false)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(d, data) {
		t.Fatalf("decode: %x", d)
	}
}

func TestBitStringOctetStringRoundtrip(t *testing.T) {
	t.Parallel()

	for _, enc := range []Encoding{Aligned, Unaligned} {
		// unconstrained octet string of varying sizes incl. fragmentation boundary
		for _, sz := range []int{0, 1, 2, 3, 100, 127, 128, 16383, 16384, 16385, 40000} {
			data := make([]byte, sz)
			for i := range data {
				data[i] = byte(i)
			}

			w := NewWriter()
			if err := EncodeOctetString(w, enc, 0, 0, false, false, false, data); err != nil {
				t.Errorf("enc=%v sz=%d enc: %v", enc, sz, err)
				continue
			}

			buf := w.Bytes()
			r := NewReader(buf)

			got, err := DecodeOctetString(r, enc, 0, 0, false, false, false)
			if err != nil {
				t.Errorf("enc=%v sz=%d dec: %v", enc, sz, err)
				continue
			}

			if !bytes.Equal(got, data) {
				t.Errorf("enc=%v sz=%d mismatch", enc, sz)
			}
		}
	}
}
