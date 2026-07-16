package per

// EncodeLength encodes a length determinant per §11.9 and then invokes emit
// to write the associated content. emit is called with the number of units
// (octets, bits, characters, or components) it must write; it is called once
// for non-fragmented lengths and once per fragment for fragmented lengths
// (n >= 16K). emit must advance its own position so successive calls write
// successive fragments.
//
// If hasUB is true and ub < 64K, the length is a constrained whole number over
// [lb, ub] (§11.9.3.3); otherwise the three-form unconstrained encoding with
// 16K fragmentation is used (§11.9.3.5–11.9.3.8).
func EncodeLength(
	w *Writer, enc Encoding,
	lb, ub int64, hasUB bool,
	n int64,
	emit func(count int64) error,
) error {
	if n < lb || (hasUB && n > ub) {
		return ErrOverflow
	}

	if hasUB && ub < sixtyFourK {
		if err := EncodeConstrainedWholeNumber(w, enc, lb, ub, n); err != nil {
			return err
		}

		if n > 0 {
			return emit(n)
		}

		return nil
	}

	return encodeUnconstrainedLengthFrag(w, enc, n, emit)
}

// encodeUnconstrainedLengthFrag implements the three-form length determinant
// (§11.9.3.6–11.9.3.8) with 16K fragmentation, invoking emit per fragment.
func encodeUnconstrainedLengthFrag(w *Writer, enc Encoding, n int64, emit func(count int64) error) error {
	if n < 0 {
		return ErrOverflow
	}

	switch {
	case n <= maxShort:
		writeOctetAligned(w, enc, []byte{byte(n)})

		if n > 0 {
			return emit(n)
		}

		return nil
	case n <= maxMedium:
		writeOctetAligned(w, enc, []byte{0x80 | byte((n>>8)&0x3F), byte(n & 0xFF)})
		return emit(n)
	default:
		m := n / fragmentUnit
		m = min(m, 4)
		m = max(m, 1)
		frag := m * fragmentUnit
		writeOctetAligned(w, enc, []byte{0xC0 | byte(m)})

		if err := emit(frag); err != nil {
			return err
		}

		remainder := n - frag
		if remainder <= 0 {
			// Content ended on a 16K boundary: a final zero-length fragment is
			// appended (§11.9.3.8.3 NOTE).
			writeOctetAligned(w, enc, []byte{0x00})
			return nil
		}

		return encodeUnconstrainedLengthFrag(w, enc, remainder, emit)
	}
}

// DecodeLength decodes a length determinant per §11.9, invoking consume for
// each fragment's worth of content. consume is called once for non-fragmented
// lengths (with the full count) and once per fragment for fragmented lengths;
// it must read exactly count units and advance its position accordingly.
func DecodeLength(
	r *Reader, enc Encoding,
	lb, ub int64, hasUB bool,
	consume func(count int64) error,
) error {
	if hasUB && ub < sixtyFourK {
		n, err := DecodeConstrainedWholeNumber(r, enc, lb, ub)
		if err != nil {
			return err
		}

		if n > 0 {
			return consume(n)
		}

		return nil
	}

	return decodeUnconstrainedLengthFrag(r, enc, consume)
}

// decodeUnconstrainedLengthFrag reads the three-form length determinant,
// looping over fragments until a final (non-fragment) form is reached.
func decodeUnconstrainedLengthFrag(r *Reader, enc Encoding, consume func(count int64) error) error {
	first, err := readOctetAligned(r, enc, 1)
	if err != nil {
		return err
	}

	b := first[0]
	switch {
	case b&0x80 == 0: // form a: n <= 127
		n := int64(b & 0x7F)
		if n > 0 {
			return consume(n)
		}

		return nil
	case b&0x40 == 0: // form b: 128 <= n < 16K
		second, err := readOctetAligned(r, enc, 1)
		if err != nil {
			return err
		}

		n := int64(b&0x3F)<<8 | int64(second[0])

		return consume(n)
	default: // fragment: m = b & 0x3F (1..4)
		m := int64(b & 0x3F)

		frag := m * fragmentUnit
		if err := consume(frag); err != nil {
			return err
		}

		return decodeUnconstrainedLengthFrag(r, enc, consume)
	}
}

// EncodeUnconstrainedLength encodes a bare (non-fragmented) unconstrained
// length determinant, used where the content is emitted separately (e.g. the
// octet count of a semi-constrained whole number, §11.7). It supports n < 16K;
// larger values require the fragmented form with interleaved content.
func EncodeUnconstrainedLength(w *Writer, enc Encoding, n int64) error {
	if n < 0 || n > maxMedium {
		return ErrOverflow
	}

	if n <= maxShort {
		writeOctetAligned(w, enc, []byte{byte(n)})
		return nil
	}

	writeOctetAligned(w, enc, []byte{0x80 | byte((n>>8)&0x3F), byte(n & 0xFF)})

	return nil
}

// DecodeUnconstrainedLength decodes a bare non-fragmented unconstrained length
// determinant (the inverse of [EncodeUnconstrainedLength]).
func DecodeUnconstrainedLength(r *Reader, enc Encoding) (int64, error) {
	first, err := readOctetAligned(r, enc, 1)
	if err != nil {
		return 0, err
	}

	b := first[0]
	if b&0x80 == 0 {
		return int64(b & 0x7F), nil
	}

	if b&0x40 == 0 {
		second, err := readOctetAligned(r, enc, 1)
		if err != nil {
			return 0, err
		}

		return int64(b&0x3F)<<8 | int64(second[0]), nil
	}

	return 0, ErrOverflow // fragment header not valid for a bare length
}
