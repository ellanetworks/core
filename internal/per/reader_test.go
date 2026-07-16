package per

import (
	"bytes"
	"testing"
)

func TestReaderReadBit(t *testing.T) {
	t.Parallel()

	r := NewReader([]byte{0b10110010, 0b10000000})

	want := []bool{true, false, true, true, false, false, true, false, true}
	for i, w := range want {
		got, err := r.ReadBit()
		if err != nil {
			t.Fatalf("bit %d: %v", i, err)
		}

		if got != w {
			t.Fatalf("bit %d: got %v, want %v", i, got, w)
		}
	}

	if r.Bits() != 7 {
		t.Fatalf("remaining = %d, want 7", r.Bits())
	}
}

func TestReaderReadBits(t *testing.T) {
	t.Parallel()

	r := NewReader([]byte{0b10101010, 0b11001100})

	v, err := r.ReadBits(12)
	if err != nil {
		t.Fatal(err)
	}

	if v != 0b101010101100 {
		t.Fatalf("v = %012b, want %012b", v, 0b101010101100)
	}

	if r.Bits() != 4 {
		t.Fatalf("remaining = %d, want 4", r.Bits())
	}
}

func TestReaderReadBitString(t *testing.T) {
	t.Parallel()

	r := NewReader([]byte{0xAB, 0xC0})

	got, err := r.ReadBitString(12)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, []byte{0xAB, 0xC0}) {
		t.Fatalf("got %x, want [AB C0]", got)
	}
}

func TestReaderAlignToByte(t *testing.T) {
	t.Parallel()

	r := NewReader([]byte{0b10100000, 0b10000000})
	if _, err := r.ReadBits(3); err != nil {
		t.Fatal(err)
	}

	r.AlignToByte()

	if !r.Aligned() {
		t.Fatal("not aligned")
	}

	if r.pos != 1 {
		t.Fatalf("pos = %d, want 1", r.pos)
	}

	bit, err := r.ReadBit()
	if err != nil || !bit {
		t.Fatalf("expected set bit, got %v %v", bit, err)
	}
}

func TestReaderReadOctetsRequiresAlignment(t *testing.T) {
	t.Parallel()

	r := NewReader([]byte{1, 2, 3})
	if _, err := r.ReadBit(); err != nil {
		t.Fatal(err)
	}

	if _, err := r.ReadOctets(1); err != ErrUnaligned {
		t.Fatalf("err = %v, want ErrUnaligned", err)
	}

	r.AlignToByte()

	got, err := r.ReadOctets(2)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, []byte{2, 3}) {
		t.Fatalf("got %v, want [2 3]", got)
	}
}

func TestReaderTruncated(t *testing.T) {
	t.Parallel()

	r := NewReader([]byte{0xFF})
	if _, err := r.ReadBit(); err != nil { // 7 bits remain, not octet-aligned
		t.Fatal(err)
	}

	if _, err := r.ReadBits(8); err != ErrTruncated {
		t.Fatalf("ReadBits: err = %v, want ErrTruncated", err)
	}

	if _, err := r.ReadBitString(8); err != ErrTruncated {
		t.Fatalf("ReadBitString: err = %v, want ErrTruncated", err)
	}

	r.AlignToByte() // consumes the remaining 7 bits

	if _, err := r.ReadOctets(2); err != ErrTruncated {
		t.Fatalf("ReadOctets: err = %v, want ErrTruncated", err)
	}
}

func TestReaderEOF(t *testing.T) {
	t.Parallel()

	r := NewReader(nil)
	if !r.EOF() {
		t.Fatal("empty reader should be EOF")
	}

	if _, err := r.ReadBit(); err != ErrTruncated {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
}

func TestReaderWriteRoundtrip(t *testing.T) {
	t.Parallel()

	w := NewWriter()
	w.WriteBits(0b1100, 4)
	w.AlignToByte()

	if err := w.WriteOctets([]byte{0xDE, 0xAD}); err != nil {
		t.Fatal(err)
	}

	w.WriteBitString([]byte{0xBE, 0xE0}, 9)
	w.AlignToByte()

	r := NewReader(w.Bytes())
	if v, err := r.ReadBits(4); err != nil || v != 0b1100 {
		t.Fatalf("nibble: %v %v", v, err)
	}

	r.AlignToByte()

	if p, err := r.ReadOctets(2); err != nil || !bytes.Equal(p, []byte{0xDE, 0xAD}) {
		t.Fatalf("octets: %x %v", p, err)
	}

	bs, err := r.ReadBitString(9)
	if err != nil {
		t.Fatal(err)
	}

	if bs[0] != 0xBE || bs[1]&0x80 != 0x80 {
		t.Fatalf("bitstring: %x", bs)
	}

	r.AlignToByte()

	if !r.EOF() {
		t.Fatal("expected EOF")
	}
}
