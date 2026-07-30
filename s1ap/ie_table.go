// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// ieSpec is one row of a message's IE table: the §9.1 entry for a single
// ProtocolIE, bound to the message type that carries it.
//
// The row is the only place a message states an IE's id, presence and
// criticality, and both directions read it — so an encoder cannot stamp a
// criticality the decoder does not expect, and a mandatory IE cannot be
// enforced on one side only.
type ieSpec[M any] struct {
	id       ProtocolIEID
	presence Presence
	crit     Criticality

	// decode fills the field this row owns from the IE's open-type octets.
	decode func(m *M, raw []byte, enc per.Encoding) error
	// encode returns the IE's value, and whether it is present. A row for a
	// mandatory IE always reports true; an optional one reports whether its
	// field is set.
	encode func(m *M) (per.Marshaler, bool)
}

// unmodeled lets the engine reach the preserved-IE store that every message
// embeds, without reflection.
func (u *unmodeledIEs) unmodeled() *unmodeledIEs { return u }

// message is satisfied by a pointer to any message struct, all of which embed
// unmodeledIEs.
type message interface {
	unmodeled() *unmodeledIEs
}

// encodeMessageBody writes a message's IE container from its table: the
// SEQUENCE extension bit, then every present IE in table order, then the
// unmodeled IEs preserved from a previous decode.
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

// parseMessageBody decodes a message's IE container using its table. IEs the
// table does not name are preserved verbatim; every mandatory row that did not
// arrive is reported through [MissingMandatoryIEsError].
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

// lookupIESpec finds the row an IE id belongs to. Tables are short — a handful
// of rows each — so a linear scan beats building a map per parse.
func lookupIESpec[M any](table []ieSpec[M], id ProtocolIEID) (ieSpec[M], bool) {
	for _, spec := range table {
		if spec.id == id {
			return spec, true
		}
	}

	return ieSpec[M]{}, false
}
