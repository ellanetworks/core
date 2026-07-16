package per

// writeOctetAlignedBitRange writes count bits from data starting at bit offset
// start (MSB-first within each octet). In the ALIGNED variant the writer is
// padded to an octet boundary first.
func writeOctetAlignedBitRange(w *Writer, enc Encoding, data []byte, start, count int) {
	if enc == Aligned {
		w.AlignToByte()
	}

	for i := range count {
		bit := data[(start+i)/8]&(1<<(7-uint((start+i)%8))) != 0
		w.WriteBit(bit)
	}
}

// EncodeBitString encodes a BIT STRING value of nbits bits per §16.
//
//   - extensible and the length is outside the root: extension bit set, then a
//     semi-constrained length (lb=0) and the value (§16.6);
//   - ub == 0: nothing (§16.8);
//   - fixed length ub == lb, ub <= 16: a bit-field of ub bits, no length (§16.9);
//   - fixed length ub == lb, 16 < ub < 64K: an octet-aligned bit-field, no
//     length (§16.10);
//   - otherwise: a length determinant (constrained if ub is set and < 64K,
//     semi-constrained if ub is unset) followed by the value bits (§16.11).
func EncodeBitString(
	w *Writer, enc Encoding,
	lb, ub int64, hasLB, hasUB, extensible bool,
	data []byte, nbits int,
) error {
	if extensible {
		inRoot := !hasUB || nbits <= int(ub)
		if hasLB {
			inRoot = inRoot && nbits >= int(lb)
		}

		w.WriteBit(!inRoot)

		if !inRoot {
			return encodeBitStringValue(w, enc, 0, 0, false, data, nbits)
		}
	}

	if hasUB && ub == 0 {
		return nil
	}

	if hasUB && hasLB && ub == lb {
		switch {
		case ub <= 16:
			w.WriteBitString(data, int(ub))
			return nil
		case ub < sixtyFourK*8:
			writeOctetAlignedBitRange(w, enc, data, 0, int(ub))
			return nil
		}
	}

	return encodeBitStringValue(w, enc, lb, ub, hasUB, data, nbits)
}

func encodeBitStringValue(
	w *Writer, enc Encoding,
	lb, ub int64, hasUB bool,
	data []byte, nbits int,
) error {
	off := 0

	return EncodeLength(w, enc, lb, ub, hasUB, int64(nbits), func(count int64) error {
		writeOctetAlignedBitRange(w, enc, data, off, int(count))
		off += int(count)

		return nil
	})
}

// DecodeBitString decodes a BIT STRING per §16, returning the value as a
// left-aligned byte slice and its bit length.
func DecodeBitString(
	r *Reader, enc Encoding,
	lb, ub int64, hasLB, hasUB, extensible bool,
) ([]byte, int, error) {
	if extensible {
		bit, err := r.ReadBit()
		if err != nil {
			return nil, 0, err
		}

		if bit {
			return decodeBitStringValue(r, enc, 0, 0, false)
		}
	}

	if hasUB && ub == 0 {
		return nil, 0, nil
	}

	if hasUB && hasLB && ub == lb {
		switch {
		case ub <= 16:
			bs, err := r.ReadBitString(int(ub))
			if err != nil {
				return nil, 0, err
			}

			return bs, int(ub), nil
		case ub < sixtyFourK*8:
			return readOctetAlignedBitRange(r, enc, int(ub))
		}
	}

	return decodeBitStringValue(r, enc, lb, ub, hasUB)
}

func decodeBitStringValue(
	r *Reader, enc Encoding,
	lb, ub int64, hasUB bool,
) ([]byte, int, error) {
	var buf []byte

	total := 0

	err := DecodeLength(r, enc, lb, ub, hasUB, func(count int64) error {
		if enc == Aligned {
			r.AlignToByte()
		}

		bs, err := r.ReadBitString(int(count))
		if err != nil {
			return err
		}

		buf = append(buf, bs...)
		total += int(count)

		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	return buf, total, nil
}

// readOctetAlignedBitRange reads nbits bits as an octet-aligned bit-field.
func readOctetAlignedBitRange(r *Reader, enc Encoding, nbits int) ([]byte, int, error) {
	if enc == Aligned {
		r.AlignToByte()
	}

	bs, err := r.ReadBitString(nbits)
	if err != nil {
		return nil, 0, err
	}

	return bs, nbits, nil
}

// EncodeOctetString encodes an OCTET STRING value per §17.
//
//   - extensible and the length is outside the root: extension bit set, then a
//     semi-constrained length (lb=0) and the value (§17.3);
//   - ub == 0: nothing (§17.5);
//   - fixed length ub == lb, ub <= 2: a bit-field of ub*8 bits, no length,
//     no alignment (§17.6);
//   - fixed length ub == lb, 2 < ub < 64K: an octet-aligned bit-field of ub
//     octets, no length (§17.7);
//   - otherwise: a length determinant (constrained if ub is set and < 64K,
//     semi-constrained if ub is unset) followed by the value octets (§17.8).
func EncodeOctetString(
	w *Writer, enc Encoding,
	lb, ub int64, hasLB, hasUB, extensible bool,
	data []byte,
) error {
	n := int64(len(data))
	if extensible {
		inRoot := !hasUB || n <= ub
		if hasLB {
			inRoot = inRoot && n >= lb
		}

		w.WriteBit(!inRoot)

		if !inRoot {
			return encodeOctetStringValue(w, enc, 0, 0, false, data)
		}
	}

	if hasUB && ub == 0 {
		return nil
	}

	if hasUB && hasLB && ub == lb {
		switch {
		case ub <= 2:
			w.WriteBitString(data, int(ub)*8)
			return nil
		case ub < sixtyFourK:
			writeOctetAligned(w, enc, data[:ub])
			return nil
		}
	}

	return encodeOctetStringValue(w, enc, lb, ub, hasUB, data)
}

func encodeOctetStringValue(
	w *Writer, enc Encoding,
	lb, ub int64, hasUB bool,
	data []byte,
) error {
	off := 0

	return EncodeLength(w, enc, lb, ub, hasUB, int64(len(data)), func(count int64) error {
		end := off + int(count)
		writeOctetAligned(w, enc, data[off:end])
		off = end

		return nil
	})
}

// DecodeOctetString decodes an OCTET STRING per §17.
func DecodeOctetString(
	r *Reader, enc Encoding,
	lb, ub int64, hasLB, hasUB, extensible bool,
) ([]byte, error) {
	if extensible {
		bit, err := r.ReadBit()
		if err != nil {
			return nil, err
		}

		if bit {
			return decodeOctetStringValue(r, enc, 0, 0, false)
		}
	}

	if hasUB && ub == 0 {
		return nil, nil
	}

	if hasUB && hasLB && ub == lb {
		switch {
		case ub <= 2:
			bs, err := r.ReadBitString(int(ub) * 8)
			if err != nil {
				return nil, err
			}

			return bs, nil
		case ub < sixtyFourK:
			return readOctetAligned(r, enc, int(ub))
		}
	}

	return decodeOctetStringValue(r, enc, lb, ub, hasUB)
}

func decodeOctetStringValue(
	r *Reader, enc Encoding,
	lb, ub int64, hasUB bool,
) ([]byte, error) {
	var buf []byte

	err := DecodeLength(r, enc, lb, ub, hasUB, func(count int64) error {
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
