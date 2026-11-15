// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package impl

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/per"
)

func EncodeMessageBody[ID ~uint16, P ~uint8, M any](p *Profile, w *per.Writer, enc per.Encoding, procedure P, table []IESpec[ID, M], m *M, meta *Message[ID]) error {
	if err := EncodeMessage(p, w, enc, table, m, meta); err != nil {
		return fmt.Errorf("%s: %v: %w", p.Name, procedure, err)
	}

	return nil
}

func DecodeMessageBody[ID ~uint16, P ~uint8, M any](p *Profile, procedure P, table []IESpec[ID, M], value []byte, m *M, meta *Message[ID]) error {
	if err := DecodeMessage(p, table, value, m, meta, false); err != nil {
		return procedureSyntaxError(p, procedure, err)
	}

	return nil
}

func ParseMessageBody[ID ~uint16, P ~uint8, M any](p *Profile, procedure P, table []IESpec[ID, M], value []byte, m *M, meta *Message[ID]) error {
	if err := DecodeMessage(p, table, value, m, meta, true); err != nil {
		return procedureSyntaxError(p, procedure, err)
	}

	err := meta.diagnostics.Check()

	var e *AbstractSyntaxError[ID]
	if errors.As(err, &e) {
		e.Decoded = modeledIEs(p, table, value)
	}

	return err
}

func modeledIEs[ID ~uint16, M any](p *Profile, table []IESpec[ID, M], value []byte) []Field[ID] {
	r := per.NewReader(value)

	if _, err := r.ReadBit(); err != nil {
		return nil
	}

	fields, err := DecodeIEContainer[ID](p, r, per.Aligned)
	if err != nil {
		return nil
	}

	var out []Field[ID]

	seen := make([]bool, len(table))

	for _, f := range fields {
		idx := lookup(table, f.ID)
		if idx < 0 || seen[idx] {
			continue
		}

		seen[idx] = true

		out = append(out, f)
	}

	return out
}

func DecodeTransfer[ID ~uint16, T any, PT interface {
	*T
	per.Unmarshaler
}](b []byte,
) (PT, error) {
	v := PT(new(T))

	if err := DecodeValue(b, v); err != nil {
		return nil, err
	}

	return v, nil
}

func ParseTransfer[ID ~uint16, T any, PT interface {
	*T
	per.Unmarshaler
}](b []byte,
) (PT, error) {
	v, err := DecodeTransfer[ID, T, PT](b)
	if err != nil {
		return nil, err
	}

	dv, ok := any(v).(Diagnosable[ID])
	if !ok {
		return v, nil
	}

	if err := Prune(dv).Check(); err != nil {
		return nil, err
	}

	return v, nil
}

func DiagnosticsOf[ID ~uint16](v any) Diagnostics[ID] {
	dv, ok := v.(Diagnosable[ID])
	if !ok {
		return Diagnostics[ID]{}
	}

	return Diagnose(dv)
}

func DiagnosticItems[ID ~uint16, T any](ds []Diagnostic[ID], limit int, item func(d Diagnostic[ID], missing bool) T) []T {
	var out []T

	for _, d := range ds {
		if len(out) >= limit {
			break
		}

		out = append(out, item(d, d.Kind == KindMissing))
	}

	return out
}

func Reportable[T any](items []T, criticality func(T) Criticality) []T {
	out := make([]T, 0, len(items))

	for _, it := range items {
		if criticality(it) != CriticalityIgnore {
			out = append(out, it)
		}
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

func OptionalIE[ID ~uint16, M any, T any, PT interface {
	*T
	per.Marshaler
	per.Unmarshaler
}](id ID, crit Criticality, field func(m *M) **T,
) IESpec[ID, M] {
	return IESpec[ID, M]{
		ID: id, Presence: PresenceOptional, Criticality: crit,
		Decode: func(m *M, r *per.Reader, enc per.Encoding) error {
			v := PT(new(T))

			if err := v.UnmarshalPER(r, enc); err != nil {
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
		Clear: func(m *M) { *field(m) = nil },
	}
}

func ConditionalIE[ID ~uint16, M any, T any, PT interface {
	*T
	per.Marshaler
	per.Unmarshaler
}](id ID, crit Criticality, cond func(m *M) bool, field func(m *M) **T,
) IESpec[ID, M] {
	s := OptionalIE[ID, M, T, PT](id, crit, field)
	s.Presence, s.Condition = PresenceConditional, cond

	return s
}

func ListIE[ID ~uint16, M any, L interface {
	~[]E
	per.Marshaler
}, E any, PL interface {
	*L
	per.Marshaler
	per.Unmarshaler
}](id ID, crit Criticality, field func(m *M) *L,
) IESpec[ID, M] {
	return IESpec[ID, M]{
		ID: id, Presence: PresenceOptional, Criticality: crit,
		Decode: func(m *M, r *per.Reader, enc per.Encoding) error {
			return PL(field(m)).UnmarshalPER(r, enc)
		},
		Encode: func(m *M) (per.Marshaler, bool) {
			if len(*field(m)) == 0 {
				return nil, false
			}

			return PL(field(m)), true
		},
		Clear: func(m *M) { *field(m) = nil },
	}
}
