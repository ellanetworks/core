// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import "encoding/binary"

// Writer accumulates a NAS octet string. The zero value is ready to use.
type Writer struct {
	buf []byte
}

// U8 appends one octet.
func (w *Writer) U8(v uint8) { w.buf = append(w.buf, v) }

// U16 appends a big-endian 16-bit value.
func (w *Writer) U16(v uint16) { w.buf = binary.BigEndian.AppendUint16(w.buf, v) }

// Raw appends value octets verbatim (IE formats V / TV value part).
func (w *Writer) Raw(b []byte) { w.buf = append(w.buf, b...) }

// LV writes b prefixed by a 1-octet length. It errors if b exceeds 255 octets.
func (w *Writer) LV(b []byte) error {
	if len(b) > 0xFF {
		return &Error{Op: "LV length", Offset: len(w.buf), Err: ErrOverflow}
	}

	w.U8(uint8(len(b)))
	w.Raw(b)

	return nil
}

// LVE writes b prefixed by a 2-octet length. It errors if b exceeds 65535 octets.
func (w *Writer) LVE(b []byte) error {
	if len(b) > 0xFFFF {
		return &Error{Op: "LV-E length", Offset: len(w.buf), Err: ErrOverflow}
	}

	w.U16(uint16(len(b)))
	w.Raw(b)

	return nil
}

// Truncate discards all but the first n octets. n is clamped to the current
// length, so it never grows the buffer.
func (w *Writer) Truncate(n int) {
	if n < len(w.buf) {
		w.buf = w.buf[:n]
	}
}

// Len is the number of octets written so far.
func (w *Writer) Len() int { return len(w.buf) }

// Bytes returns the accumulated octets. The slice aliases the Writer's buffer.
func (w *Writer) Bytes() []byte { return w.buf }
