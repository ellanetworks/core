package per

// EncodeOpenType encodes a value as an open type field per §11.2: the value's
// complete PER encoding (padded to an octet boundary) is preceded by an
// unconstrained length determinant in octets. This is used for CHOICE
// extension additions and SEQUENCE extension additions.
func EncodeOpenType(w *Writer, enc Encoding, m Marshaler) error {
	inner := NewWriter()
	if err := m.MarshalPER(inner, enc); err != nil {
		return err
	}

	inner.AlignToByte()
	content := inner.Bytes()
	off := 0

	return EncodeLength(w, enc, 0, 0, false, int64(len(content)), func(count int64) error {
		end := off + int(count)
		writeOctetAligned(w, enc, content[off:end])
		off = end

		return nil
	})
}

// DecodeOpenType decodes an open type field per §11.2, delegating to u.
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

// SkipOpenType reads and discards an open type field (for unknown extensions).
func SkipOpenType(r *Reader, enc Encoding) error {
	return DecodeLength(r, enc, 0, 0, false, func(count int64) error {
		_, err := readOctetAligned(r, enc, int(count))
		return err
	})
}

// EncodeNormallySmallLength encodes a "normally small length" determinant per
// §11.9.3.4 (distinct from §11.6 normally-small number: the length has lb=1,
// so the 6-bit field encodes n-1 for n ≤ 64). emit writes the associated
// field of count units.
func EncodeNormallySmallLength(w *Writer, enc Encoding, n int64, emit func(count int64) error) error {
	if n <= 64 {
		w.WriteBit(false)
		w.WriteBits(uint64(n-1), 6)

		return emit(n)
	}

	w.WriteBit(true)

	return encodeUnconstrainedLengthFrag(w, enc, n, emit)
}

// DecodeNormallySmallLength decodes a "normally small length" determinant per
// §11.9.3.4, invoking consume for the associated field.
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
