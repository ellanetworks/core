// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

type Marshaler interface {
	MarshalPER(w *Writer, enc Encoding) error
}

type Unmarshaler interface {
	UnmarshalPER(r *Reader, enc Encoding) error
}

// MarshalerFunc adapts an ordinary function to [Marshaler].
type MarshalerFunc func(w *Writer, enc Encoding) error

func (f MarshalerFunc) MarshalPER(w *Writer, enc Encoding) error { return f(w, enc) }

// UnmarshalerFunc adapts an ordinary function to [Unmarshaler].
type UnmarshalerFunc func(r *Reader, enc Encoding) error

func (f UnmarshalerFunc) UnmarshalPER(r *Reader, enc Encoding) error { return f(r, enc) }

// Marshal encodes m; enc defaults to [Aligned]. The result is a copy.
func Marshal(m Marshaler, enc ...Encoding) ([]byte, error) {
	e := Aligned
	if len(enc) > 0 {
		e = enc[0]
	}

	w := NewWriter()
	if err := m.MarshalPER(w, e); err != nil {
		return nil, err
	}
	// A complete PER encoding is padded to an octet boundary.
	w.AlignToByte()

	return w.Bytes(), nil
}

// Unmarshal decodes b into u; enc defaults to [Aligned]. Bits left unread are
// not an error, since a top-level encoding is padded to an octet boundary.
func Unmarshal(b []byte, u Unmarshaler, enc ...Encoding) error {
	e := Aligned
	if len(enc) > 0 {
		e = enc[0]
	}

	r := NewReader(b)

	return u.UnmarshalPER(r, e)
}
