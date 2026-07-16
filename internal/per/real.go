package per

import "math"

// realContentBER produces the CER/DER content octets for a REAL value per
// Rec. ITU-T X.690 §11.3. For base-2 reals (the common case), the format is:
//
//	first octet: 1_00000_S (S = sign bit, 0=positive, 1=negative)
//	following:  exponent (two's complement, min octets) + mantissa (unsigned)
//
// PLUS-INFINITY and MINUS-INFINITY encode as 0x40 and 0x41 respectively.
// Zero encodes as an empty octet string (§11.3.2).
//
// The value (mantissa, exponent) satisfies: value = S * mantissa * 2^exponent
// where mantissa = N / 2^F, and N, F are the unsigned mantissa and scale.
func realContentBER(f float64) []byte {
	if math.IsInf(f, 1) {
		return []byte{0x40}
	}

	if math.IsInf(f, -1) {
		return []byte{0x41}
	}

	if f == 0 {
		return nil // empty content octets
	}

	if math.IsNaN(f) {
		return []byte{0x42}
	}

	sign := byte(0)
	if f < 0 {
		sign = 1
		f = -f
	}

	// Decompose into mantissa * 2^exp using math.Frexp (gives mantissa in [0.5,1)).
	// We want an integer mantissa, so we shift.
	mant, exp := math.Frexp(f)
	// mant in [0.5, 1), so mant * 2^exp = f. Multiply mant by 2^53 to get integer.
	mantInt := uint64(mant * (1 << 53))
	exp -= 53
	// Remove trailing zero bits from mantInt (normalize).
	for mantInt > 0 && mantInt&1 == 0 {
		mantInt >>= 1
		exp++
	}

	// Encode exponent as two's complement minimum octets.
	expBytes := encodeSignedBER(int64(exp))
	// Encode mantissa as unsigned minimum octets.
	mantBytes := encodeUnsignedBER(mantInt)

	// First octet: 1_00_00_S | length-of-exponent (bits 1-6 = exp length).
	first := byte(0x80) | (sign << 6) | byte(len(expBytes)&0x3F)
	out := make([]byte, 0, 1+len(expBytes)+len(mantBytes))
	out = append(out, first)
	out = append(out, expBytes...)
	out = append(out, mantBytes...)

	return out
}

// realParseBER decodes CER/DER content octets back to a float64.
func realParseBER(p []byte) (float64, error) {
	if len(p) == 0 {
		return 0, nil
	}

	first := p[0]
	// Special values.
	if first == 0x40 {
		return math.Inf(1), nil
	}

	if first == 0x41 {
		return math.Inf(-1), nil
	}

	if first == 0x42 {
		return math.NaN(), nil
	}
	// Base-2 binary encoding: bit 8 = 1.
	if first&0x80 == 0 {
		// Base-10 (NR1/NR2/NR3) — not supported in binary REAL. Parse as decimal.
		return 0, ErrOverflow
	}

	sign := int((first >> 6) & 1)
	// Scale (F) is bits 5-4 for base-2 with explicit scale; bits 3-2 select
	// the exponent base. We only support the canonical base-2 form (00).
	scale := int((first >> 4) & 0x03)

	expLen := int(first & 0x0F)
	if expLen == 0 || 1+expLen > len(p) {
		return 0, ErrTruncated
	}

	exp, err := decodeSignedBER(p[1 : 1+expLen])
	if err != nil {
		return 0, err
	}

	mantBytes := p[1+expLen:]

	mant, err := decodeUnsignedBER(mantBytes)
	if err != nil {
		return 0, err
	}
	// value = (-1)^sign * mant * 2^(exp - scale)
	val := float64(mant) * math.Ldexp(1, int(exp)-scale)
	if sign != 0 {
		val = -val
	}

	return val, nil
}

// encodeSignedBER encodes a signed integer as minimum-octet two's complement.
func encodeSignedBER(n int64) []byte {
	if n == 0 {
		return []byte{0}
	}

	octets := minOctetsTwosComp(n)

	buf := make([]byte, octets)
	for i := range octets {
		buf[octets-1-i] = byte(n >> (8 * i))
	}

	return buf
}

// encodeUnsignedBER encodes an unsigned integer as minimum octets.
func encodeUnsignedBER(v uint64) []byte {
	if v == 0 {
		return []byte{0}
	}

	octets := minOctetsNonNeg(v)

	buf := make([]byte, octets)
	for i := range octets {
		buf[octets-1-i] = byte(v >> (8 * i))
	}

	return buf
}

// decodeSignedBER decodes a two's-complement integer.
func decodeSignedBER(p []byte) (int64, error) {
	if len(p) == 0 {
		return 0, ErrTruncated
	}

	var v int64
	for _, b := range p {
		v = v<<8 | int64(b)
	}

	bits := len(p) * 8
	if bits < 64 && v&(1<<(bits-1)) != 0 {
		v |= ^((1 << bits) - 1)
	}

	return v, nil
}

// decodeUnsignedBER decodes an unsigned integer.
func decodeUnsignedBER(p []byte) (uint64, error) {
	if len(p) == 0 {
		return 0, ErrTruncated
	}

	var v uint64
	for _, b := range p {
		v = v<<8 | uint64(b)
	}

	return v, nil
}

// EncodeREAL encodes a REAL value per §15: CER/DER content octets preceded by
// an unconstrained length determinant (semi-constrained whole number, lb=0).
func EncodeREAL(w *Writer, enc Encoding, f float64) error {
	content := realContentBER(f)
	off := 0

	return EncodeLength(w, enc, 0, 0, false, int64(len(content)), func(count int64) error {
		end := off + int(count)
		writeOctetAligned(w, enc, content[off:end])
		off = end

		return nil
	})
}

// DecodeREAL decodes a REAL value per §15.
func DecodeREAL(r *Reader, enc Encoding) (float64, error) {
	var buf []byte

	err := DecodeLength(r, enc, 0, 0, false, func(count int64) error {
		p, err := readOctetAligned(r, enc, int(count))
		if err != nil {
			return err
		}

		buf = append(buf, p...)

		return nil
	})
	if err != nil {
		return 0, err
	}

	return realParseBER(buf)
}
