// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

func encodeExtensionContainer(w *per.Writer, enc per.Encoding, fields []ieField) error {
	return ap.EncodeExtensionContainer(profile, w, enc, fields)
}

func decodeExtensionContainer[M any](r *per.Reader, enc per.Encoding, table []ieSpec[M], m *M, meta *extensionMeta) error {
	return ap.DecodeExtensionContainer(profile, r, enc, table, m, meta)
}

func optionalExtension[M any, T any, PT interface {
	*T
	per.Marshaler
	per.Unmarshaler
}](id ProtocolIEID, crit Criticality, field func(m *M) **T,
) ieSpec[M] {
	return ieSpec[M]{
		ID: id, Presence: presenceOptional, Criticality: crit,
		Decode: func(m *M, r *per.Reader, _ per.Encoding) error {
			v := PT(new(T))

			if err := decodeIE(r, v); err != nil {
				return err
			}

			*field(m) = (*T)(v)

			return nil
		},
		Encode: func(m *M) (per.Marshaler, bool) {
			v := *field(m)
			if v == nil {
				return nil, false
			}

			return PT(v), true
		},
	}
}

func listExtension[M any, L interface {
	~[]E
	per.Marshaler
}, E any, PL interface {
	*L
	per.Unmarshaler
}](id ProtocolIEID, crit Criticality, field func(m *M) *L,
) ieSpec[M] {
	return ieSpec[M]{
		ID: id, Presence: presenceOptional, Criticality: crit,
		Decode: func(m *M, r *per.Reader, _ per.Encoding) error {
			return decodeIE(r, PL(field(m)))
		},
		Encode: func(m *M) (per.Marshaler, bool) {
			v := *field(m)
			if len(v) == 0 {
				return nil, false
			}

			return v, true
		},
	}
}
