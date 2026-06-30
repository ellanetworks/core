// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

import "fmt"

// Unbounded marks a string length constraint with no upper bound.
const Unbounded = -1

// stringNeedsAlign reports whether string content is octet-aligned. Per the
// X.691 NOTE 1 rule, a fixed length of 16 bits or fewer stays in the
// bit-field; any variable length or longer fixed length is aligned.
func stringNeedsAlign(lb, ub, unitBits int) bool {
	return lb != ub || ub*unitBits > 16
}

// putStringUnits writes nUnits units of unitBits bits each from data,
// most-significant bit first. For bit strings the final partial octet's high
// bits carry the value.
func (w *Writer) putStringUnits(data []byte, nUnits, unitBits int) {
	nbits := nUnits * unitBits
	full := nbits / 8
	w.WriteOctets(data[:full])

	if rem := nbits % 8; rem > 0 {
		w.WriteBits(uint64(data[full])>>uint(8-rem), rem)
	}
}

func (w *Writer) putString(data []byte, nUnits, lb, ub, unitBits int, ext bool) error {
	inRoot := ub < 0 || (nUnits >= lb && nUnits <= ub)
	if ub >= 0 && !inRoot && !ext {
		return fmt.Errorf("aper: string size %d outside [%d, %d]", nUnits, lb, ub)
	}

	if ext {
		w.WriteBool(!inRoot)
	}

	if ub < 0 || !inRoot {
		// Unbounded form (X.691); also carries out-of-root values. The
		// length determinant aligns, so content follows octet-aligned.
		if err := w.WriteLength(nUnits); err != nil {
			return err
		}

		w.putStringUnits(data, nUnits, unitBits)

		return nil
	}

	if lb != ub {
		if err := w.WriteConstrainedLength(nUnits-lb, 0, ub-lb); err != nil {
			return err
		}
	}

	if stringNeedsAlign(lb, ub, unitBits) {
		w.Align()
	}

	w.putStringUnits(data, nUnits, unitBits)

	return nil
}

// getString reads a string's length and aligns the reader at the start of its
// content, returning the number of units.
func (r *Reader) getString(lb, ub, unitBits int, ext bool) (int, error) {
	inRoot := true

	if ext {
		b, err := r.ReadBool()
		if err != nil {
			return 0, err
		}

		inRoot = !b
	}

	if ub < 0 || !inRoot {
		return r.ReadLength() // aligns; content follows aligned
	}

	if lb != ub {
		n, err := r.ReadConstrainedLength(0, ub-lb)
		if err != nil {
			return 0, err
		}

		if err := r.Align(); err != nil {
			return 0, err
		}

		return lb + n, nil
	}

	if stringNeedsAlign(lb, ub, unitBits) {
		if err := r.Align(); err != nil {
			return 0, err
		}
	}

	return lb, nil
}

// WriteOctetString encodes an OCTET STRING of b whose length in octets is
// constrained to [lb, ub] (X.691). Pass ub = [Unbounded] for no upper
// bound and ext = true when the size constraint is extensible. A fixed length
// of two octets or fewer stays in the bit-field; longer content is aligned.
func (w *Writer) WriteOctetString(b []byte, lb, ub int, ext bool) error {
	return w.putString(b, len(b), lb, ub, 8, ext)
}

// ReadOctetString decodes an OCTET STRING constrained to [lb, ub] octets.
func (r *Reader) ReadOctetString(lb, ub int, ext bool) ([]byte, error) {
	n, err := r.getString(lb, ub, 8, ext)
	if err != nil {
		return nil, err
	}

	return r.ReadOctets(n)
}

// WriteBitString encodes a BIT STRING of nbits bits taken MSB-first from b,
// whose length is constrained to [lb, ub] bits (X.691). b must hold at
// least ceil(nbits/8) octets. A fixed length of 16 bits or fewer stays in the
// bit-field; longer content is aligned.
func (w *Writer) WriteBitString(b []byte, nbits, lb, ub int, ext bool) error {
	if need := (nbits + 7) / 8; len(b) < need {
		return fmt.Errorf("aper: bit string has %d octets, need %d for %d bits", len(b), need, nbits)
	}

	return w.putString(b, nbits, lb, ub, 1, ext)
}

// ReadBitString decodes a BIT STRING constrained to [lb, ub] bits, returning
// the octets (MSB-first, final octet zero-padded) and the bit count.
func (r *Reader) ReadBitString(lb, ub int, ext bool) ([]byte, int, error) {
	n, err := r.getString(lb, ub, 1, ext)
	if err != nil {
		return nil, 0, err
	}

	if n > r.BitsLeft() {
		return nil, 0, &DecodeError{Offset: r.bits, Msg: "bit string exceeds input"}
	}

	out := make([]byte, (n+7)/8)
	full := n / 8

	chunk, err := r.ReadOctets(full)
	if err != nil {
		return nil, 0, err
	}

	copy(out, chunk)

	if rem := n % 8; rem > 0 {
		b, err := r.ReadBits(rem)
		if err != nil {
			return nil, 0, err
		}

		out[full] = byte(b << uint(8-rem))
	}

	return out, n, nil
}
