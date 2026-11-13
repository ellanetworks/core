// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package protocolie

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/per"
)

const (
	pduChoiceCount   = 3
	maxProcedureCode = 255
	criticalityCount = 3
	maxProtocolIEs   = 65535
	minFieldOctets   = 4
	fragmentedForm   = 0xC0
)

var (
	ErrExtensionAlternative   = errors.New("protocolie: PDU extension alternative")
	ErrNotProtocolIEContainer = errors.New("protocolie: message body is not a ProtocolIE-Container")
	ErrIndex                  = errors.New("protocolie: field index out of range")
	ErrTooManyFields          = errors.New("protocolie: more than maxProtocolIEs fields")
	ErrCriticality            = errors.New("protocolie: criticality out of range")
)

type Field struct {
	ID          uint16
	Criticality uint8
	Value       []byte
}

type openType struct {
	det     []byte
	content []byte
	raw     []byte
}

type field struct {
	id      uint16
	crit    uint8
	hdr     []byte
	val     openType
	content []byte
}

type Message struct {
	choice        int
	procedureCode uint8
	criticality   uint8

	orig     []byte
	head     []byte
	body     openType
	bodyHdr  []byte
	count    []byte
	fields   []field
	bodyTail []byte
	tail     []byte

	countChanged bool
	dirty        bool
}

func Parse(b []byte, opaque func(procedureCode uint8) bool) (*Message, error) {
	r := per.NewReader(b)
	enc := per.Aligned

	ext, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("protocolie: PDU choice: %w", err)
	}

	if ext {
		return nil, ErrExtensionAlternative
	}

	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, pduChoiceCount-1)
	if err != nil {
		return nil, fmt.Errorf("protocolie: PDU choice: %w", err)
	}

	pc, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProcedureCode)
	if err != nil {
		return nil, fmt.Errorf("protocolie: procedureCode: %w", err)
	}

	crit, err := per.DecodeEnumerated(r, enc, criticalityCount, false)
	if err != nil {
		return nil, fmt.Errorf("protocolie: criticality: %w", err)
	}

	if opaque != nil && opaque(uint8(pc)) {
		return nil, ErrNotProtocolIEContainer
	}

	r.AlignToByte()
	headEnd := r.Offset() / 8

	body, err := readOpenType(r, b)
	if err != nil {
		return nil, fmt.Errorf("protocolie: value: %w", err)
	}

	m := &Message{
		choice:        int(idx),
		procedureCode: uint8(pc),
		criticality:   uint8(crit),
		orig:          b,
		head:          b[:headEnd],
		body:          body,
		tail:          b[r.Offset()/8:],
	}

	if err := m.parseBody(); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Message) parseBody() error {
	content := m.body.content
	r := per.NewReader(content)
	enc := per.Aligned

	if _, err := r.ReadBit(); err != nil {
		return fmt.Errorf("protocolie: message preamble: %w", err)
	}

	r.AlignToByte()
	hdrEnd := r.Offset() / 8

	n, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs)
	if err != nil {
		return fmt.Errorf("protocolie: IE count: %w", err)
	}

	countEnd := r.Offset() / 8

	if n > int64(len(content)-countEnd)/minFieldOctets {
		return fmt.Errorf("protocolie: %d IEs declared in %d octets", n, len(content)-countEnd)
	}

	m.bodyHdr = content[:hdrEnd]
	m.count = content[hdrEnd:countEnd]
	m.fields = make([]field, 0, n)

	for i := range n {
		start := r.Offset() / 8

		id, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs)
		if err != nil {
			return fmt.Errorf("protocolie: IE %d id: %w", i, err)
		}

		crit, err := per.DecodeEnumerated(r, enc, criticalityCount, false)
		if err != nil {
			return fmt.Errorf("protocolie: IE %d criticality: %w", i, err)
		}

		r.AlignToByte()
		hdrEnd := r.Offset() / 8

		val, err := readOpenType(r, content)
		if err != nil {
			return fmt.Errorf("protocolie: IE %d value: %w", i, err)
		}

		m.fields = append(m.fields, field{id: uint16(id), crit: uint8(crit), hdr: content[start:hdrEnd], val: val, content: val.content})
	}

	m.bodyTail = content[r.Offset()/8:]

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

func (m *Message) Choice() int { return m.choice }

func (m *Message) ProcedureCode() uint8 { return m.procedureCode }

func (m *Message) Criticality() uint8 { return m.criticality }

func (m *Message) Len() int { return len(m.fields) }

func (m *Message) Field(i int) (Field, error) {
	if i < 0 || i >= len(m.fields) {
		return Field{}, ErrIndex
	}

	f := m.fields[i]

	return Field{ID: f.id, Criticality: f.crit, Value: append([]byte(nil), f.content...)}, nil
}

func (m *Message) Fields() []Field {
	out := make([]Field, len(m.fields))
	for i := range m.fields {
		out[i], _ = m.Field(i)
	}

	return out
}

func (m *Message) Index(id uint16) int {
	for i, f := range m.fields {
		if f.id == id {
			return i
		}
	}

	return -1
}

func (m *Message) SetValue(i int, value []byte) error {
	if i < 0 || i >= len(m.fields) {
		return ErrIndex
	}

	m.fields[i].content = completeEncoding(value)
	m.dirty = true

	return nil
}

func (m *Message) Insert(i int, f Field) error {
	if i < 0 || i > len(m.fields) {
		return ErrIndex
	}

	if len(m.fields) >= maxProtocolIEs {
		return ErrTooManyFields
	}

	if f.Criticality >= criticalityCount {
		return ErrCriticality
	}

	nf := field{id: f.ID, crit: f.Criticality, content: completeEncoding(f.Value)}

	m.fields = append(m.fields, field{})
	copy(m.fields[i+1:], m.fields[i:])
	m.fields[i] = nf
	m.countChanged = true
	m.dirty = true

	return nil
}

func (m *Message) Remove(i int) error {
	if i < 0 || i >= len(m.fields) {
		return ErrIndex
	}

	m.fields = append(m.fields[:i], m.fields[i+1:]...)
	m.countChanged = true
	m.dirty = true

	return nil
}

func (m *Message) Bytes() ([]byte, error) {
	if !m.dirty {
		return append([]byte(nil), m.orig...), nil
	}

	content := make([]byte, 0, len(m.body.content)+16)
	content = append(content, m.bodyHdr...)

	if m.countChanged {
		w := per.NewWriter()
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, int64(len(m.fields))); err != nil {
			return nil, err
		}

		content = append(content, w.Bytes()...)
	} else {
		content = append(content, m.count...)
	}

	for _, f := range m.fields {
		enc, err := f.encode()
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

	out := make([]byte, 0, len(m.head)+len(body)+len(m.tail))
	out = append(out, m.head...)
	out = append(out, body...)
	out = append(out, m.tail...)

	return out, nil
}

func (f field) encode() ([]byte, error) {
	hdr := f.hdr
	if hdr == nil {
		w := per.NewWriter()
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, int64(f.id)); err != nil {
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

func completeEncoding(value []byte) []byte {
	return append([]byte{}, value...)
}
