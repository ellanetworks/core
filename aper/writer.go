// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

// Writer encodes PER primitives into a growing octet buffer. The zero value is
// ready to use and encodes the ALIGNED variant. The final octet is zero-padded,
// as PER requires.
type Writer struct {
	buf       []byte
	bits      int // total bits written
	unaligned bool
}

// NewUnalignedWriter returns a Writer that encodes the UNALIGNED variant of
// X.691, in which no production is padded to an octet boundary. LPP requires it
// (TS 37.355 §5: "BASIC-PER, Unaligned Variant"), whereas S1AP/NGAP/NRPPa use
// the aligned variant the zero value provides.
func NewUnalignedWriter() *Writer {
	return &Writer{unaligned: true}
}

// Unaligned reports whether the Writer encodes the unaligned variant.
func (w *Writer) Unaligned() bool {
	return w.unaligned
}

// WriteBit writes the least significant bit of b.
func (w *Writer) WriteBit(b uint) {
	if w.bits%8 == 0 {
		w.buf = append(w.buf, 0)
	}

	if b&1 != 0 {
		w.buf[w.bits/8] |= 1 << uint(7-w.bits%8)
	}

	w.bits++
}

// WriteBits writes the low n bits of v, most significant first. n must be in
// the range 0..64; the caller must honour this.
func (w *Writer) WriteBits(v uint64, n int) {
	for i := n - 1; i >= 0; i-- {
		w.WriteBit(uint(v >> uint(i)))
	}
}

// WriteBool writes a single boolean bit (X.691).
func (w *Writer) WriteBool(b bool) {
	if b {
		w.WriteBit(1)
	} else {
		w.WriteBit(0)
	}
}

// Align pads with zero bits up to the next octet boundary (X.691). The
// unaligned variant has no octet-alignment points, so this is a no-op there.
func (w *Writer) Align() {
	if w.unaligned {
		return
	}

	for w.bits%8 != 0 {
		w.WriteBit(0)
	}
}

// WriteOctets writes raw octets, fast-pathing the octet-aligned case.
func (w *Writer) WriteOctets(p []byte) {
	if w.bits%8 == 0 {
		w.buf = append(w.buf, p...)
		w.bits += 8 * len(p)

		return
	}

	for _, b := range p {
		w.WriteBits(uint64(b), 8)
	}
}

// Bytes returns the encoded octets.
func (w *Writer) Bytes() []byte {
	return w.buf
}
