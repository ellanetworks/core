// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

import "fmt"

// Reader decodes PER primitives from an octet buffer. Every read is
// bounds-checked; a read past the end returns a [DecodeError] and does not
// panic.
type Reader struct {
	buf       []byte
	bits      int // total bits consumed
	unaligned bool
}

// NewReader returns a Reader over b decoding the ALIGNED variant of X.691.
func NewReader(b []byte) *Reader {
	return &Reader{buf: b}
}

// NewUnalignedReader returns a Reader over b decoding the UNALIGNED variant of
// X.691, in which no production is padded to an octet boundary. LPP requires it
// (TS 37.355 §5: "BASIC-PER, Unaligned Variant"), whereas S1AP/NGAP/NRPPa use
// the aligned variant.
func NewUnalignedReader(b []byte) *Reader {
	return &Reader{buf: b, unaligned: true}
}

// Unaligned reports whether the Reader decodes the unaligned variant.
func (r *Reader) Unaligned() bool {
	return r.unaligned
}

// BitsLeft returns the number of unread bits.
func (r *Reader) BitsLeft() int {
	return len(r.buf)*8 - r.bits
}

// ReadBit reads a single bit.
func (r *Reader) ReadBit() (uint, error) {
	if r.bits >= len(r.buf)*8 {
		return 0, &DecodeError{Offset: r.bits, Msg: "unexpected end of input"}
	}

	b := (r.buf[r.bits/8] >> uint(7-r.bits%8)) & 1
	r.bits++

	return uint(b), nil
}

// ReadBits reads n bits, most significant first, into the low bits of the
// result (0 <= n <= 64).
func (r *Reader) ReadBits(n int) (uint64, error) {
	if n < 0 || n > 64 {
		return 0, &DecodeError{Offset: r.bits, Msg: fmt.Sprintf("invalid bit count %d", n)}
	}

	if n > r.BitsLeft() {
		return 0, &DecodeError{Offset: r.bits, Msg: "unexpected end of input"}
	}

	var v uint64

	for i := 0; i < n; i++ {
		bit, _ := r.ReadBit()
		v = v<<1 | uint64(bit)
	}

	return v, nil
}

// ReadBool reads a single boolean bit (X.691).
func (r *Reader) ReadBool() (bool, error) {
	b, err := r.ReadBit()
	return b == 1, err
}

// Align discards bits up to the next octet boundary (X.691). The unaligned
// variant has no octet-alignment points, so this is a no-op there.
func (r *Reader) Align() error {
	if r.unaligned {
		return nil
	}

	for r.bits%8 != 0 {
		if _, err := r.ReadBit(); err != nil {
			return err
		}
	}

	return nil
}

// ReadOctets reads n raw octets. The count is validated against the remaining
// input before allocation, so a corrupt length cannot trigger a large alloc.
func (r *Reader) ReadOctets(n int) ([]byte, error) {
	if n < 0 {
		return nil, &DecodeError{Offset: r.bits, Msg: fmt.Sprintf("negative octet count %d", n)}
	}

	if n > r.BitsLeft()/8 {
		return nil, &DecodeError{Offset: r.bits, Msg: "octet string exceeds input"}
	}

	out := make([]byte, n)
	if r.bits%8 == 0 {
		copy(out, r.buf[r.bits/8:r.bits/8+n])
		r.bits += 8 * n

		return out, nil
	}

	for i := 0; i < n; i++ {
		b, _ := r.ReadBits(8)
		out[i] = byte(b)
	}

	return out, nil
}
