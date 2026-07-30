// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import "math"

// Size thresholds from Rec. ITU-T X.691 (02/2021).
//
//	§11.5.7  constrained whole number: range ≤ 255 bit-field; ==256 one octet;
//	         257..64K two octets; >64K indefinite.
//	§11.9.3  length determinant: ≤127 one octet (bit8=0); <16K two octets
//	         (bit8=1, bit7=0); ≥16K fragment (bit8=1, bit7=1, m=1..4 → m×16K).
const (
	maxShort        = 127
	maxMedium       = 16383
	fragmentUnit    = 16384
	sixtyFourK      = 65536
	rangeBitField   = 255
	rangeOneOctet   = 256
	rangeTwoOctetLo = 257
)

// bitsNeeded returns ceil(log2(rangeVal)), and 0 for rangeVal == 1 (§11.5.4).
func bitsNeeded(rangeVal int64) int {
	if rangeVal <= 1 {
		return 0
	}

	b := 0
	for r := rangeVal - 1; r > 0; r >>= 1 {
		b++
	}

	return b
}

// minOctetsNonNeg returns the octet count of a minimal non-negative binary
// integer encoding of v (§11.3.6); v == 0 takes one octet.
func minOctetsNonNeg(v uint64) int {
	if v == 0 {
		return 1
	}

	b := 0
	for tmp := v; tmp > 0; tmp >>= 1 {
		b++
	}

	return (b + 7) / 8
}

// minOctetsTwosComp returns the octet count of a minimal 2's-complement binary
// integer encoding of n (§11.4.6).
func minOctetsTwosComp(n int64) int {
	var mag uint64
	if n >= 0 {
		mag = uint64(n)
	} else {
		mag = uint64(-n - 1)
	}

	w := 1
	for tmp := mag; tmp > 0; tmp >>= 1 {
		w++
	}

	return (w + 7) / 8
}

// EncodeConstrainedWholeNumber encodes n as a constrained whole number (§11.5).
// The range ub-lb+1 must be ≤ 65536; the indefinite case (§11.5.7.4) is
// [ErrOverflow] here and handled by [EncodeInteger].
func EncodeConstrainedWholeNumber(w *Writer, enc Encoding, lb, ub, n int64) error {
	if n < lb || n > ub {
		return ErrOverflow
	}

	rng := ub - lb + 1
	if rng == 1 {
		return nil
	}

	v := uint64(n - lb)
	if enc == Unaligned {
		w.WriteBits(v, bitsNeeded(rng))
		return nil
	}

	switch {
	case rng <= rangeBitField:
		w.WriteBits(v, bitsNeeded(rng))
	case rng == rangeOneOctet:
		w.AlignToByte()
		_ = w.WriteOctets([]byte{byte(v)})
	case rng <= sixtyFourK:
		w.AlignToByte()
		_ = w.WriteOctets([]byte{byte(v >> 8), byte(v)})
	default:
		return ErrOverflow
	}

	return nil
}

// DecodeConstrainedWholeNumber decodes a constrained whole number of range
// ≤ 65536 (§11.5). A bit pattern above the range is rejected: §11.5 assigns it
// no value.
func DecodeConstrainedWholeNumber(r *Reader, enc Encoding, lb, ub int64) (int64, error) {
	rng := ub - lb + 1
	if rng == 1 {
		return lb, nil
	}

	if enc == Unaligned {
		v, err := r.ReadBits(bitsNeeded(rng))
		if err != nil {
			return 0, err
		}

		if int64(v) >= rng {
			return 0, ErrOverflow
		}

		return lb + int64(v), nil
	}

	switch {
	case rng <= rangeBitField:
		v, err := r.ReadBits(bitsNeeded(rng))
		if err != nil {
			return 0, err
		}

		if int64(v) >= rng {
			return 0, ErrOverflow
		}

		return lb + int64(v), nil
	case rng == rangeOneOctet:
		r.AlignToByte()

		p, err := r.ReadOctets(1)
		if err != nil {
			return 0, err
		}

		return lb + int64(p[0]), nil
	case rng <= sixtyFourK:
		r.AlignToByte()

		p, err := r.ReadOctets(2)
		if err != nil {
			return 0, err
		}

		v := int64(p[0])<<8 + int64(p[1])
		if v >= rng {
			return 0, ErrOverflow
		}

		return lb + v, nil
	default:
		return 0, ErrOverflow
	}
}

// EncodeNormallySmall encodes a normally-small non-negative whole number
// (§11.6): n ≤ 63 as 0-bit plus a 6-bit field, otherwise 1-bit plus a
// semi-constrained (lb=0) field.
func EncodeNormallySmall(w *Writer, enc Encoding, n int64) error {
	if n < 0 {
		return ErrOverflow
	}

	if n <= 63 {
		w.WriteBit(false)
		w.WriteBits(uint64(n), 6)

		return nil
	}

	w.WriteBit(true)

	return encodeSemiConstrained(w, enc, 0, n)
}

// DecodeNormallySmall decodes a normally-small non-negative whole number (§11.6).
func DecodeNormallySmall(r *Reader, enc Encoding) (int64, error) {
	bit, err := r.ReadBit()
	if err != nil {
		return 0, err
	}

	if !bit {
		v, err := r.ReadBits(6)
		if err != nil {
			return 0, err
		}

		return int64(v), nil
	}

	return decodeSemiConstrained(r, enc, 0)
}

// encodeSemiConstrained encodes n with lower bound lb and no upper bound
// (§11.7): n-lb as a minimal non-negative binary integer, preceded by an
// unconstrained length determinant (§11.9).
func encodeSemiConstrained(w *Writer, enc Encoding, lb, n int64) error {
	v := uint64(n - lb)
	if n < lb {
		return ErrOverflow
	}

	octets := minOctetsNonNeg(v)

	buf := make([]byte, octets)
	for i := range octets {
		buf[octets-1-i] = byte(v >> (8 * i))
	}

	if err := EncodeUnconstrainedLength(w, enc, int64(octets)); err != nil {
		return err
	}

	writeOctetAligned(w, enc, buf)

	return nil
}

// decodeSemiConstrained decodes a semi-constrained whole number with lower
// bound lb (§11.7).
func decodeSemiConstrained(r *Reader, enc Encoding, lb int64) (int64, error) {
	n, err := DecodeUnconstrainedLength(r, enc)
	if err != nil {
		return 0, err
	}

	// §11.7: a zero-octet field has no value, and more than 8 octets exceed int64.
	if n < 1 || n > 8 {
		return 0, ErrOverflow
	}

	p, err := readOctetAligned(r, enc, int(n))
	if err != nil {
		return 0, err
	}

	var v uint64
	for _, b := range p {
		v = v<<8 | uint64(b)
	}
	// §11.7 values are non-negative: reject rather than wrap to a negative int64.
	if v > math.MaxInt64 || int64(v) > math.MaxInt64-lb {
		return 0, ErrOverflow
	}

	return lb + int64(v), nil
}
