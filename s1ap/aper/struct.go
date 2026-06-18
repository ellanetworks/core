// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

import "fmt"

// putExtensibleIndex writes an extension marker (when ext) followed by an index
// that is either a root bit-field value or, for an extension addition, a
// normally-small number. Shared by ENUMERATED (X.691 §13) and CHOICE (§22),
// whose index encodings are identical.
func (w *Writer) putExtensibleIndex(index, nRoot int, ext, isExt bool) error {
	if ext {
		w.WriteBool(isExt)
	}

	if isExt {
		return w.WriteNormallySmall(uint64(index))
	}

	if index < 0 || index >= nRoot {
		return fmt.Errorf("aper: index %d outside root count %d", index, nRoot)
	}

	w.WriteBits(uint64(index), bitsForRange(uint64(nRoot)))

	return nil
}

// getExtensibleIndex decodes an extension marker (when ext) and the following
// root or extension index.
func (r *Reader) getExtensibleIndex(nRoot int, ext bool) (index int, isExt bool, err error) {
	if ext {
		b, err := r.ReadBool()
		if err != nil {
			return 0, false, err
		}

		if b {
			v, err := r.ReadNormallySmall()
			return int(v), true, err
		}
	}

	v, err := r.ReadBits(bitsForRange(uint64(nRoot)))
	if err != nil {
		return 0, false, err
	}

	if int(v) >= nRoot {
		return 0, false, &DecodeError{Offset: r.bits, Msg: "index out of range"}
	}

	return int(v), false, nil
}

// WriteEnum encodes an ENUMERATED value by its index among the nRoot root
// values (X.691 §13). ext reports whether the type is extensible; set isExt and
// pass the index among extension values when the value is an extension addition.
func (w *Writer) WriteEnum(index, nRoot int, ext, isExt bool) error {
	return w.putExtensibleIndex(index, nRoot, ext, isExt)
}

// ReadEnum decodes an ENUMERATED value, returning its index and whether it is
// an extension addition.
func (r *Reader) ReadEnum(nRoot int, ext bool) (index int, isExt bool, err error) {
	return r.getExtensibleIndex(nRoot, ext)
}

// WriteChoiceIndex encodes the chosen alternative of a CHOICE among nRoot root
// alternatives (X.691 §22). For an extension alternative, set isExt and pass
// the index among extension alternatives; the caller then writes the value as
// an open type.
func (w *Writer) WriteChoiceIndex(index, nRoot int, ext, isExt bool) error {
	return w.putExtensibleIndex(index, nRoot, ext, isExt)
}

// ReadChoiceIndex decodes a CHOICE alternative index, returning it and whether
// it is an extension alternative.
func (r *Reader) ReadChoiceIndex(nRoot int, ext bool) (index int, isExt bool, err error) {
	return r.getExtensibleIndex(nRoot, ext)
}

// WriteNSLength encodes a normally-small length n >= 1 (X.691 §10.9.3.4), used
// for the SEQUENCE extension-addition bitmap.
func (w *Writer) WriteNSLength(n int) error {
	if n < 1 {
		return fmt.Errorf("aper: normally-small length %d must be >= 1", n)
	}

	if n <= 64 {
		w.WriteBits(uint64(n-1), 7)
		return nil
	}

	w.WriteBit(1)

	return w.WriteLength(n)
}

// ReadNSLength decodes a normally-small length.
func (r *Reader) ReadNSLength() (int, error) {
	b, err := r.ReadBit()
	if err != nil {
		return 0, err
	}

	if b == 0 {
		v, err := r.ReadBits(6)
		if err != nil {
			return 0, err
		}

		return int(v) + 1, nil
	}

	return r.ReadLength()
}

// WriteSequencePreamble writes a SEQUENCE preamble (X.691 §18): the extension
// bit (when the type is extensible) followed by one presence bit per OPTIONAL
// or DEFAULT root field, in declaration order. Pass extPresent = true only when
// extension additions are encoded after the root.
func (w *Writer) WriteSequencePreamble(extensible, extPresent bool, optionals []bool) {
	if extensible {
		w.WriteBool(extPresent)
	}

	for _, present := range optionals {
		w.WriteBool(present)
	}
}

// ReadSequencePreamble reads a SEQUENCE preamble. nOptional is the number of
// OPTIONAL/DEFAULT root fields. It returns whether extension additions follow
// (always false for a non-extensible type) and the presence bit per optional
// field, in declaration order.
func (r *Reader) ReadSequencePreamble(extensible bool, nOptional int) (extPresent bool, optionals []bool, err error) {
	if extensible {
		extPresent, err = r.ReadBool()
		if err != nil {
			return false, nil, err
		}
	}

	optionals = make([]bool, nOptional)
	for i := range optionals {
		optionals[i], err = r.ReadBool()
		if err != nil {
			return false, nil, err
		}
	}

	return extPresent, optionals, nil
}

// SkipExtensionAdditions consumes the extension-addition block of an extensible
// SEQUENCE whose preamble reported additions present (X.691 §18.7-18.9): the
// normally-small-length bitmap followed by that many open-type fields, which
// are read and discarded. This lets the decoder accept messages carrying
// additions it does not model.
func (r *Reader) SkipExtensionAdditions() error {
	n, err := r.ReadNSLength()
	if err != nil {
		return err
	}

	present := make([]bool, n)
	for i := range present {
		present[i], err = r.ReadBool()
		if err != nil {
			return err
		}
	}

	for i := range present {
		if present[i] {
			if _, err := r.ReadOpenType(); err != nil {
				return err
			}
		}
	}

	return nil
}
