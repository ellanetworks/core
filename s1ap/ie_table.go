// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// ieSpec is one row of a message's TS 36.413 §9.1 IE table. Encode and decode
// both read it, so the two directions cannot disagree on an IE's criticality
// or presence.
type ieSpec[M any] struct {
	id       ProtocolIEID
	presence Presence
	crit     Criticality

	decode func(m *M, raw []byte, enc per.Encoding) error
	// encode reports false when the row's optional field is unset.
	encode func(m *M) (per.Marshaler, bool)
}

func (u *unmodeledIEs) unmodeled() *unmodeledIEs { return u }

// message is satisfied by a pointer to any message struct.
type message interface {
	unmodeled() *unmodeledIEs
}

// encodeMessageBody writes the SEQUENCE extension bit, then every present IE
// in table order, then any IE preserved verbatim from a previous decode.
func encodeMessageBody[M any, PM interface {
	*M
	message
}](w *per.Writer, enc per.Encoding, table []ieSpec[M], m PM,
) error {
	w.WriteBit(false)

	fields := make([]ieField, 0, len(table))

	for _, spec := range table {
		val, ok := spec.encode((*M)(m))
		if !ok {
			continue
		}

		fields = append(fields, ieField{id: spec.id, crit: spec.crit, val: val})
	}

	for _, e := range m.unmodeled().unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// parseMessageBody decodes an IE container against its table. IEs the table
// does not name are preserved verbatim rather than dropped.
func parseMessageBody[M any, PM interface {
	*M
	message
}](procedure ProcedureCode, table []ieSpec[M], value []byte,
) (PM, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: %s preamble: %w", procedure, err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, err
		}
	}

	m := PM(new(M))
	seen := make(map[ProtocolIEID]bool, len(table))

	for _, f := range fields {
		spec, ok := lookupIESpec(table, f.id)
		if !ok {
			u := m.unmodeled()
			u.unknownIEs = append(u.unknownIEs, f)

			continue
		}

		if err := spec.decode((*M)(m), f.value, enc); err != nil {
			return nil, fmt.Errorf("s1ap: %s %s: %w", procedure, f.id, err)
		}

		seen[f.id] = true
	}

	checks := make([]ieCheck, 0, len(table))

	for _, spec := range table {
		if spec.presence == PresenceMandatory {
			checks = append(checks, ieCheck{spec.id, spec.crit, seen[spec.id]})
		}
	}

	if err := requireIEs(procedure, checks...); err != nil {
		return nil, err
	}

	return m, nil
}

// Tables run to a handful of rows, so a linear scan beats building a map per
// parse.
func lookupIESpec[M any](table []ieSpec[M], id ProtocolIEID) (ieSpec[M], bool) {
	for _, spec := range table {
		if spec.id == id {
			return spec, true
		}
	}

	return ieSpec[M]{}, false
}
