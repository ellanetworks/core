package per

// Writer is a bit-oriented output buffer. Bits are written most-significant-bit
// first within each octet, as required by PER.
//
// A Writer tracks whether the current position is octet-aligned via [Writer.Aligned];
// the [Writer.AlignToByte] method inserts zero padding bits up to the next octet
// boundary, which is how Aligned PER inserts padding. Unaligned PER callers simply
// never call AlignToByte between fields.
type Writer struct {
	buf []byte
	bit uint8 // 0..7: number of bits filled in the current (last) partial octet
}

// NewWriter returns an empty Writer.
func NewWriter() *Writer { return &Writer{} }

// Bits returns the total number of bits written so far.
func (w *Writer) Bits() int {
	if w.bit == 0 {
		return len(w.buf) * 8
	}

	return (len(w.buf)-1)*8 + int(w.bit)
}

// Aligned reports whether the current position is on an octet boundary.
func (w *Writer) Aligned() bool { return w.bit == 0 }

// Buf returns the internal buffer (do not retain across further writes).
func (w *Writer) Buf() []byte { return w.buf }

// Bytes returns the encoded octets. It panics if the writer is not octet-aligned,
// since a partial octet has no well-formed PER representation at the top level.
func (w *Writer) Bytes() []byte {
	if w.bit != 0 {
		panic("per: Bytes() called on non-octet-aligned writer")
	}

	out := make([]byte, len(w.buf))
	copy(out, w.buf)

	return out
}

// WriteBit appends a single bit.
func (w *Writer) WriteBit(v bool) {
	if w.bit == 0 {
		w.buf = append(w.buf, 0)
	}

	if v {
		w.buf[len(w.buf)-1] |= 1 << (7 - w.bit)
	}

	w.bit++
	if w.bit == 8 {
		w.bit = 0
	}
}

// WriteBits writes the n least-significant bits of v, most-significant first.
// It panics if n < 0 or n > 64.
func (w *Writer) WriteBits(v uint64, n int) {
	if n < 0 || n > 64 {
		panic("per: WriteBits: invalid bit count")
	}

	for i := n - 1; i >= 0; i-- {
		w.WriteBit(v&(1<<i) != 0)
	}
}

// WriteBitString writes nbits bits from data, MSB-first. nbits must not exceed
// len(data)*8. Trailing bits of the final octet beyond nbits are ignored.
func (w *Writer) WriteBitString(data []byte, nbits int) {
	if nbits < 0 || nbits > len(data)*8 {
		panic("per: WriteBitString: nbits out of range")
	}

	for i := range nbits {
		w.WriteBit(data[i/8]&(1<<(7-uint(i%8))) != 0)
	}
}

// WriteOctets writes whole octets. The writer must be octet-aligned.
func (w *Writer) WriteOctets(p []byte) error {
	if w.bit != 0 {
		return ErrUnaligned
	}

	w.buf = append(w.buf, p...)

	return nil
}

// AlignToByte inserts zero bits until the position is on an octet boundary.
// It is a no-op when already aligned. Used by Aligned PER.
func (w *Writer) AlignToByte() {
	for w.bit != 0 {
		w.WriteBit(false)
	}
}
