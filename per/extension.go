// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

type ExtensionAdditions struct {
	present []bool
	fields  []rawBits
}

type rawBits struct {
	data  []byte
	nbits int
}

func (e *ExtensionAdditions) Present() bool { return len(e.present) > 0 }

func (e *ExtensionAdditions) MarshalPER(w *Writer, enc Encoding) error {
	if len(e.present) == 0 {
		return nil
	}

	off := 0

	err := EncodeNormallySmallLength(w, enc, int64(len(e.present)), func(count int64) error {
		end := off + int(count)
		for _, p := range e.present[off:end] {
			w.WriteBit(p)
		}

		off = end

		return nil
	})
	if err != nil {
		return err
	}

	for _, f := range e.fields {
		if enc == Aligned {
			w.AlignToByte()
		}

		w.WriteBitString(f.data, f.nbits)
	}

	return nil
}

func (e *ExtensionAdditions) UnmarshalPER(r *Reader, enc Encoding) error {
	var present []bool

	err := DecodeNormallySmallLength(r, enc, func(count int64) error {
		if int64(r.Bits()) < count {
			return ErrTruncated
		}

		for range count {
			b, err := r.ReadBit()
			if err != nil {
				return err
			}

			present = append(present, b)
		}

		return nil
	})
	if err != nil {
		return err
	}

	var fields []rawBits

	for _, p := range present {
		if !p {
			continue
		}

		if enc == Aligned {
			r.AlignToByte()
		}

		start := r.Offset()

		if err := SkipOpenType(r, enc); err != nil {
			return err
		}

		data, nbits, err := r.Since(start)
		if err != nil {
			return err
		}

		fields = append(fields, rawBits{data: data, nbits: nbits})
	}

	e.present, e.fields = present, fields

	return nil
}
