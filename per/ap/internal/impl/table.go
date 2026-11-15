// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package impl

import (
	"fmt"
	"slices"

	"github.com/ellanetworks/core/per"
)

type IESpec[ID ~uint16, M any] struct {
	ID          ID
	Presence    Presence
	Criticality Criticality
	Condition   func(m *M) bool
	Decode      func(m *M, r *per.Reader, enc per.Encoding) error
	Encode      func(m *M) (per.Marshaler, bool)
	Clear       func(m *M)
}

func (s IESpec[ID, M]) Required(m *M) bool {
	switch s.Presence {
	case PresenceMandatory:
		return true
	case PresenceConditional:
		return s.Condition != nil && s.Condition(m)
	default:
		return false
	}
}

func lookup[ID ~uint16, M any](table []IESpec[ID, M], id ID) int {
	for i := range table {
		if table[i].ID == id {
			return i
		}
	}

	return -1
}

func EncodeMessage[ID ~uint16, M any](p *Profile, w *per.Writer, enc per.Encoding, table []IESpec[ID, M], m *M, meta *Message[ID]) error {
	typed := make([]*FieldValue[ID], len(table))

	for i, spec := range table {
		val, ok := spec.Encode(m)
		required := spec.Required(m)
		exempt := slices.Contains(meta.exempt, spec.ID)

		if ok && meta.unchangedAbsent(spec.ID, val) {
			ok = false
		}

		if !ok {
			if required && !exempt {
				return fmt.Errorf("required IE %s is not set", p.ieName(uint16(spec.ID)))
			}

			continue
		}

		if spec.Presence == PresenceConditional && !required && !exempt {
			return fmt.Errorf("conditional IE %s is set but its condition does not hold", p.ieName(uint16(spec.ID)))
		}

		typed[i] = &FieldValue[ID]{ID: spec.ID, Criticality: spec.Criticality, Value: val}
	}

	w.WriteBit(meta.additions.Present())

	if err := EncodeIEContainer(p, w, enc, orderFields(table, typed, meta.preserved)); err != nil {
		return err
	}

	return meta.additions.MarshalPER(w, enc)
}

func ExtensionFields[ID ~uint16, M any](table []IESpec[ID, M], m *M, pres Preserved[ID]) []FieldValue[ID] {
	typed := make([]*FieldValue[ID], len(table))

	for i, spec := range table {
		if val, ok := spec.Encode(m); ok {
			typed[i] = &FieldValue[ID]{ID: spec.ID, Criticality: spec.Criticality, Value: val}
		}
	}

	return orderFields(table, typed, pres)
}

func orderFields[ID ~uint16, M any](table []IESpec[ID, M], typed []*FieldValue[ID], pres Preserved[ID]) []FieldValue[ID] {
	fields := make([]FieldValue[ID], 0, len(table))
	rest := pres.fields

	nextRaw := func() FieldValue[ID] {
		var f Field[ID]

		f, rest = nextPreserved[ID](rest)

		return FieldValue[ID]{ID: f.ID, Criticality: f.Criticality, Value: RawValue(f.Value)}
	}

	slots := pres.slots()
	onWire := make([]bool, len(table))

	for _, s := range slots {
		if idx := lookup(table, s.id); idx >= 0 && !s.preserved {
			onWire[idx] = true
		}
	}

	next := 0

	flush := func(upto int) {
		for ; next < upto; next++ {
			if typed[next] != nil && !onWire[next] {
				fields = append(fields, *typed[next])
			}
		}
	}

	for _, s := range slots {
		if s.preserved {
			fields = append(fields, nextRaw())

			continue
		}

		idx := lookup(table, s.id)
		if idx < 0 {
			continue
		}

		flush(idx)

		if f := typed[idx]; f != nil {
			fields = append(fields, FieldValue[ID]{ID: f.ID, Criticality: s.crit, Value: f.Value})
		}

		next = max(next, idx+1)
	}

	flush(len(table))

	for rest != "" {
		fields = append(fields, nextRaw())
	}

	return fields
}

type outcome uint8

const (
	outcomeDecoded outcome = iota
	outcomeUnknown
	outcomeDuplicate
	outcomePruned
)

type decodedField struct {
	idx        int
	outcome    outcome
	wrongOrder bool
}

func DecodeMessage[ID ~uint16, M any](p *Profile, table []IESpec[ID, M], value []byte, m *M, meta *Message[ID], prune bool) error {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return fmt.Errorf("preamble: %w", err)
	}

	fields, err := DecodeIEContainer[ID](p, r, enc)
	if err != nil {
		return err
	}

	var additions per.ExtensionAdditions

	if extPresent {
		if err := additions.UnmarshalPER(r, enc); err != nil {
			return err
		}
	}

	if err := r.Complete(); err != nil {
		return err
	}

	results, pres, err := decodeTable(p, table, fields, m, nil)
	if err != nil {
		return err
	}

	d := Diagnosis[ID]{prune: prune}

	var pruned []bool

	seen := make([]bool, len(table))

	for i, f := range fields {
		d.pos = i

		res := results[i]

		switch res.outcome {
		case outcomeUnknown, outcomePruned:
			d.report(f.ID, f.Criticality, KindNotComprehended)
		case outcomeDuplicate:
			d.report(f.ID, f.Criticality, KindTooManyOccurrences)
		case outcomeDecoded:
			seen[res.idx] = true

			if res.wrongOrder {
				d.report(f.ID, f.Criticality, KindWrongOrder)
			}

			val, ok := table[res.idx].Encode(m)
			if ok && d.IE(f.ID, f.Criticality, val) && prune {
				if pruned == nil {
					pruned = make([]bool, len(fields))
				}

				pruned[i] = true
			}
		}
	}

	d.pos = len(fields)

	var exempt []ID

	for i := range table {
		spec := &table[i]

		switch {
		case spec.Presence == PresenceOptional, seen[i] && spec.Presence == PresenceMandatory:
		case seen[i] && !spec.Required(m):
			d.report(spec.ID, spec.Criticality, KindErroneouslyPresent)
			exempt = append(exempt, spec.ID)
		case !seen[i] && spec.Required(m):
			d.report(spec.ID, spec.Criticality, KindMissing)
			exempt = append(exempt, spec.ID)
		}
	}

	diagnostics := d.Diagnostics()

	if pruned != nil {
		var zero M

		*m = zero

		if results, pres, err = decodeTable(p, table, fields, m, pruned); err != nil {
			return err
		}

		nested := Diagnosis[ID]{prune: true}

		for i, f := range fields {
			if results[i].outcome != outcomeDecoded {
				continue
			}

			if val, ok := table[results[i].idx].Encode(m); ok {
				nested.IE(f.ID, f.Criticality, val)
			}
		}

		for i, f := range fields {
			if pruned[i] && table[results[i].idx].Required(m) {
				exempt = append(exempt, f.ID)
			}
		}
	}

	var absent []absentIE[ID]

	for i := range table {
		spec := &table[i]

		if seen[i] && !prunedID(fields, pruned, spec.ID) {
			continue
		}

		if val, ok := spec.Encode(m); ok {
			absent = append(absent, absentIE[ID]{id: spec.ID, encoded: encodedValue(val)})
		}
	}

	*meta = Message[ID]{
		preserved:   pres,
		additions:   additions,
		diagnostics: diagnostics,
		exempt:      exempt,
		absent:      absent,
	}

	return nil
}

func prunedID[ID ~uint16](fields []Field[ID], pruned []bool, id ID) bool {
	for i := range pruned {
		if pruned[i] && fields[i].ID == id {
			return true
		}
	}

	return false
}

func DecodeExtensionContainer[ID ~uint16, M any](p *Profile, r *per.Reader, enc per.Encoding, table []IESpec[ID, M], m *M, pres *Preserved[ID]) error {
	fields, err := decodeExtensionFields[ID](p, r, enc, false)
	if err != nil {
		return err
	}

	_, decoded, err := decodeTable(p, table, fields, m, nil)
	if err != nil {
		return err
	}

	*pres = decoded

	return nil
}

func decodeTable[ID ~uint16, M any](p *Profile, table []IESpec[ID, M], fields []Field[ID], m *M, skip []bool) ([]decodedField, Preserved[ID], error) {
	results := make([]decodedField, len(fields))
	occurred := make([]bool, len(table))
	b := newPreservedBuilder[ID]()
	last := -1
	r := per.NewReader(nil)

	for i, f := range fields {
		idx := lookup(table, f.ID)
		results[i].idx = idx

		switch {
		case idx < 0:
			results[i].outcome = outcomeUnknown

			b.keep(f, false)

			continue
		case occurred[idx]:
			results[i].outcome = outcomeDuplicate

			b.keep(f, true)

			continue
		}

		occurred[idx] = true
		results[i].wrongOrder = idx < last
		last = max(last, idx)

		if skip != nil && skip[i] {
			results[i].outcome = outcomePruned

			b.keep(f, false)

			continue
		}

		r.Reset(f.Value)

		err := table[idx].Decode(m, r, per.Aligned)
		if err == nil {
			err = r.Complete()
		}

		if err != nil {
			return nil, Preserved[ID]{}, fmt.Errorf("IE %s: %w", p.ieName(uint16(f.ID)), err)
		}

		b.known(f.ID, f.Criticality, idx, table[idx].Criticality)
	}

	return results, b.build(), nil
}

func encodedValue(v per.Marshaler) string {
	b, err := per.Marshal(v, per.Aligned)
	if err != nil {
		return "\x00error:" + err.Error()
	}

	return string(b)
}

func (m *Message[ID]) unchangedAbsent(id ID, v per.Marshaler) bool {
	for _, a := range m.absent {
		if a.id == id {
			return a.encoded == encodedValue(v)
		}
	}

	return false
}
