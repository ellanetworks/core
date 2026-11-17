// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package impl

import (
	"bytes"
	"fmt"
	"iter"

	"github.com/ellanetworks/core/per"
)

const fragmentedForm = 0xC0

type openType struct {
	det     []byte
	content []byte
	raw     []byte
}

type rawField[ID ~uint16] struct {
	id      ID
	crit    Criticality
	hdr     []byte
	val     openType
	content []byte
}

type RawPDU[ID ~uint16, P ~uint8, T ~uint8] struct {
	profile *Profile
	header  Envelope
	opaque  bool

	orig     []byte
	head     []byte
	body     openType
	bodyHdr  []byte
	count    []byte
	fields   []rawField[ID]
	bodyTail []byte

	countChanged bool
	dirty        bool
}

func ParseRawPDU[ID ~uint16, P ~uint8, T ~uint8](p *Profile, b []byte) (*RawPDU[ID, P, T], error) {
	if err := p.validate(); err != nil {
		return nil, err
	}

	b = bytes.Clone(b)
	r := per.NewReader(b)

	header, err := decodeHeader[P](p, r)
	if err != nil {
		return nil, err
	}

	r.AlignToByte()
	headEnd := r.Offset() / 8

	body, err := readOpenType(r, b)
	if err == nil {
		err = r.Complete()
	}

	if err != nil {
		return nil, procedureSyntaxError(p, P(header.ProcedureCode), fmt.Errorf("value: %w", err))
	}

	header.Value = body.content

	m := &RawPDU[ID, P, T]{
		profile: p,
		header:  header,
		opaque:  p.Opaque(header.ProcedureCode),
		orig:    b,
		head:    b[:headEnd],
		body:    body,
	}

	if m.opaque {
		return m, nil
	}

	if err := m.parseBody(); err != nil {
		return nil, procedureSyntaxError(p, P(header.ProcedureCode), err)
	}

	return m, nil
}

func (m *RawPDU[ID, P, T]) parseBody() error {
	p := m.profile
	content := m.body.content
	r := per.NewReader(content)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return fmt.Errorf("message preamble: %w", err)
	}

	r.AlignToByte()
	hdrEnd := r.Offset() / 8

	n, err := per.DecodeConstrainedWholeNumber(r, enc, 0, p.MaxProtocolIEs)
	if err != nil {
		return fmt.Errorf("IE count: %w", err)
	}

	countEnd := r.Offset() / 8

	if n > int64(len(content)-countEnd)/(minFieldBits/8) {
		return fmt.Errorf("%d IEs declared in %d octets", n, len(content)-countEnd)
	}

	m.bodyHdr = content[:hdrEnd]
	m.count = content[hdrEnd:countEnd]
	m.fields = make([]rawField[ID], 0, n)

	for i := range n {
		start := r.Offset() / 8

		id, err := per.DecodeConstrainedWholeNumber(r, enc, 0, p.MaxProtocolIEs)
		if err != nil {
			return fmt.Errorf("IE %d id: %w", i, err)
		}

		crit, err := per.DecodeEnumerated(r, enc, criticalityCount, false)
		if err != nil {
			return fmt.Errorf("IE %d criticality: %w", i, err)
		}

		r.AlignToByte()
		hdrEnd := r.Offset() / 8

		val, err := readOpenType(r, content)
		if err != nil {
			return fmt.Errorf("IE %d value: %w", i, err)
		}

		m.fields = append(m.fields, rawField[ID]{id: ID(id), crit: Criticality(crit), hdr: content[start:hdrEnd], val: val, content: val.content})
	}

	tail := r.Offset() / 8

	if extPresent {
		var additions per.ExtensionAdditions

		if err := additions.UnmarshalPER(r, enc); err != nil {
			return fmt.Errorf("extension additions: %w", err)
		}
	}

	if err := r.Complete(); err != nil {
		return err
	}

	m.bodyTail = content[tail:]

	return nil
}

func readOpenType(r *per.Reader, buf []byte) (openType, error) {
	start := r.Offset() / 8

	content, err := per.DecodeOpenTypeBytes(r, per.Aligned)
	if err != nil {
		return openType{}, err
	}

	raw := buf[start : r.Offset()/8]
	ot := openType{content: content, raw: raw}

	if raw[0]&fragmentedForm != fragmentedForm {
		ot.det = raw[:len(raw)-len(content)]
	}

	return ot, nil
}

func (m *RawPDU[ID, P, T]) Trigger() T { return T(m.header.Choice) }

func (m *RawPDU[ID, P, T]) ProcedureCode() P { return P(m.header.ProcedureCode) }

func (m *RawPDU[ID, P, T]) Criticality() Criticality { return m.header.Criticality }

func (m *RawPDU[ID, P, T]) Opaque() bool { return m.opaque }

func (m *RawPDU[ID, P, T]) Body() []byte { return bytes.Clone(m.body.content) }

func (m *RawPDU[ID, P, T]) Len() int { return len(m.fields) }

func (m *RawPDU[ID, P, T]) container() error {
	if m.opaque {
		return m.profile.errorf("%s: %w", m.profile.procedureName(m.header.ProcedureCode), ErrNotProtocolIEContainer)
	}

	return nil
}

func (m *RawPDU[ID, P, T]) IE(i int) (Field[ID], error) {
	if i < 0 || i >= len(m.fields) {
		return Field[ID]{}, ErrIndex
	}

	f := m.fields[i]

	return Field[ID]{ID: f.id, Criticality: f.crit, Value: bytes.Clone(f.content)}, nil
}

func (m *RawPDU[ID, P, T]) IEs() []Field[ID] {
	out := make([]Field[ID], len(m.fields))
	for i := range m.fields {
		out[i], _ = m.IE(i)
	}

	return out
}

func (m *RawPDU[ID, P, T]) All(id ID) iter.Seq2[int, Field[ID]] {
	return func(yield func(int, Field[ID]) bool) {
		for i, f := range m.fields {
			if f.id != id {
				continue
			}

			if !yield(i, Field[ID]{ID: f.id, Criticality: f.crit, Value: bytes.Clone(f.content)}) {
				return
			}
		}
	}
}

func (m *RawPDU[ID, P, T]) Index(id ID) (int, error) {
	idx := -1

	for i, f := range m.fields {
		if f.id != id {
			continue
		}

		if idx >= 0 {
			return -1, m.profile.errorf("%w: %s", ErrDuplicateIE, m.profile.ieName(uint16(id)))
		}

		idx = i
	}

	return idx, nil
}

func (m *RawPDU[ID, P, T]) Get(id ID, v per.Unmarshaler) (bool, error) {
	i, err := m.Index(id)
	if err != nil || i < 0 {
		return false, err
	}

	if err := DecodeValue(m.fields[i].content, v); err != nil {
		return false, m.profile.errorf("%s: %w", m.profile.ieName(uint16(id)), err)
	}

	return true, nil
}

func (m *RawPDU[ID, P, T]) Set(id ID, v per.Marshaler) error {
	i, err := m.Index(id)
	if err != nil {
		return err
	}

	if i < 0 {
		return m.profile.errorf("%w: %s", ErrIENotPresent, m.profile.ieName(uint16(id)))
	}

	b, err := per.Marshal(v, per.Aligned)
	if err != nil {
		return m.profile.errorf("%s: %w", m.profile.ieName(uint16(id)), err)
	}

	return m.SetValue(i, b)
}

func (m *RawPDU[ID, P, T]) SetValue(i int, value []byte) error {
	if i < 0 || i >= len(m.fields) {
		return ErrIndex
	}

	m.fields[i].content = bytes.Clone(value)
	m.dirty = true

	return nil
}

func (m *RawPDU[ID, P, T]) Insert(i int, f Field[ID]) error {
	if err := m.container(); err != nil {
		return err
	}

	if i < 0 || i > len(m.fields) {
		return ErrIndex
	}

	if int64(len(m.fields)) >= m.profile.MaxProtocolIEs {
		return ErrTooManyFields
	}

	if f.Criticality >= criticalityCount {
		return ErrCriticality
	}

	nf := rawField[ID]{id: f.ID, crit: f.Criticality, content: append([]byte{}, f.Value...)}

	m.fields = append(m.fields, rawField[ID]{})
	copy(m.fields[i+1:], m.fields[i:])
	m.fields[i] = nf
	m.countChanged = true
	m.dirty = true

	return nil
}

func (m *RawPDU[ID, P, T]) Remove(i int) error {
	if i < 0 || i >= len(m.fields) {
		return ErrIndex
	}

	m.fields = append(m.fields[:i], m.fields[i+1:]...)
	m.countChanged = true
	m.dirty = true

	return nil
}

func (m *RawPDU[ID, P, T]) Marshal() ([]byte, error) {
	if !m.dirty {
		return bytes.Clone(m.orig), nil
	}

	content := make([]byte, 0, len(m.body.content)+16)
	content = append(content, m.bodyHdr...)

	if m.countChanged {
		w := per.NewWriter()
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, m.profile.MaxProtocolIEs, int64(len(m.fields))); err != nil {
			return nil, err
		}

		content = append(content, w.Bytes()...)
	} else {
		content = append(content, m.count...)
	}

	for _, f := range m.fields {
		enc, err := m.encodeField(f)
		if err != nil {
			return nil, err
		}

		content = append(content, enc...)
	}

	content = append(content, m.bodyTail...)

	body, err := m.body.encode(content)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, len(m.head)+len(body))
	out = append(out, m.head...)

	return append(out, body...), nil
}

func (m *RawPDU[ID, P, T]) encodeField(f rawField[ID]) ([]byte, error) {
	hdr := f.hdr
	if hdr == nil {
		w := per.NewWriter()
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, m.profile.MaxProtocolIEs, int64(f.id)); err != nil {
			return nil, err
		}

		if err := per.EncodeEnumerated(w, per.Aligned, criticalityCount, false, int64(f.crit)); err != nil {
			return nil, err
		}

		hdr = w.Bytes()
	}

	val, err := f.val.encode(f.content)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, len(hdr)+len(val))
	out = append(out, hdr...)

	return append(out, val...), nil
}

func (o openType) encode(content []byte) ([]byte, error) {
	if o.raw != nil && bytes.Equal(content, o.content) {
		return o.raw, nil
	}

	if o.det != nil && len(content) == len(o.content) && len(content) > 0 {
		out := make([]byte, 0, len(o.det)+len(content))
		out = append(out, o.det...)

		return append(out, content...), nil
	}

	w := per.NewWriter()
	if err := per.EncodeOpenTypeBytes(w, per.Aligned, content); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}
