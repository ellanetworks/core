// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

// WriteLength encodes an unconstrained length determinant (X.691).
// Counts up to 16383 use the one- or two-octet form; larger counts require the
// 16K-fragmented form and return [ErrFragmented].
func (w *Writer) WriteLength(n int) error {
	if n < 0 {
		return errOutOfRange("length", int64(n), 0, 0x3fff)
	}

	w.Align()

	switch {
	case n <= 0x7f:
		w.WriteBits(uint64(n), 8)
	case n <= 0x3fff:
		w.WriteBit(1)
		w.WriteBit(0)
		w.WriteBits(uint64(n), 14)
	default:
		return ErrFragmented
	}

	return nil
}

// ReadLength decodes an unconstrained length determinant (X.691).
func (r *Reader) ReadLength() (int, error) {
	if err := r.Align(); err != nil {
		return 0, err
	}

	b0, err := r.ReadBits(8)
	if err != nil {
		return 0, err
	}

	if b0&0x80 == 0 {
		return int(b0), nil
	}

	if b0&0x40 == 0 {
		b1, err := r.ReadBits(8)
		if err != nil {
			return 0, err
		}

		return int((b0&0x3f)<<8 | b1), nil
	}

	return 0, ErrFragmented
}

// WriteConstrainedLength encodes a length/count constrained to [lb, ub]. When
// ub is below 64K the count is a constrained whole number; otherwise it falls
// back to the unconstrained determinant (X.691).
func (w *Writer) WriteConstrainedLength(n, lb, ub int) error {
	if ub >= 65536 {
		return w.WriteLength(n)
	}

	return w.WriteConstrainedInt(int64(n), int64(lb), int64(ub))
}

// ReadConstrainedLength decodes a length/count constrained to [lb, ub].
func (r *Reader) ReadConstrainedLength(lb, ub int) (int, error) {
	if ub >= 65536 {
		return r.ReadLength()
	}

	v, err := r.ReadConstrainedInt(int64(lb), int64(ub))

	return int(v), err
}
