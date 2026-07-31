// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

// Null represents the ASN.1 NULL type and empty SEQUENCE {}; both occupy zero
// bits (§18, §20.6).
type Null struct{}

func (n *Null) MarshalPER(w *Writer, enc Encoding) error {
	return EncodeNull(w, enc)
}

func (n *Null) UnmarshalPER(r *Reader, enc Encoding) error {
	return DecodeNull(r, enc)
}

// BoolsToBits packs bools MSB-first within each octet into ceil(len(bools)/8)
// bytes.
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

// BitsToBools unpacks the first nbits MSB-first bits of data.
func BitsToBools(data []byte, nbits int) []bool {
	out := make([]bool, nbits)
	for i := range nbits {
		out[i] = data[i/8]&(1<<(7-uint(i%8))) != 0
	}

	return out
}
