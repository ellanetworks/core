// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

import "fmt"

func errOutOfRange(what string, v, lb, ub int64) error {
	return fmt.Errorf("aper: %s value %d outside range [%d, %d]", what, v, lb, ub)
}

// minOctets returns the minimum number of octets needed to hold v (at least 1).
func minOctets(v uint64) int {
	n := 1
	for v >>= 8; v > 0; v >>= 8 {
		n++
	}

	return n
}

// bitsForRange returns the minimum number of bits needed to represent the
// values 0..rang-1, i.e. ceil(log2(rang)); 0 when rang <= 1.
func bitsForRange(rang uint64) int {
	n := 0
	for (uint64(1) << uint(n)) < rang {
		n++
	}

	return n
}

// lowMask returns a mask of the low octets*8 bits.
func lowMask(octets int) uint64 {
	if octets >= 8 {
		return ^uint64(0)
	}

	return (uint64(1) << uint(8*octets)) - 1
}

// WriteConstrainedInt encodes v in the inclusive range [lb, ub] (X.691).
// The bounds are assumed to satisfy ub-lb < 2^63, which holds for every S1AP
// constraint (the widest is INTEGER(0..2^32-1)).
func (w *Writer) WriteConstrainedInt(v, lb, ub int64) error {
	if v < lb || v > ub {
		return errOutOfRange("constrained int", v, lb, ub)
	}

	rang := uint64(ub-lb) + 1
	n := uint64(v - lb)

	if rang == 1 { // single value carries no bits
		return nil
	}

	// X.691 §13.2.6: the unaligned variant always uses a bit-field of the
	// minimum width for the range, with none of the octet forms below.
	if w.unaligned {
		w.WriteBits(n, bitsForRange(rang))
		return nil
	}

	switch {
	case rang <= 255: // bit-field
		w.WriteBits(n, bitsForRange(rang))
	case rang == 256: // one octet, aligned
		w.Align()
		w.WriteBits(n, 8)
	case rang <= 65536: // two octets, aligned
		w.Align()
		w.WriteBits(n, 16)
	default: // octet count, aligned, then the value octets
		valOctets := minOctets(n)
		maxOctets := minOctets(rang - 1)
		w.WriteBits(uint64(valOctets-1), bitsForRange(uint64(maxOctets)))
		w.Align()
		w.WriteBits(n, valOctets*8)
	}

	return nil
}

// ReadConstrainedInt decodes a constrained whole number in [lb, ub]. A value
// that decodes outside the range (for example a bit-field pattern with no
// assigned value) is rejected, matching X.691 strictly.
func (r *Reader) ReadConstrainedInt(lb, ub int64) (int64, error) {
	rang := uint64(ub-lb) + 1

	var n uint64

	if rang == 1 {
		return lb, nil
	}

	// X.691 §13.2.6: the unaligned variant always uses a bit-field of the
	// minimum width for the range, with none of the octet forms below.
	if r.unaligned {
		v, err := r.ReadBits(bitsForRange(rang))
		if err != nil {
			return 0, err
		}

		if v >= rang {
			return 0, &DecodeError{Offset: r.bits, Msg: "constrained int value out of range"}
		}

		return lb + int64(v), nil
	}

	switch {
	case rang <= 255:
		v, err := r.ReadBits(bitsForRange(rang))
		if err != nil {
			return 0, err
		}

		n = v
	case rang == 256:
		if err := r.Align(); err != nil {
			return 0, err
		}

		v, err := r.ReadBits(8)
		if err != nil {
			return 0, err
		}

		n = v
	case rang <= 65536:
		if err := r.Align(); err != nil {
			return 0, err
		}

		v, err := r.ReadBits(16)
		if err != nil {
			return 0, err
		}

		n = v
	default:
		maxOctets := minOctets(rang - 1)

		cnt, err := r.ReadBits(bitsForRange(uint64(maxOctets)))
		if err != nil {
			return 0, err
		}

		valOctets := int(cnt) + 1
		if valOctets > maxOctets {
			return 0, &DecodeError{Offset: r.bits, Msg: "constrained int octet count too large"}
		}

		if err := r.Align(); err != nil {
			return 0, err
		}

		v, err := r.ReadBits(valOctets * 8)
		if err != nil {
			return 0, err
		}

		n = v
	}

	if n >= rang {
		return 0, &DecodeError{Offset: r.bits, Msg: "constrained int value out of range"}
	}

	return lb + int64(n), nil
}

// WriteSemiConstrainedInt encodes v >= lb with no upper bound (X.691).
func (w *Writer) WriteSemiConstrainedInt(v, lb int64) error {
	if v < lb {
		return errOutOfRange("semi-constrained int", v, lb, v)
	}

	n := uint64(v - lb)

	oct := minOctets(n)
	if err := w.WriteLength(oct); err != nil {
		return err
	}

	w.WriteBits(n, oct*8)

	return nil
}

// ReadSemiConstrainedInt decodes a semi-constrained whole number with lower
// bound lb.
func (r *Reader) ReadSemiConstrainedInt(lb int64) (int64, error) {
	oct, err := r.ReadLength()
	if err != nil {
		return 0, err
	}

	if oct == 0 || oct > 8 {
		return 0, &DecodeError{Offset: r.bits, Msg: fmt.Sprintf("invalid integer length %d", oct)}
	}

	v, err := r.ReadBits(oct * 8)
	if err != nil {
		return 0, err
	}

	return lb + int64(v), nil
}

// twosComplementLen returns the minimum number of octets for the 2's-complement
// representation of v (at least 1). Bounds are computed without relying on
// signed overflow: for n in 1..7 the half-range 2^(8n-1) is exact in int64, and
// any int64 fits in 8 octets.
func twosComplementLen(v int64) int {
	for n := 1; n < 8; n++ {
		hi := int64(1) << uint(8*n-1) // 2^(8n-1)
		if v >= -hi && v < hi {
			return n
		}
	}

	return 8
}

// WriteUnconstrainedInt encodes v with no bounds, as a length-prefixed
// 2's-complement integer (X.691).
func (w *Writer) WriteUnconstrainedInt(v int64) error {
	oct := twosComplementLen(v)
	if err := w.WriteLength(oct); err != nil {
		return err
	}

	w.WriteBits(uint64(v)&lowMask(oct), oct*8)

	return nil
}

// ReadUnconstrainedInt decodes an unconstrained 2's-complement integer.
func (r *Reader) ReadUnconstrainedInt() (int64, error) {
	oct, err := r.ReadLength()
	if err != nil {
		return 0, err
	}

	if oct == 0 || oct > 8 {
		return 0, &DecodeError{Offset: r.bits, Msg: fmt.Sprintf("invalid integer length %d", oct)}
	}

	raw, err := r.ReadBits(oct * 8)
	if err != nil {
		return 0, err
	}

	shift := uint(64 - 8*oct)

	return int64(raw<<shift) >> shift, nil
}

// WriteNormallySmall encodes a non-negative whole number that is usually small
// (X.691): values 0..63 take a 7-bit bit-field, larger values fall back
// to a semi-constrained encoding.
func (w *Writer) WriteNormallySmall(v uint64) error {
	if v <= 63 {
		w.WriteBit(0)
		w.WriteBits(v, 6)

		return nil
	}

	w.WriteBit(1)

	return w.WriteSemiConstrainedInt(int64(v), 0)
}

// ReadNormallySmall decodes a normally-small non-negative whole number.
func (r *Reader) ReadNormallySmall() (uint64, error) {
	bit, err := r.ReadBit()
	if err != nil {
		return 0, err
	}

	if bit == 0 {
		return r.ReadBits(6)
	}

	v, err := r.ReadSemiConstrainedInt(0)

	return uint64(v), err
}
