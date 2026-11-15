// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package impl

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ellanetworks/core/per"
)

type Kind uint8

const (
	KindNotComprehended Kind = iota
	KindMissing
	KindWrongOrder
	KindTooManyOccurrences
	KindErroneouslyPresent
)

var kindNames = [...]string{
	KindNotComprehended:    "not-comprehended",
	KindMissing:            "missing",
	KindWrongOrder:         "wrong-order",
	KindTooManyOccurrences: "too-many-occurrences",
	KindErroneouslyPresent: "erroneously-present",
}

func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}

	return fmt.Sprintf("Kind(%d)", uint8(k))
}

type Diagnostic[ID ~uint16] struct {
	Path        []ID
	ID          ID
	Criticality Criticality
	Kind        Kind

	pos int
}

func (d Diagnostic[ID]) String() string {
	var b strings.Builder

	for _, id := range d.Path {
		fmt.Fprintf(&b, "%v/", id)
	}

	fmt.Fprintf(&b, "%v (%s, %s)", d.ID, d.Criticality, d.Kind)

	return b.String()
}

type Diagnostics[ID ~uint16] struct {
	IEs                 []Diagnostic[ID]
	RootNotComprehended bool
}

func (d Diagnostics[ID]) Empty() bool { return len(d.IEs) == 0 && !d.RootNotComprehended }

func (d Diagnostics[ID]) Notify() []Diagnostic[ID] {
	var out []Diagnostic[ID]

	for _, e := range d.IEs {
		if e.Criticality == CriticalityNotify && (e.Kind == KindNotComprehended || e.Kind == KindMissing) {
			out = append(out, e)
		}
	}

	return out
}

func (d Diagnostics[ID]) ReportRequired() bool { return len(d.Notify()) > 0 }

func (d Diagnostics[ID]) Check() error {
	first := -1

	for i, e := range d.IEs {
		if (e.Kind == KindWrongOrder || e.Kind == KindTooManyOccurrences) && (first < 0 || e.pos < d.IEs[first].pos) {
			first = i
		}
	}

	if first >= 0 {
		var ies []Diagnostic[ID]

		for _, e := range d.IEs {
			if e.pos < d.IEs[first].pos && e.Kind == KindNotComprehended && e.Criticality == CriticalityReject {
				ies = append(ies, e)
			}
		}

		return &AbstractSyntaxError[ID]{FalselyConstructed: true, IEs: ies}
	}

	if ies := d.filter(KindNotComprehended, CriticalityReject); len(ies) > 0 || d.RootNotComprehended {
		return &AbstractSyntaxError[ID]{IEs: ies}
	}

	if slices.ContainsFunc(d.IEs, func(e Diagnostic[ID]) bool { return e.Kind == KindErroneouslyPresent }) {
		return &AbstractSyntaxError[ID]{FalselyConstructed: true}
	}

	if ies := d.filter(KindMissing, CriticalityReject); len(ies) > 0 {
		return &AbstractSyntaxError[ID]{IEs: ies}
	}

	return nil
}

func (d Diagnostics[ID]) filter(kind Kind, crit Criticality) []Diagnostic[ID] {
	var out []Diagnostic[ID]

	for _, e := range d.IEs {
		if e.Kind == kind && e.Criticality == crit {
			out = append(out, e)
		}
	}

	return out
}

type AbstractSyntaxError[ID ~uint16] struct {
	FalselyConstructed bool
	IEs                []Diagnostic[ID]
	Decoded            []Field[ID]
}

func (e *AbstractSyntaxError[ID]) Error() string {
	cause := "abstract syntax error (reject)"
	if e.FalselyConstructed {
		cause = "abstract syntax error (falsely constructed message)"
	}

	if len(e.IEs) == 0 {
		return "ap: " + cause
	}

	ies := make([]string, len(e.IEs))
	for i, d := range e.IEs {
		ies[i] = d.String()
	}

	return "ap: " + cause + ": " + strings.Join(ies, ", ")
}

type Diagnosable[ID ~uint16] interface {
	Diagnose(d *Diagnosis[ID]) bool
}

type Diagnosis[ID ~uint16] struct {
	entries []Diagnostic[ID]
	path    []ID
	pos     int
	prune   bool
}

func (d *Diagnosis[ID]) report(id ID, crit Criticality, kind Kind) {
	d.entries = append(d.entries, Diagnostic[ID]{
		Path:        slices.Clone(d.path),
		ID:          id,
		Criticality: crit,
		Kind:        kind,
		pos:         d.pos,
	})
}

func (d *Diagnosis[ID]) Unknown(id ID, crit Criticality) {
	d.report(id, crit, KindNotComprehended)
}

func (d *Diagnosis[ID]) IE(id ID, crit Criticality, v any) bool {
	dv, ok := v.(Diagnosable[ID])
	if !ok {
		return false
	}

	d.path = append(d.path, id)
	notComprehended := dv.Diagnose(d)
	d.path = d.path[:len(d.path)-1]

	if notComprehended {
		d.report(id, crit, KindNotComprehended)
	}

	return notComprehended
}

func (d *Diagnosis[ID]) Diagnostics() Diagnostics[ID] {
	return Diagnostics[ID]{IEs: slices.Clone(d.entries)}
}

func Diagnose[ID ~uint16](v Diagnosable[ID]) Diagnostics[ID] {
	return walk(v, false)
}

func Prune[ID ~uint16](v Diagnosable[ID]) Diagnostics[ID] {
	return walk(v, true)
}

func walk[ID ~uint16](v Diagnosable[ID], prune bool) Diagnostics[ID] {
	d := Diagnosis[ID]{prune: prune}

	notComprehended := v.Diagnose(&d)
	out := d.Diagnostics()
	out.RootNotComprehended = notComprehended

	return out
}

func DiagnoseExtensions[ID ~uint16, M any](d *Diagnosis[ID], table []IESpec[ID, M], m *M, pres *Preserved[ID]) {
	top := len(d.path) == 0
	slots := pres.slots()
	last := -1

	for i, s := range slots {
		if top {
			d.pos = i
		}

		switch {
		case s.duplicate:
			d.report(s.id, s.crit, KindTooManyOccurrences)
		case s.preserved:
			d.report(s.id, s.crit, KindNotComprehended)
		default:
			idx := lookup(table, s.id)
			if idx < last {
				d.report(s.id, s.crit, KindWrongOrder)
			}

			last = max(last, idx)
		}
	}

	for _, spec := range table {
		crit := spec.Criticality
		received := false

		if top {
			d.pos = len(slots)
		}

		for i, s := range slots {
			if s.id != spec.ID {
				continue
			}

			received = true

			if !s.preserved {
				crit = s.crit

				if top {
					d.pos = i
				}

				break
			}
		}

		val, ok := spec.Encode(m)

		switch {
		case ok && spec.Presence == PresenceConditional && !spec.Required(m):
			d.report(spec.ID, crit, KindErroneouslyPresent)
		case !ok && !received && spec.Required(m):
			d.report(spec.ID, crit, KindMissing)
		}

		if ok && d.IE(spec.ID, crit, val) && d.prune && spec.Clear != nil {
			raw, err := per.Marshal(val, per.Aligned)
			if err != nil {
				continue
			}

			*pres = demote(*pres, table, m, spec.ID, crit, raw)
			spec.Clear(m)
		}
	}
}

func DiagnoseChoiceExtension[ID ~uint16, T any, PT interface {
	*T
	per.Marshaler
}](d *Diagnosis[ID], id ID, crit Criticality, alt *PT, raw **SingleContainer[ID],
) bool {
	if !d.IE(id, crit, *alt) {
		return false
	}

	if d.prune {
		if b, err := per.Marshal(*alt, per.Aligned); err == nil {
			*raw = &SingleContainer[ID]{ID: id, Criticality: crit, Value: string(b)}
			*alt = nil
		}
	}

	return true
}

func DiagnoseUnknownChoiceExtension[ID ~uint16](d *Diagnosis[ID], c *SingleContainer[ID]) bool {
	d.Unknown(c.ID, c.Criticality)

	return true
}
