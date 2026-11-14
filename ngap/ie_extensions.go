// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/per"
)

func extensionFields[M any](table []ieSpec[M], m *M, meta *messageMeta) []ieField {
	typed := make([]*ieField, len(table))

	for i, spec := range table {
		if val, ok := spec.encode(m); ok {
			typed[i] = &ieField{id: spec.id, crit: spec.crit, val: val}
		}
	}

	return orderFields(table, typed, meta)
}

func encodeExtensionContainer(w *per.Writer, enc per.Encoding, fields []ieField) error {
	if len(fields) > maxProtocolExtensions {
		return fmt.Errorf("ngap: %d extensions exceed maxProtocolExtensions", len(fields))
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 1, maxProtocolExtensions, int64(len(fields))); err != nil {
		return err
	}

	for _, f := range fields {
		if err := encodeContainerField(w, enc, f); err != nil {
			return err
		}
	}

	return nil
}

func decodeExtensionContainer[M any](r *per.Reader, enc per.Encoding, table []ieSpec[M], m *M, meta *messageMeta) error {
	n, err := per.DecodeConstrainedWholeNumber(r, enc, 1, maxProtocolExtensions)
	if err != nil {
		return err
	}

	if maxBits := int64(r.Bits()); n > maxBits/minIEFieldBits {
		return fmt.Errorf("ngap: extension container declares %d fields in %d bits", n, maxBits)
	}

	var rejected *notComprehendedIE

	lastIdx := -1

	for i := int64(0); i < n; i++ {
		id, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs)
		if err != nil {
			return err
		}

		crit, err := per.DecodeEnumerated(r, enc, criticalityRootCount, false)
		if err != nil {
			return err
		}

		val, err := per.DecodeOpenTypeBytes(r, enc)
		if err != nil {
			return err
		}

		if len(val) == 0 {
			return fmt.Errorf("ngap: extension %d has an empty open type", id)
		}

		f := rawIE{id: ProtocolIEID(id), crit: Criticality(crit), value: val}

		if idx, spec, ok := lookupIESpec(table, f.id); ok {
			if idx <= lastIdx {
				return fmt.Errorf("ngap: extension %s out of order or repeated", f.id)
			}

			lastIdx = idx

			err := spec.decode(m, f.value, enc)
			if err == nil {
				meta.wire = append(meta.wire, wireSlot{id: f.id, crit: f.crit, unknown: -1})

				continue
			}

			if !errors.Is(err, errNotComprehended) {
				return err
			}

			var inner *notComprehendedIE
			if errors.As(err, &inner) && inner.Crit == CriticalityReject && rejected == nil {
				rejected = inner
			}
		}

		if f.crit == CriticalityReject {
			if rejected == nil {
				rejected = &notComprehendedIE{ID: f.id, Crit: CriticalityReject, What: "iE-Extensions"}
			}

			continue
		}

		meta.preserveUpTo(f, maxProtocolExtensions)
	}

	if rejected != nil {
		return rejected
	}

	if wireMatchesTable(table, meta.wire) {
		meta.wire = nil
	}

	return nil
}

func optionalExtension[M any, T any, PT interface {
	*T
	per.Marshaler
	per.Unmarshaler
}](id ProtocolIEID, crit Criticality, field func(m *M) **T,
) ieSpec[M] {
	return ieSpec[M]{
		id: id, presence: presenceOptional, crit: crit,
		decode: func(m *M, raw []byte, _ per.Encoding) error {
			v := PT(new(T))

			if err := perIEDecode(raw, v); err != nil {
				return err
			}

			*field(m) = (*T)(v)

			return nil
		},
		encode: func(m *M) (per.Marshaler, bool) {
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
		id: id, presence: presenceOptional, crit: crit,
		decode: func(m *M, raw []byte, _ per.Encoding) error {
			return perIEDecode(raw, PL(field(m)))
		},
		encode: func(m *M) (per.Marshaler, bool) {
			v := *field(m)
			if len(v) == 0 {
				return nil, false
			}

			return v, true
		},
	}
}
