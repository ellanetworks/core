// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

// WriteOpenType wraps an already-encoded inner value as an open type: an
// unconstrained length determinant followed by the octet-aligned content
// (X.691 §10.2). The inner encoding is at least one octet (§10.1.3).
func (w *Writer) WriteOpenType(inner []byte) error {
	if len(inner) == 0 {
		inner = []byte{0x00}
	}

	if err := w.WriteLength(len(inner)); err != nil {
		return err
	}

	w.WriteOctets(inner)

	return nil
}

// ReadOpenType reads an open type and returns its raw inner octets, to be
// decoded with a fresh [Reader]. The fragmented form is not supported.
func (r *Reader) ReadOpenType() ([]byte, error) {
	n, err := r.ReadLength()
	if err != nil {
		return nil, err
	}

	return r.ReadOctets(n)
}
