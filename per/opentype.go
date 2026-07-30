// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

// EncodeOpenType encodes m as an open type field (§11.2): its complete PER
// encoding, padded to an octet boundary, preceded by a length in octets.
func EncodeOpenType(w *Writer, enc Encoding, m Marshaler) error {
	inner := NewWriter()
	if err := m.MarshalPER(inner, enc); err != nil {
		return err
	}

	inner.AlignToByte()

	return EncodeOpenTypeBytes(w, enc, inner.Bytes())
}

// DecodeOpenType decodes an open type field (§11.2), delegating to u.
func DecodeOpenType(r *Reader, enc Encoding, u Unmarshaler) error {
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
		return err
	}

	inner := NewReader(buf)

	return u.UnmarshalPER(inner, enc)
}

// SkipOpenType reads and discards an open type field (§11.2).
func SkipOpenType(r *Reader, enc Encoding) error {
	return DecodeLength(r, enc, 0, 0, false, func(count int64) error {
		_, err := readOctetAligned(r, enc, int(count))
		return err
	})
}

// EncodeOpenTypeBytes encodes an already-encoded complete PER value as an open
// type field (§11.2). An empty encoding becomes the single zero octet required
// by §11.2.1.
func EncodeOpenTypeBytes(w *Writer, enc Encoding, content []byte) error {
	if len(content) == 0 {
		content = []byte{0x00}
	}

	off := 0

	return EncodeLength(w, enc, 0, 0, false, int64(len(content)), func(count int64) error {
		end := off + int(count)
		writeOctetAligned(w, enc, content[off:end])
		off = end

		return nil
	})
}

// DecodeOpenTypeBytes reads an open type field (§11.2) and returns its content
// octets undecoded.
func DecodeOpenTypeBytes(r *Reader, enc Encoding) ([]byte, error) {
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
		return nil, err
	}

	return buf, nil
}

// EncodeNormallySmallLength encodes a normally-small length determinant
// (§11.9.3.4). Unlike the §11.6 normally-small number, this has lb = 1: the
// 6-bit field holds n-1, so n == 0 has no representation.
func EncodeNormallySmallLength(w *Writer, enc Encoding, n int64, emit func(count int64) error) error {
	if n < 1 {
		return ErrOverflow
	}

	if n <= 64 {
		w.WriteBit(false)
		w.WriteBits(uint64(n-1), 6)

		return emit(n)
	}

	w.WriteBit(true)

	return encodeUnconstrainedLengthFrag(w, enc, n, emit)
}

// DecodeNormallySmallLength decodes a normally-small length determinant
// (§11.9.3.4).
func DecodeNormallySmallLength(r *Reader, enc Encoding, consume func(count int64) error) error {
	bit, err := r.ReadBit()
	if err != nil {
		return err
	}

	if !bit {
		v, err := r.ReadBits(6)
		if err != nil {
			return err
		}

		n := int64(v) + 1

		return consume(n)
	}

	return decodeUnconstrainedLengthFrag(r, enc, consume)
}
