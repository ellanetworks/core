// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import "fmt"

type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

type Enum[T Integer] struct {
	Root       []string
	Extensible bool
	Extension  []string
}

func (e *Enum[T]) extensible() bool { return e.Extensible || len(e.Extension) > 0 }

func (e *Enum[T]) Encode(w *Writer, enc Encoding, v T) error {
	return EncodeEnumerated(w, enc, int64(len(e.Root)), e.extensible(), int64(v))
}

func (e *Enum[T]) Decode(r *Reader, enc Encoding) (T, error) {
	idx, err := DecodeEnumerated(r, enc, int64(len(e.Root)), e.extensible())
	if err != nil {
		return 0, err
	}

	if v := T(idx); int64(v) == idx {
		return v, nil
	}

	return 0, ErrOverflow
}

func (e *Enum[T]) Known(v T) bool {
	return int64(v) >= 0 && int64(v) < int64(len(e.Root)+len(e.Extension))
}

func (e *Enum[T]) Name(v T) string {
	switch i := int64(v); {
	case i >= 0 && i < int64(len(e.Root)):
		return e.Root[i]
	case i >= int64(len(e.Root)) && i < int64(len(e.Root)+len(e.Extension)):
		return e.Extension[i-int64(len(e.Root))]
	default:
		return fmt.Sprintf("unknown(%d)", i)
	}
}

type ExtensionAlternative struct {
	Index int64
	Value string
}

func (a *ExtensionAlternative) MarshalPER(w *Writer, enc Encoding) error {
	if err := EncodeNormallySmall(w, enc, a.Index); err != nil {
		return err
	}

	return EncodeOpenTypeBytes(w, enc, []byte(a.Value))
}

func DecodeExtensionAlternative(r *Reader, enc Encoding, index int64) (*ExtensionAlternative, error) {
	b, err := DecodeOpenTypeBytes(r, enc)
	if err != nil {
		return nil, err
	}

	return &ExtensionAlternative{Index: index, Value: string(b)}, nil
}
