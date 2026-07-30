// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

// EncodeEnumerated encodes an ENUMERATED value per §14. index is the position
// of the value in the enumeration: 0..nRoot-1 for a root value, encoded as a
// constrained whole number; nRoot+k for the k-th extension addition, encoded
// as a normally-small whole number after the extension bit. Extension values
// require extensible.
func EncodeEnumerated(w *Writer, enc Encoding, nRoot int64, extensible bool, index int64) error {
	if index < 0 || (!extensible && index >= nRoot) {
		return ErrOverflow
	}

	if extensible {
		w.WriteBit(index >= nRoot)

		if index >= nRoot {
			return EncodeNormallySmall(w, enc, index-nRoot)
		}
	}

	return EncodeConstrainedWholeNumber(w, enc, 0, nRoot-1, index)
}

// DecodeEnumerated decodes an ENUMERATED value per §14, returning its index:
// 0..nRoot-1 for a root value, nRoot+k for the k-th extension addition.
func DecodeEnumerated(r *Reader, enc Encoding, nRoot int64, extensible bool) (int64, error) {
	if extensible {
		bit, err := r.ReadBit()
		if err != nil {
			return 0, err
		}

		if bit {
			k, err := DecodeNormallySmall(r, enc)
			if err != nil {
				return 0, err
			}

			return nRoot + k, nil
		}
	}

	return DecodeConstrainedWholeNumber(r, enc, 0, nRoot-1)
}
