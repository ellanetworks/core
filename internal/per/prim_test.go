package per

import (
	"bytes"
	"testing"
)

func TestBooleanRoundtrip(t *testing.T) {
	t.Parallel()

	for _, v := range []bool{true, false} {
		w := NewWriter()
		EncodeBoolean(w, Aligned, v)
		w.AlignToByte()
		got := w.Bytes()

		var want byte
		if v {
			want = 0x80
		}

		if len(got) != 1 || got[0] != want {
			t.Fatalf("got %x, want %x", got, want)
		}

		r := NewReader(got)

		b, err := DecodeBoolean(r, Aligned)
		if err != nil || b != v {
			t.Fatalf("decode %v: b=%v err=%v", v, b, err)
		}
	}
}

func TestNullNoBits(t *testing.T) {
	t.Parallel()

	w := NewWriter()
	if err := EncodeNull(w, Aligned); err != nil {
		t.Fatal(err)
	}

	if w.Bits() != 0 {
		t.Fatalf("null wrote %d bits", w.Bits())
	}

	r := NewReader(nil)
	if err := DecodeNull(r, Aligned); err != nil {
		t.Fatal(err)
	}
}

func TestIntegerConstrainedBitField(t *testing.T) {
	t.Parallel()
	// INTEGER (0..9) = 5 -> 4-bit bit-field 0101
	w := NewWriter()
	if err := EncodeInteger(w, Aligned, Bounds{LB: 0, UB: 9, HasLB: true, HasUB: true}, 5); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	got := w.Bytes()

	want := []byte{0b0101_0000}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

func TestIntegerConstrainedOneOctet(t *testing.T) {
	t.Parallel()
	// INTEGER (0..255) = 200 -> octet-aligned 0xC8
	w := NewWriter()
	if err := EncodeInteger(w, Aligned, Bounds{LB: 0, UB: 255, HasLB: true, HasUB: true}, 200); err != nil {
		t.Fatal(err)
	}

	got := w.Bytes()

	want := []byte{0xC8}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

func TestIntegerSpecExample(t *testing.T) {
	t.Parallel()
	// §13.2.6 NOTE: foo INTEGER (256..1234567) ::= 256, aligned -> 0x00 0x00
	w := NewWriter()
	if err := EncodeInteger(w, Aligned, Bounds{LB: 256, UB: 1234567, HasLB: true, HasUB: true}, 256); err != nil {
		t.Fatal(err)
	}

	got := w.Bytes()

	want := []byte{0x00, 0x00}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}

	r := NewReader(got)

	n, err := DecodeInteger(r, Aligned, Bounds{LB: 256, UB: 1234567, HasLB: true, HasUB: true})
	if err != nil || n != 256 {
		t.Fatalf("decode: n=%d err=%v", n, err)
	}
}

func TestIntegerUnconstrained(t *testing.T) {
	t.Parallel()
	// unconstrained -1 -> length 1 (0x01) + 0xFF (2's complement min octets)
	w := NewWriter()
	if err := EncodeInteger(w, Aligned, Bounds{}, -1); err != nil {
		t.Fatal(err)
	}

	got := w.Bytes()

	want := []byte{0x01, 0xFF}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}

	r := NewReader(got)

	n, err := DecodeInteger(r, Aligned, Bounds{})
	if err != nil || n != -1 {
		t.Fatalf("decode: n=%d err=%v", n, err)
	}
}

func TestIntegerSemiConstrained(t *testing.T) {
	t.Parallel()
	// INTEGER (0..MAX) lb=0, value 300 -> length 2 (0x02) + 0x012C
	w := NewWriter()
	if err := EncodeInteger(w, Aligned, Bounds{LB: 0, HasLB: true}, 300); err != nil {
		t.Fatal(err)
	}

	got := w.Bytes()

	want := []byte{0x02, 0x01, 0x2C}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}

	r := NewReader(got)

	n, err := DecodeInteger(r, Aligned, Bounds{LB: 0, HasLB: true})
	if err != nil || n != 300 {
		t.Fatalf("decode: n=%d err=%v", n, err)
	}
}

func TestIntegerExtensibleInRoot(t *testing.T) {
	t.Parallel()
	// INTEGER (0..9, ...) = 5 -> ext bit 0 + 4-bit 0101
	w := NewWriter()

	b := Bounds{LB: 0, UB: 9, HasLB: true, HasUB: true, Extensible: true}
	if err := EncodeInteger(w, Aligned, b, 5); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	got := w.Bytes()

	want := []byte{0b0_0101_000}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %08b, want %08b", got, want)
	}

	r := NewReader(got)

	n, err := DecodeInteger(r, Aligned, b)
	if err != nil || n != 5 {
		t.Fatalf("decode: n=%d err=%v", n, err)
	}
}

func TestIntegerExtensibleOutOfRoot(t *testing.T) {
	t.Parallel()
	// INTEGER (0..9, ...) = 100 -> ext bit 1 + unconstrained: length 1 + 0x64
	w := NewWriter()

	b := Bounds{LB: 0, UB: 9, HasLB: true, HasUB: true, Extensible: true}
	if err := EncodeInteger(w, Aligned, b, 100); err != nil {
		t.Fatal(err)
	}

	got := w.Bytes()
	// 1 + align to byte (7 pad) = 0x80, then length 0x01, then 0x64
	want := []byte{0x80, 0x01, 0x64}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}

	r := NewReader(got)

	n, err := DecodeInteger(r, Aligned, b)
	if err != nil || n != 100 {
		t.Fatalf("decode: n=%d err=%v", n, err)
	}
}

func TestIntegerRoundtripMany(t *testing.T) {
	t.Parallel()

	cases := []struct {
		b Bounds
		n int64
	}{
		{Bounds{LB: 0, UB: 9, HasLB: true, HasUB: true}, 0},
		{Bounds{LB: 0, UB: 9, HasLB: true, HasUB: true}, 9},
		{Bounds{LB: 0, UB: 255, HasLB: true, HasUB: true}, 200},
		{Bounds{LB: 0, UB: 65535, HasLB: true, HasUB: true}, 300},
		{Bounds{LB: 256, UB: 1234567, HasLB: true, HasUB: true}, 256},
		{Bounds{LB: 256, UB: 1234567, HasLB: true, HasUB: true}, 1234567},
		{Bounds{LB: 0, HasLB: true}, 300},
		{Bounds{}, -1},
		{Bounds{}, 1000000},
		{Bounds{LB: 0, UB: 9, HasLB: true, HasUB: true, Extensible: true}, 5},
		{Bounds{LB: 0, UB: 9, HasLB: true, HasUB: true, Extensible: true}, 100},
	}
	for _, enc := range []Encoding{Aligned, Unaligned} {
		for _, c := range cases {
			w := NewWriter()
			if err := EncodeInteger(w, enc, c.b, c.n); err != nil {
				t.Errorf("enc=%v b=%+v n=%d: encode %v", enc, c.b, c.n, err)
				continue
			}

			w.AlignToByte()
			r := NewReader(w.Bytes())

			got, err := DecodeInteger(r, enc, c.b)
			if err != nil || got != c.n {
				t.Errorf("enc=%v b=%+v n=%d: got %d err %v", enc, c.b, c.n, got, err)
			}
		}
	}
}
