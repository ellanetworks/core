// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

// Writer is a bit-oriented output buffer. Bits are written most-significant-bit
// first within each octet.
type Writer struct {
	buf []byte
	bit uint8 // 0..7: bits filled in the last partial octet
}

func NewWriter() *Writer { return &Writer{} }

// Bits returns the number of bits written so far.
func (w *Writer) Bits() int {
	if w.bit == 0 {
		return len(w.buf) * 8
	}

	return (len(w.buf)-1)*8 + int(w.bit)
}

func (w *Writer) Aligned() bool { return w.bit == 0 }

// Buf returns the internal buffer, invalidated by any further write.
func (w *Writer) Buf() []byte { return w.buf }

// Bytes returns a copy of the encoded octets. It panics unless the writer is
// octet-aligned.
func (w *Writer) Bytes() []byte {
	if w.bit != 0 {
		panic("per: Bytes() called on non-octet-aligned writer")
	}

	out := make([]byte, len(w.buf))
	copy(out, w.buf)

	return out
}

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
// It panics if n > 64.
func (w *Writer) WriteBits(v uint64, n int) {
	if n < 0 || n > 64 {
		panic("per: WriteBits: invalid bit count")
	}

	for i := n - 1; i >= 0; i-- {
		w.WriteBit(v&(1<<i) != 0)
	}
}

// WriteBitString writes nbits bits from data, MSB-first. It panics if nbits
// exceeds len(data)*8.
func (w *Writer) WriteBitString(data []byte, nbits int) {
	if nbits < 0 || nbits > len(data)*8 {
		panic("per: WriteBitString: nbits out of range")
	}

	for i := range nbits {
		w.WriteBit(data[i/8]&(1<<(7-uint(i%8))) != 0)
	}
}

// WriteOctets writes whole octets; the writer must be octet-aligned.
func (w *Writer) WriteOctets(p []byte) error {
	if w.bit != 0 {
		return ErrUnaligned
	}

	w.buf = append(w.buf, p...)

	return nil
}

// AlignToByte pads with zero bits up to the next octet boundary.
func (w *Writer) AlignToByte() {
	for w.bit != 0 {
		w.WriteBit(false)
	}
}
