// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "encoding/binary"

// Writer accumulates a NAS octet string. The zero value is ready to use and
// starts from an empty buffer; NewWriter appends to an existing one.
//
// A framing error — a value too long for the length prefix that must precede it
// — is recorded rather than returned. The Writer is poisoned from that point on:
// every subsequent write is dropped, and Bytes reports the first error. Encoders
// therefore run straight-line, with one error check at the end.
type Writer struct {
	buf []byte
	err error
}

// NewWriter returns a Writer that appends to b. b may be nil. Ownership of b
// passes to the Writer until Bytes returns it.
func NewWriter(b []byte) *Writer { return &Writer{buf: b} }

// U8 appends one octet.
func (w *Writer) U8(v uint8) {
	if w.err != nil {
		return
	}

	w.buf = append(w.buf, v)
}

// U16 appends a big-endian 16-bit value.
func (w *Writer) U16(v uint16) {
	if w.err != nil {
		return
	}

	w.buf = binary.BigEndian.AppendUint16(w.buf, v)
}

// Raw appends value octets verbatim (IE formats V / TV value part).
func (w *Writer) Raw(b []byte) {
	if w.err != nil {
		return
	}

	w.buf = append(w.buf, b...)
}

// LV writes b prefixed by a 1-octet length. A value longer than 255 octets
// poisons the Writer.
func (w *Writer) LV(b []byte) {
	if w.err != nil {
		return
	}

	if len(b) > 0xFF {
		w.fail("LV length")

		return
	}

	w.U8(uint8(len(b)))
	w.Raw(b)
}

// LVE writes b prefixed by a 2-octet length. A value longer than 65535 octets
// poisons the Writer.
func (w *Writer) LVE(b []byte) {
	if w.err != nil {
		return
	}

	if len(b) > 0xFFFF {
		w.fail("LV-E length")

		return
	}

	w.U16(uint16(len(b)))
	w.Raw(b)
}

// LVFunc writes the octets fn produces, prefixed by a 1-octet length. It saves
// the caller from assembling nested content in a second Writer to measure it.
func (w *Writer) LVFunc(fn func(*Writer)) {
	w.LV(w.content(fn))
}

// LVEFunc writes the octets fn produces, prefixed by a 2-octet length.
func (w *Writer) LVEFunc(fn func(*Writer)) {
	w.LVE(w.content(fn))
}

// content runs fn against a fresh Writer and returns its octets, propagating a
// failure to w so the caller never sees a partial value.
func (w *Writer) content(fn func(*Writer)) []byte {
	if w.err != nil {
		return nil
	}

	var inner Writer

	fn(&inner)

	if inner.err != nil {
		w.err = inner.err

		return nil
	}

	return inner.buf
}

// TV3 writes an optional type-3 IE with a single value octet: the IEI followed
// by the value (TS 24.007 §11.2.1.1.3).
func (w *Writer) TV3(iei, value uint8) {
	w.U8(iei)
	w.U8(value)
}

// TLV writes an optional type-4 IE: the IEI, a 1-octet length, and the value.
func (w *Writer) TLV(iei uint8, value []byte) {
	w.U8(iei)
	w.LV(value)
}

// TLVE writes an optional type-6 IE: the IEI, a 2-octet length, and the value.
func (w *Writer) TLVE(iei uint8, value []byte) {
	w.U8(iei)
	w.LVE(value)
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

// Bytes returns the accumulated octets and the first framing error, if any.
// Ownership of the slice passes to the caller: it is the buffer NewWriter was
// given, extended, and the Writer must not be used afterwards.
//
// On error the octets are those written before the failure, which is a partial
// encoding no peer would accept. Encoders return through Result instead, so a
// caller who ignores the error cannot transmit one.
func (w *Writer) Bytes() ([]byte, error) { return w.buf, w.err }

// Result returns the accumulated octets for an AppendBinary whose caller passed
// orig, or orig unchanged and the first framing error.
//
// Returning the caller's buffer untouched on failure is what makes the append
// form safe to chain: a failed element leaves the octets already built intact,
// and no partial encoding of the failed one is appended. It matches the standard
// library's appenders, whose contract is to leave the input alone on error.
func (w *Writer) Result(orig []byte) ([]byte, error) {
	if w.err != nil {
		return orig, w.err
	}

	return w.buf, nil
}

func (w *Writer) fail(op string) {
	w.err = &Error{Op: op, Offset: len(w.buf), Err: ErrOverflow}
}
