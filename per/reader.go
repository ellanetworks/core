// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import "io"

// Reader is a bit-oriented input buffer. Bits are read most-significant-bit
// first within each octet.
type Reader struct {
	buf []byte
	pos int
	bit uint8 // 0..7: bit position within buf[pos]
}

// NewReader returns a Reader over b without copying it.
func NewReader(b []byte) *Reader { return &Reader{buf: b} }

// Bits returns the number of bits remaining.
func (r *Reader) Bits() int {
	return (len(r.buf)-r.pos)*8 - int(r.bit)
}

func (r *Reader) Aligned() bool { return r.bit == 0 }

func (r *Reader) EOF() bool { return r.Bits() <= 0 }

// AlignToByte skips any remaining bits in the current octet.
func (r *Reader) AlignToByte() {
	if r.bit != 0 {
		r.bit = 0
		r.pos++
	}
}

func (r *Reader) ReadBit() (bool, error) {
	if r.pos >= len(r.buf) {
		return false, ErrTruncated
	}

	b := r.buf[r.pos]&(1<<(7-r.bit)) != 0

	r.bit++
	if r.bit == 8 {
		r.bit = 0
		r.pos++
	}

	return b, nil
}

// ReadBits reads n bits (n ≤ 64) into a uint64, MSB first.
func (r *Reader) ReadBits(n int) (uint64, error) {
	if n < 0 || n > 64 {
		return 0, io.ErrUnexpectedEOF
	}

	if r.Bits() < n {
		return 0, ErrTruncated
	}

	var v uint64

	for i := n - 1; i >= 0; i-- {
		bit, err := r.ReadBit()
		if err != nil {
			return 0, err
		}

		if bit {
			v |= 1 << i
		}
	}

	return v, nil
}

// ReadBitString reads nbits into ceil(nbits/8) bytes, MSB first and
// left-aligned; trailing bits of the final octet are zero.
func (r *Reader) ReadBitString(nbits int) ([]byte, error) {
	if nbits < 0 {
		return nil, io.ErrUnexpectedEOF
	}

	if r.Bits() < nbits {
		return nil, ErrTruncated
	}

	out := make([]byte, (nbits+7)/8)
	for i := range nbits {
		bit, err := r.ReadBit()
		if err != nil {
			return nil, err
		}

		if bit {
			out[i/8] |= 1 << (7 - uint(i%8))
		}
	}

	return out, nil
}

// ReadOctets reads n whole octets. The reader must be octet-aligned.
func (r *Reader) ReadOctets(n int) ([]byte, error) {
	if r.bit != 0 {
		return nil, ErrUnaligned
	}

	if n < 0 || r.pos+n > len(r.buf) {
		return nil, ErrTruncated
	}

	out := make([]byte, n)
	copy(out, r.buf[r.pos:r.pos+n])
	r.pos += n

	return out, nil
}
