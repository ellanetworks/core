// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import "encoding/binary"

// Reader consumes a NAS octet string left to right. Every method is
// bounds-checked: a read past the end returns an *Error wrapping ErrTruncated
// and never panics.
type Reader struct {
	buf []byte
	pos int
}

// NewReader returns a Reader over b. b is not copied; callers must not mutate it
// while the Reader is in use.
func NewReader(b []byte) *Reader { return &Reader{buf: b} }

// Offset is the number of octets already consumed.
func (r *Reader) Offset() int { return r.pos }

// Remaining is the number of octets left to read.
func (r *Reader) Remaining() int { return len(r.buf) - r.pos }

func (r *Reader) need(n int, op string) error {
	if n < 0 || r.pos+n > len(r.buf) {
		return &Error{Op: op, Offset: r.pos, Err: ErrTruncated}
	}

	return nil
}

// U8 reads one octet.
func (r *Reader) U8() (uint8, error) {
	if err := r.need(1, "uint8"); err != nil {
		return 0, err
	}

	v := r.buf[r.pos]
	r.pos++

	return v, nil
}

// PeekU8 returns the next octet without consuming it.
func (r *Reader) PeekU8() (uint8, error) {
	if err := r.need(1, "peek"); err != nil {
		return 0, err
	}

	return r.buf[r.pos], nil
}

// U16 reads a big-endian 16-bit value.
func (r *Reader) U16() (uint16, error) {
	if err := r.need(2, "uint16"); err != nil {
		return 0, err
	}

	v := binary.BigEndian.Uint16(r.buf[r.pos:])
	r.pos += 2

	return v, nil
}

// Bytes reads n octets and returns a copy.
func (r *Reader) Bytes(n int) ([]byte, error) {
	if err := r.need(n, "bytes"); err != nil {
		return nil, err
	}

	out := make([]byte, n)
	copy(out, r.buf[r.pos:r.pos+n])
	r.pos += n

	return out, nil
}

// LV reads a value prefixed by a 1-octet length (TS 24.007, used by
// IE formats LV and TLV after the IEI is consumed).
func (r *Reader) LV() ([]byte, error) {
	n, err := r.U8()
	if err != nil {
		return nil, err
	}

	return r.Bytes(int(n))
}

// LVE reads a value prefixed by a 2-octet length (used by IE formats LV-E and
// TLV-E — EPS/5GS containers).
func (r *Reader) LVE() ([]byte, error) {
	n, err := r.U16()
	if err != nil {
		return nil, err
	}

	return r.Bytes(int(n))
}
