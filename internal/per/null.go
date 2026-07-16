// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

// Null represents the ASN.1 NULL type and empty SEQUENCE {}. It encodes and
// decodes as zero bits (Rec. ITU-T X.691 §18 and §20.6).
type Null struct{}

// MarshalPER encodes a Null value: no bits are written.
func (n *Null) MarshalPER(w *Writer, enc Encoding) error {
	return EncodeNull(w, enc)
}

// UnmarshalPER decodes a Null value: no bits are consumed.
func (n *Null) UnmarshalPER(r *Reader, enc Encoding) error {
	return DecodeNull(r, enc)
}

// BoolsToBits packs a []bool into a []byte where each bit is stored MSB-first
// within each octet, matching the PER BIT STRING wire representation. The
// result length is ceil(len(bools)/8).
func BoolsToBits(bools []bool) []byte {
	n := len(bools)
	if n == 0 {
		return nil
	}

	out := make([]byte, (n+7)/8)

	for i, b := range bools {
		if b {
			out[i/8] |= 1 << (7 - uint(i%8))
		}
	}

	return out
}

// BitsToBools unpacks a packed []byte (MSB-first bits) into a []bool of the
// given length. It is the inverse of [BoolsToBits].
func BitsToBools(data []byte, nbits int) []bool {
	out := make([]bool, nbits)
	for i := range nbits {
		out[i] = data[i/8]&(1<<(7-uint(i%8))) != 0
	}

	return out
}
