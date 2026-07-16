package per

// PER size thresholds, from Rec. ITU-T X.691 (02/2021).
//
//	§11.5.7  constrained whole number: range ≤ 255 bit-field; ==256 one octet;
//	         257..64K two octets; >64K indefinite.
//	§11.9.3  length determinant: ≤127 one octet (bit8=0); <16K two octets
//	         (bit8=1, bit7=0); ≥16K fragment (bit8=1, bit7=1, m=1..4 → m×16K).
const (
	maxShort        = 127   // length form a: n ≤ 127
	maxMedium       = 16383 // length form b: n < 16K (16384)
	fragmentUnit    = 16384 // 16K
	sixtyFourK      = 65536 // 64K
	rangeBitField   = 255   // constrained whole number bit-field case
	rangeOneOctet   = 256   // constrained whole number one-octet case
	rangeTwoOctetLo = 257   // constrained whole number two-octet case lower bound
)

// bitsNeeded returns the minimum number of bits to distinguish `rangeVal`
// values (rangeVal >= 1). Returns 0 for rangeVal == 1 (no bits, §11.5.4).
// This is ceil(log2(rangeVal)) = floor(log2(rangeVal-1)) + 1 for rangeVal >= 2.
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

// minOctetsNonNeg returns the minimum number of octets for a non-negative
// binary integer encoding of v (§11.3.6): a multiple of 8 bits whose leading
// 8 bits are not all zero unless the field is exactly 8 bits. v=0 → 1 octet.
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

// minOctetsTwosComp returns the minimum number of octets for a 2's-complement
// binary integer encoding of n (§11.4.6): a multiple of 8 bits whose leading
// 9 bits are neither all zero nor all one. This is ceil((bitlen(|n|)+1)/8) for
// n != 0, and 1 for n == 0.
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

// EncodeConstrainedWholeNumber encodes n (lb ≤ n ≤ ub) as a constrained whole
// number per §11.5. Supports range 1..65536. For range > 65536 (the indefinite
// case, §11.5.7.4) use [EncodeInteger], which also emits the required length
// determinant; this function returns [ErrOverflow] for that case.
//
// Aligned variant:
//
//	range == 1          → empty bit-field (no bits)
//	range 2..255        → bit-field of ceil(log2(range)) bits (no padding)
//	range == 256        → one octet, octet-aligned (padded first)
//	range 257..65536    → two octets, octet-aligned (padded first)
//
// Unaligned variant: always ceil(log2(range)) bits (no padding), or empty for
// range == 1.
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

// DecodeConstrainedWholeNumber decodes a constrained whole number with range
// 1..65536 per §11.5. It returns the value n in [lb, ub]. For range > 65536
// (indefinite case) use [DecodeInteger].
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

		return lb + int64(v), nil
	}

	switch {
	case rng <= rangeBitField:
		v, err := r.ReadBits(bitsNeeded(rng))
		if err != nil {
			return 0, err
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

		return lb + int64(p[0])<<8 + int64(p[1]), nil
	default:
		return 0, ErrOverflow
	}
}

// EncodeNormallySmall encodes a normally-small non-negative whole number per
// §11.6: n ≤ 63 → 0-bit + 6-bit field; n ≥ 64 → 1-bit + semi-constrained (lb=0)
// length-determined field.
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

// DecodeNormallySmall decodes a normally-small non-negative whole number per §11.6.
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

// encodeSemiConstrained encodes n with lower bound lb and no upper bound per
// §11.7: (n-lb) as a non-negative binary integer in the minimum number of
// octets, octet-aligned in the aligned variant, preceded by an unconstrained
// length determinant (§11.9).
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

	w.AlignToByte()

	return w.WriteOctets(buf)
}

// decodeSemiConstrained decodes a semi-constrained whole number with lower
// bound lb per §11.7.
func decodeSemiConstrained(r *Reader, enc Encoding, lb int64) (int64, error) {
	n, err := DecodeUnconstrainedLength(r, enc)
	if err != nil {
		return 0, err
	}

	if n < 0 {
		return 0, ErrOverflow
	}

	r.AlignToByte()

	p, err := r.ReadOctets(int(n))
	if err != nil {
		return 0, err
	}

	var v uint64
	for _, b := range p {
		v = v<<8 | uint64(b)
	}

	return lb + int64(v), nil
}
