// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import "encoding/binary"

type ExtensionAdditions struct {
	present string
	fields  string
}

func (e *ExtensionAdditions) Present() bool { return len(e.present) > 0 }

func (e *ExtensionAdditions) MarshalPER(w *Writer, enc Encoding) error {
	if len(e.present) == 0 {
		return nil
	}

	off := 0

	err := EncodeNormallySmallLength(w, enc, int64(len(e.present)), func(count int64) error {
		end := off + int(count)
		for i := off; i < end; i++ {
			w.WriteBit(e.present[i] != 0)
		}

		off = end

		return nil
	})
	if err != nil {
		return err
	}

	for rest := e.fields; rest != ""; {
		var (
			data  string
			nbits int
		)

		data, nbits, rest = nextRawBits(rest)

		if enc == Aligned {
			w.AlignToByte()
		}

		w.WriteBitString([]byte(data), nbits)
	}

	return nil
}

func (e *ExtensionAdditions) UnmarshalPER(r *Reader, enc Encoding) error {
	var present []byte

	err := DecodeNormallySmallLength(r, enc, func(count int64) error {
		if int64(r.Bits()) < count {
			return ErrTruncated
		}

		for range count {
			b, err := r.ReadBit()
			if err != nil {
				return err
			}

			if b {
				present = append(present, 1)
			} else {
				present = append(present, 0)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	var fields []byte

	for _, p := range present {
		if p == 0 {
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

		fields = binary.AppendUvarint(fields, uint64(nbits))
		fields = append(fields, data...)
	}

	e.present, e.fields = string(present), string(fields)

	return nil
}

func nextRawBits(s string) (string, int, string) {
	n, w := binary.Uvarint([]byte(s[:min(len(s), binary.MaxVarintLen64)]))
	end := w + (int(n)+7)/8

	return s[w:end], int(n), s[end:]
}
