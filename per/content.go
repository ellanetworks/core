// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import "fmt"

// writeOctetAlignedBitRange writes count bits from data starting at bit offset
// start, MSB-first within each octet.
func writeOctetAlignedBitRange(w *Writer, enc Encoding, data []byte, start, count int) {
	if enc == Aligned {
		w.AlignToByte()
	}

	for i := range count {
		bit := data[(start+i)/8]&(1<<(7-uint((start+i)%8))) != 0
		w.WriteBit(bit)
	}
}

// EncodeBitString encodes a BIT STRING of nbits bits (§16): out-of-root
// extension values (§16.6), empty (§16.8), fixed size ≤ 16 bits as a bare
// bit-field (§16.9), larger fixed size octet-aligned (§16.10), otherwise a
// length determinant followed by the value bits (§16.11).
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
		if int64(nbits) != ub {
			return fmt.Errorf("%w: bit string has %d bits, fixed size is %d", ErrOverflow, nbits, ub)
		}

		if need := (nbits + 7) / 8; len(data) < need {
			return fmt.Errorf("%w: bit string buffer has %d octets, need %d for %d bits", ErrOverflow, len(data), need, nbits)
		}

		switch {
		case ub <= 16:
			w.WriteBitString(data, int(ub))
			return nil
		case ub < sixtyFourK*8:
			writeOctetAlignedBitRange(w, enc, data, 0, int(ub))
			return nil
		}
	}

	if need := (nbits + 7) / 8; len(data) < need {
		return fmt.Errorf("%w: bit string buffer has %d octets, need %d for %d bits", ErrOverflow, len(data), need, nbits)
	}

	if hasUB && int64(nbits) > ub {
		return fmt.Errorf("%w: bit string has %d bits, maximum is %d", ErrOverflow, nbits, ub)
	}

	if hasLB && int64(nbits) < lb {
		return fmt.Errorf("%w: bit string has %d bits, minimum is %d", ErrOverflow, nbits, lb)
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

// DecodeBitString decodes a BIT STRING (§16), returning left-aligned octets
// and the bit length.
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

// EncodeOctetString encodes an OCTET STRING (§17): out-of-root extension
// values (§17.3), empty (§17.5), fixed size ≤ 2 octets as an unaligned
// bit-field (§17.6), larger fixed size octet-aligned (§17.7), otherwise a
// length determinant followed by the value octets (§17.8).
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
		if int64(len(data)) != ub {
			return fmt.Errorf("%w: octet string has %d octets, fixed size is %d", ErrOverflow, len(data), ub)
		}

		switch {
		case ub <= 2:
			w.WriteBitString(data, int(ub)*8)
			return nil
		case ub < sixtyFourK:
			writeOctetAligned(w, enc, data)
			return nil
		}
	}

	if hasUB && int64(len(data)) > ub {
		return fmt.Errorf("%w: octet string has %d octets, maximum is %d", ErrOverflow, len(data), ub)
	}

	if hasLB && int64(len(data)) < lb {
		return fmt.Errorf("%w: octet string has %d octets, minimum is %d", ErrOverflow, len(data), lb)
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

// DecodeOctetString decodes an OCTET STRING (§17).
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
