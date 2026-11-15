// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ap

import (
	"encoding/binary"
	"slices"

	"github.com/ellanetworks/core/per"
)

const (
	slotSize      = 3
	slotPreserved = 0x80
	slotDuplicate = 0x40
	slotFlags     = slotPreserved | slotDuplicate
)

type Preserved[ID ~uint16] struct {
	fields string
	order  string
}

func (p Preserved[ID]) UnknownIEs() []Field[ID] {
	var out []Field[ID]

	for rest := p.fields; rest != ""; {
		var f Field[ID]

		f, rest = nextPreserved[ID](rest)
		out = append(out, f)
	}

	return out
}

func nextPreserved[ID ~uint16](s string) (Field[ID], string) {
	id := ID(uint16(s[0])<<8 | uint16(s[1]))
	crit := Criticality(s[2])
	n, w := binary.Uvarint([]byte(s[3:min(len(s), 3+binary.MaxVarintLen64)]))
	start := 3 + w
	end := start + int(n)

	return Field[ID]{ID: id, Criticality: crit, Value: []byte(s[start:end])}, s[end:]
}

type slot[ID ~uint16] struct {
	id        ID
	crit      Criticality
	preserved bool
	duplicate bool
}

func (p Preserved[ID]) slots() []slot[ID] {
	out := make([]slot[ID], 0, len(p.order)/slotSize)

	for i := 0; i+slotSize <= len(p.order); i += slotSize {
		out = append(out, slot[ID]{
			id:        ID(uint16(p.order[i])<<8 | uint16(p.order[i+1])),
			crit:      Criticality(p.order[i+2] &^ slotFlags),
			preserved: p.order[i+2]&slotPreserved != 0,
			duplicate: p.order[i+2]&slotDuplicate != 0,
		})
	}

	return out
}

type preservedBuilder[ID ~uint16] struct {
	fields    []byte
	order     []byte
	canonical bool
	last      int
}

func newPreservedBuilder[ID ~uint16]() *preservedBuilder[ID] {
	return &preservedBuilder[ID]{canonical: true, last: -1}
}

func (b *preservedBuilder[ID]) keep(f Field[ID], duplicate bool) {
	flags := byte(slotPreserved)
	if duplicate {
		flags |= slotDuplicate
	}

	b.fields = binary.BigEndian.AppendUint16(b.fields, uint16(f.ID))
	b.fields = append(b.fields, byte(f.Criticality))
	b.fields = binary.AppendUvarint(b.fields, uint64(len(f.Value)))
	b.fields = append(b.fields, f.Value...)
	b.order = binary.BigEndian.AppendUint16(b.order, uint16(f.ID))
	b.order = append(b.order, byte(f.Criticality)|flags)
	b.canonical = false
}

func (b *preservedBuilder[ID]) known(id ID, crit Criticality, idx int, specCrit Criticality) {
	b.order = binary.BigEndian.AppendUint16(b.order, uint16(id))
	b.order = append(b.order, byte(crit))

	if idx <= b.last || crit != specCrit {
		b.canonical = false
	}

	b.last = idx
}

func (b *preservedBuilder[ID]) build() Preserved[ID] {
	if b.canonical {
		return Preserved[ID]{}
	}

	return Preserved[ID]{fields: string(b.fields), order: string(b.order)}
}

type Message[ID ~uint16] struct {
	Preserved[ID]

	diagnostics        Diagnostics[ID]
	exempt             []ID
	absent             []absentIE[ID]
	ExtensionAdditions per.ExtensionAdditions
}

func (m Message[ID]) Diagnostics() Diagnostics[ID] { return m.diagnostics }

func (m Message[ID]) Equal(o Message[ID]) bool {
	return m.Preserved == o.Preserved && m.ExtensionAdditions == o.ExtensionAdditions &&
		slices.Equal(m.exempt, o.exempt) && slices.Equal(m.absent, o.absent)
}

func (p *Preserved[ID]) Append(f Field[ID]) {
	b := preservedBuilder[ID]{fields: []byte(p.fields), order: []byte(p.order)}

	b.keep(f, false)

	p.fields, p.order = string(b.fields), string(b.order)
}

type absentIE[ID ~uint16] struct {
	id      ID
	encoded string
}
