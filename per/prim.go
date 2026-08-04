// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

// Bounds describes the PER-visible constraints on an INTEGER (§11.5–11.8, §13).
// A bound is absent where MIN or MAX applies.
type Bounds struct {
	LB, UB       int64
	HasLB, HasUB bool
	Extensible   bool
}

func (b Bounds) Constrained() bool { return b.HasLB && b.HasUB }

// EncodeBoolean encodes a BOOLEAN (§12).
func EncodeBoolean(w *Writer, _ Encoding, v bool) {
	w.WriteBit(v)
}

// DecodeBoolean decodes a BOOLEAN (§12).
func DecodeBoolean(r *Reader, _ Encoding) (bool, error) {
	return r.ReadBit()
}

// EncodeNull encodes a NULL (§18): nothing is added to the field-list.
func EncodeNull(*Writer, Encoding) error { return nil }

// DecodeNull decodes a NULL (§18): no bits are consumed.
func DecodeNull(*Reader, Encoding) error { return nil }

// EncodeInteger encodes an INTEGER (§13), dispatching to the constrained
// (§11.5), semi-constrained (§11.7) or unconstrained (§11.8) procedure, plus
// the extensible (§13.1) and indefinite (§13.2.6) cases.
func EncodeInteger(w *Writer, enc Encoding, b Bounds, n int64) error {
	if b.Extensible {
		inRoot := !b.Constrained() || (n >= b.LB && n <= b.UB)
		w.WriteBit(!inRoot)

		if !inRoot {
			return encodeUnconstrainedInteger(w, enc, n)
		}
	}

	switch {
	case b.Constrained():
		rng := b.UB - b.LB + 1
		if rng == 1 {
			// §13.2.1 adds nothing to the field-list, which presupposes the
			// value is the single permitted one. Checking here because the
			// early return skips EncodeConstrainedWholeNumber's range check.
			if n != b.LB {
				return ErrOverflow
			}

			return nil
		}

		if enc == Unaligned || rng <= sixtyFourK {
			return EncodeConstrainedWholeNumber(w, enc, b.LB, b.UB, n)
		}

		return encodeIndefiniteInteger(w, enc, b.LB, rng, n)
	case b.HasLB:
		return encodeSemiConstrained(w, enc, b.LB, n)
	default:
		return encodeUnconstrainedInteger(w, enc, n)
	}
}

// DecodeInteger decodes an INTEGER (§13).
func DecodeInteger(r *Reader, enc Encoding, b Bounds) (int64, error) {
	if b.Extensible {
		bit, err := r.ReadBit()
		if err != nil {
			return 0, err
		}

		if bit {
			return decodeUnconstrainedInteger(r, enc)
		}
	}

	switch {
	case b.Constrained():
		rng := b.UB - b.LB + 1
		if rng == 1 {
			return b.LB, nil
		}

		if enc == Unaligned || rng <= sixtyFourK {
			return DecodeConstrainedWholeNumber(r, enc, b.LB, b.UB)
		}

		return decodeIndefiniteInteger(r, enc, b.LB, rng)
	case b.HasLB:
		return decodeSemiConstrained(r, enc, b.LB)
	default:
		return decodeUnconstrainedInteger(r, enc)
	}
}

// encodeIndefiniteInteger handles the aligned-variant indefinite case
// (§11.5.7.4, §13.2.6a): n-lb as a minimal non-negative binary integer,
// preceded by a length determinant constrained to [1, octets holding the range].
func encodeIndefiniteInteger(w *Writer, enc Encoding, lb, rng, n int64) error {
	if n < lb || n > lb+rng-1 {
		return ErrOverflow
	}

	v := uint64(n - lb)
	octets := minOctetsNonNeg(v)

	buf := make([]byte, octets)
	for i := range octets {
		buf[octets-1-i] = byte(v >> (8 * i))
	}

	rangeOctets := int64(minOctetsNonNeg(uint64(rng - 1)))

	return EncodeLength(w, enc, 1, rangeOctets, true, int64(octets), func(int64) error {
		writeOctetAligned(w, enc, buf)
		return nil
	})
}

func decodeIndefiniteInteger(r *Reader, enc Encoding, lb, rng int64) (int64, error) {
	rangeOctets := int64(minOctetsNonNeg(uint64(rng - 1)))

	var v uint64

	err := DecodeLength(r, enc, 1, rangeOctets, true, func(count int64) error {
		p, err := readOctetAligned(r, enc, int(count))
		if err != nil {
			return err
		}

		for _, bb := range p {
			v = v<<8 | uint64(bb)
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	if v > uint64(rng-1) {
		return 0, ErrOverflow
	}

	return lb + int64(v), nil
}

// encodeUnconstrainedInteger encodes n as a 2's-complement minimum-octet field
// preceded by an unconstrained length determinant (§11.8, §13.2.6b).
func encodeUnconstrainedInteger(w *Writer, enc Encoding, n int64) error {
	octets := minOctetsTwosComp(n)

	buf := make([]byte, octets)
	for i := range octets {
		buf[octets-1-i] = byte(n >> (8 * i))
	}

	if err := EncodeUnconstrainedLength(w, enc, int64(octets)); err != nil {
		return err
	}

	writeOctetAligned(w, enc, buf)

	return nil
}

func decodeUnconstrainedInteger(r *Reader, enc Encoding) (int64, error) {
	n, err := DecodeUnconstrainedLength(r, enc)
	if err != nil {
		return 0, err
	}

	// §11.8: a zero-octet field has no value, and more than 8 octets exceed int64.
	if n < 1 || n > 8 {
		return 0, ErrOverflow
	}

	p, err := readOctetAligned(r, enc, int(n))
	if err != nil {
		return 0, err
	}

	var v int64
	for _, b := range p {
		v = v<<8 | int64(b)
	}

	bits := len(p) * 8
	if bits > 0 && bits < 64 && v&(1<<(bits-1)) != 0 {
		v |= ^((1 << bits) - 1)
	}

	return v, nil
}
