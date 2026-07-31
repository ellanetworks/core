// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import "math"

// realContentBER produces the CER/DER content octets of a REAL value in the
// base-2 form (Rec. ITU-T X.690 §11.3). Zero is empty content (§11.3.2).
func realContentBER(f float64) []byte {
	if math.IsInf(f, 1) {
		return []byte{0x40}
	}

	if math.IsInf(f, -1) {
		return []byte{0x41}
	}

	if f == 0 {
		return nil
	}

	if math.IsNaN(f) {
		return []byte{0x42}
	}

	sign := byte(0)
	if f < 0 {
		sign = 1
		f = -f
	}

	// Frexp yields mant in [0.5, 1); scaling by 2^53 makes it an integer.
	mant, exp := math.Frexp(f)
	mantInt := uint64(mant * (1 << 53))
	exp -= 53

	for mantInt > 0 && mantInt&1 == 0 {
		mantInt >>= 1
		exp++
	}

	expBytes := encodeSignedBER(int64(exp))
	mantBytes := encodeUnsignedBER(mantInt)

	// First octet: binary flag (bit 8), sign (bit 7), exponent octet count (bits 6-1).
	first := byte(0x80) | (sign << 6) | byte(len(expBytes)&0x3F)
	out := make([]byte, 0, 1+len(expBytes)+len(mantBytes))
	out = append(out, first)
	out = append(out, expBytes...)
	out = append(out, mantBytes...)

	return out
}

func realParseBER(p []byte) (float64, error) {
	if len(p) == 0 {
		return 0, nil
	}

	first := p[0]
	if first == 0x40 {
		return math.Inf(1), nil
	}

	if first == 0x41 {
		return math.Inf(-1), nil
	}

	if first == 0x42 {
		return math.NaN(), nil
	}
	// The base-10 forms (NR1/NR2/NR3, bit 8 clear) are not supported.
	if first&0x80 == 0 {
		return 0, ErrOverflow
	}

	sign := int((first >> 6) & 1)
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

// EncodeREAL encodes a REAL value (§15): CER/DER content octets preceded by an
// unconstrained length determinant.
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

// DecodeREAL decodes a REAL value (§15).
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
