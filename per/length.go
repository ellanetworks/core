// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

// EncodeLength encodes a length determinant (§11.9), then calls emit with the
// number of units to write — once, or once per 16K fragment when n ≥ 16K, in
// which case emit must advance its own position between calls. The determinant
// is a constrained whole number when hasUB and ub < 64K (§11.9.3.3), otherwise
// the three-form encoding (§11.9.3.5–11.9.3.8).
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
// with 16K fragmentation (§11.9.3.6–11.9.3.8).
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
			// §11.9.3.8.3 NOTE: content ending on a 16K boundary needs a final
			// zero-length fragment.
			writeOctetAligned(w, enc, []byte{0x00})
			return nil
		}

		return encodeUnconstrainedLengthFrag(w, enc, remainder, emit)
	}
}

// DecodeLength decodes a length determinant (§11.9), then calls consume once
// per fragment; consume must read exactly count units.
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

	// §11.9.3.3 leaves the length determinant unconstrained once ub reaches 64K,
	// so the wire format carries no bound. The size constraint still holds on the
	// abstract value, and the count is the peer's to choose, so it is enforced
	// across the fragments here: without this a bounded SEQUENCE OF grows until
	// the input runs out.
	var total int64

	guard := func(count int64) error {
		total += count

		if hasUB && total > ub {
			return ErrOverflow
		}

		return consume(count)
	}

	if err := decodeUnconstrainedLengthFrag(r, enc, guard); err != nil {
		return err
	}

	if total < lb {
		return ErrOverflow
	}

	return nil
}

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
		// §11.9.3.8: only m in 1..4 is defined; m == 0 would recurse without
		// consuming any content.
		if m < 1 || m > 4 {
			return ErrOverflow
		}

		frag := m * fragmentUnit
		if err := consume(frag); err != nil {
			return err
		}

		return decodeUnconstrainedLengthFrag(r, enc, consume)
	}
}

// EncodeUnconstrainedLength encodes a bare unconstrained length determinant,
// for content emitted separately (e.g. §11.7). n must be < 16K; larger values
// require the fragmented form with interleaved content.
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

// DecodeUnconstrainedLength decodes a bare unconstrained length determinant.
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
