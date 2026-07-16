package per

import (
	"bytes"
	"testing"
)

func TestBitStringDecodeFixedLarge(t *testing.T) {
	t.Parallel()

	data := []byte{0xAB, 0xCD, 0xEF}

	w := NewWriter()
	if err := EncodeBitString(w, Aligned, 24, 24, true, true, false, data, 24); err != nil {
		t.Fatal(err)
	}

	r := NewReader(w.Bytes())

	got, nbits, err := DecodeBitString(r, Aligned, 24, 24, true, true, false)
	if err != nil {
		t.Fatal(err)
	}

	if nbits != 24 || !bytes.Equal(got, data) {
		t.Fatalf("got nbits=%d data=%x", nbits, got)
	}
}

func TestBitStringExtensible(t *testing.T) {
	t.Parallel()
	// BIT STRING (SIZE (0..8), ...) with 8 bits -> in root, ext bit 0
	data := []byte{0xB6}

	w := NewWriter()
	if err := EncodeBitString(w, Aligned, 0, 8, true, true, true, data, 8); err != nil {
		t.Fatal(err)
	}

	r := NewReader(w.Bytes())

	got, nbits, err := DecodeBitString(r, Aligned, 0, 8, true, true, true)
	if err != nil || nbits != 8 || !bytes.Equal(got, data) {
		t.Fatalf("in-root: nbits=%d data=%x err=%v", nbits, got, err)
	}

	// out of root: 16 bits, ext bit 1 -> semi-constrained length + value
	data16 := []byte{0xAB, 0xCD}

	w = NewWriter()
	if err := EncodeBitString(w, Aligned, 0, 8, true, true, true, data16, 16); err != nil {
		t.Fatal(err)
	}

	r = NewReader(w.Bytes())

	got, nbits, err = DecodeBitString(r, Aligned, 0, 8, true, true, true)
	if err != nil || nbits != 16 || !bytes.Equal(got, data16) {
		t.Fatalf("out-of-root: nbits=%d data=%x err=%v", nbits, got, err)
	}
}

func TestOctetStringDecodeFixedPaths(t *testing.T) {
	t.Parallel()
	// fixed 1 octet (<=2, no align)
	data1 := []byte{0x42}

	w := NewWriter()
	if err := EncodeOctetString(w, Aligned, 1, 1, true, true, false, data1); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	r := NewReader(w.Bytes())

	got, err := DecodeOctetString(r, Aligned, 1, 1, true, true, false)
	if err != nil || !bytes.Equal(got, data1) {
		t.Fatalf("fixed1: %x err=%v", got, err)
	}

	// fixed 4 octets (>2, octet-aligned)
	data4 := []byte{1, 2, 3, 4}

	w = NewWriter()
	if err := EncodeOctetString(w, Aligned, 4, 4, true, true, false, data4); err != nil {
		t.Fatal(err)
	}

	r = NewReader(w.Bytes())

	got, err = DecodeOctetString(r, Aligned, 4, 4, true, true, false)
	if err != nil || !bytes.Equal(got, data4) {
		t.Fatalf("fixed4: %x err=%v", got, err)
	}

	// fixed 0 octets
	w = NewWriter()
	if err := EncodeOctetString(w, Aligned, 0, 0, true, true, false, nil); err != nil {
		t.Fatal(err)
	}

	if w.Bits() != 0 {
		t.Fatalf("fixed0: wrote %d bits", w.Bits())
	}
}

func TestOctetStringExtensible(t *testing.T) {
	t.Parallel()
	// OCTET STRING (SIZE (0..4), ...) with 6 bytes -> out of root
	data := []byte{1, 2, 3, 4, 5, 6}

	w := NewWriter()
	if err := EncodeOctetString(w, Aligned, 0, 4, true, true, true, data); err != nil {
		t.Fatal(err)
	}

	r := NewReader(w.Bytes())

	got, err := DecodeOctetString(r, Aligned, 0, 4, true, true, true)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("out-of-root: %x err=%v", got, err)
	}
}

func TestUnconstrainedLengthFormB(t *testing.T) {
	t.Parallel()
	// bare unconstrained length n=130 -> form b 0x80 0x82
	w := NewWriter()
	if err := EncodeUnconstrainedLength(w, Aligned, 130); err != nil {
		t.Fatal(err)
	}

	got := w.Bytes()

	want := []byte{0x80, 0x82}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}

	r := NewReader(got)

	n, err := DecodeUnconstrainedLength(r, Aligned)
	if err != nil || n != 130 {
		t.Fatalf("decode: n=%d err=%v", n, err)
	}
}

func TestUnconstrainedLengthOverflow(t *testing.T) {
	t.Parallel()

	w := NewWriter()
	if err := EncodeUnconstrainedLength(w, Aligned, 16384); err != ErrOverflow {
		t.Fatalf("err = %v, want ErrOverflow", err)
	}

	r := NewReader([]byte{0xC0, 0x00})
	if _, err := DecodeUnconstrainedLength(r, Aligned); err != ErrOverflow {
		t.Fatalf("err = %v, want ErrOverflow", err)
	}
}

func TestConstrainedWholeNumberIndefiniteOverflow(t *testing.T) {
	t.Parallel()

	w := NewWriter()
	if err := EncodeConstrainedWholeNumber(w, Aligned, 0, 65536, 0); err != ErrOverflow {
		t.Fatalf("err = %v, want ErrOverflow", err)
	}

	r := NewReader(nil)
	if _, err := DecodeConstrainedWholeNumber(r, Aligned, 0, 65536); err != ErrOverflow {
		t.Fatalf("err = %v, want ErrOverflow", err)
	}
}

func TestSemiConstrainedOverflow(t *testing.T) {
	t.Parallel()

	w := NewWriter()
	if err := encodeSemiConstrained(w, Aligned, 10, 5); err != ErrOverflow {
		t.Fatalf("err = %v, want ErrOverflow", err)
	}
}

func TestOIDParseTruncated(t *testing.T) {
	t.Parallel()

	if _, err := oidParseContent(nil); err != ErrTruncated {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
	// continuation bit set but no following octet
	if _, err := oidParseContent([]byte{0x2B, 0x80}); err != ErrTruncated {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
}

func TestTwosCompOctetWidths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		n      int64
		octets int
	}{
		{0, 1},
		{127, 1},
		{128, 2},
		{255, 2},
		{32767, 2},
		{32768, 3},
		{-1, 1},
		{-128, 1},
		{-129, 2},
		{-32768, 2},
		{-32769, 3},
	}
	for _, c := range cases {
		if got := minOctetsTwosComp(c.n); got != c.octets {
			t.Errorf("minOctetsTwosComp(%d) = %d, want %d", c.n, got, c.octets)
		}
	}
}

func TestWriteBitsPanics(t *testing.T) {
	t.Parallel()

	w := NewWriter()

	for _, bad := range []int{-1, 65} {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("expected panic for n=%d", bad)
				}
			}()

			w.WriteBits(0, bad)
		}()
	}
}

func TestWriterBuf(t *testing.T) {
	t.Parallel()

	w := NewWriter()
	w.WriteBits(0xFF, 8)

	if len(w.Buf()) != 1 || w.Buf()[0] != 0xFF {
		t.Fatalf("Buf = %x", w.Buf())
	}
}
