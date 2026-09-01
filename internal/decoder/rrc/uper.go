// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rrc

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

const enc = per.Unaligned

type node interface {
	decode(r *per.Reader) (any, error)
}

type field struct {
	name     string
	typ      node
	optional bool
}

type sequence struct {
	fields     []field
	extensible bool
}

type choice struct {
	alternatives []field
	extensible   bool
}

type enumerated struct {
	values     []string
	extValues  []string
	extensible bool
}

type integer struct {
	lb, ub     int64
	extensible bool
}

type bitString struct {
	lb, ub     int64
	extensible bool
}

type octetString struct {
	lb, ub int64
	hasUB  bool
}

type sequenceOf struct {
	lb, ub int64
	elem   node
}

type boolean struct{}

type deferred struct {
	name string
	reg  map[string]node
}

func (d deferred) decode(r *per.Reader) (any, error) {
	n, ok := d.reg[d.name]
	if !ok {
		return nil, fmt.Errorf("rrc: undefined type %q", d.name)
	}

	return n.decode(r)
}

func skipExtensionAdditions(r *per.Reader) error {
	return per.DecodeNormallySmallLength(r, enc, func(count int64) error {
		present := make([]bool, count)

		for i := range present {
			bit, err := r.ReadBit()
			if err != nil {
				return err
			}

			present[i] = bit
		}

		for _, p := range present {
			if !p {
				continue
			}

			if err := per.SkipOpenType(r, enc); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s sequence) decode(r *per.Reader) (any, error) {
	extended := false

	if s.extensible {
		bit, err := r.ReadBit()
		if err != nil {
			return nil, err
		}

		extended = bit
	}

	presence := make(map[string]bool, len(s.fields))

	for _, f := range s.fields {
		if !f.optional {
			continue
		}

		bit, err := r.ReadBit()
		if err != nil {
			return nil, err
		}

		presence[f.name] = bit
	}

	out := make(map[string]any, len(s.fields))

	for _, f := range s.fields {
		if f.optional && !presence[f.name] {
			continue
		}

		v, err := f.typ.decode(r)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.name, err)
		}

		out[f.name] = v
	}

	if extended {
		if err := skipExtensionAdditions(r); err != nil {
			return nil, err
		}
	}

	return out, nil
}

func (c choice) decode(r *per.Reader) (any, error) {
	if c.extensible {
		bit, err := r.ReadBit()
		if err != nil {
			return nil, err
		}

		if bit {
			if _, err := per.DecodeNormallySmall(r, enc); err != nil {
				return nil, err
			}

			if err := per.SkipOpenType(r, enc); err != nil {
				return nil, err
			}

			return map[string]any{}, nil
		}
	}

	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, int64(len(c.alternatives)-1))
	if err != nil {
		return nil, err
	}

	if idx < 0 || idx >= int64(len(c.alternatives)) {
		return nil, fmt.Errorf("rrc: choice index %d out of range", idx)
	}

	alt := c.alternatives[idx]

	v, err := alt.typ.decode(r)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", alt.name, err)
	}

	return map[string]any{alt.name: v}, nil
}

func (e enumerated) decode(r *per.Reader) (any, error) {
	idx, err := per.DecodeEnumerated(r, enc, int64(len(e.values)), e.extensible)
	if err != nil {
		return nil, err
	}

	if idx < int64(len(e.values)) {
		return e.values[idx], nil
	}

	add := idx - int64(len(e.values))
	if add < int64(len(e.extValues)) {
		return e.extValues[add], nil
	}

	return fmt.Sprintf("ext%d", add), nil
}

func (i integer) decode(r *per.Reader) (any, error) {
	return per.DecodeInteger(r, enc, per.Bounds{
		LB: i.lb, UB: i.ub, HasLB: true, HasUB: true, Extensible: i.extensible,
	})
}

func (b bitString) decode(r *per.Reader) (any, error) {
	bits, n, err := per.DecodeBitString(r, enc, b.lb, b.ub, true, true, b.extensible)
	if err != nil {
		return nil, err
	}

	return bitValue{bytes: bits, length: n}, nil
}

type bitValue struct {
	bytes  []byte
	length int
}

func (o octetString) decode(r *per.Reader) (any, error) {
	return per.DecodeOctetString(r, enc, o.lb, o.ub, o.hasUB, o.hasUB, false)
}

func (s sequenceOf) decode(r *per.Reader) (any, error) {
	var items []any

	err := per.DecodeLength(r, enc, s.lb, s.ub, true, func(count int64) error {
		for range count {
			v, err := s.elem.decode(r)
			if err != nil {
				return err
			}

			items = append(items, v)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (boolean) decode(r *per.Reader) (any, error) {
	return per.DecodeBoolean(r, enc)
}

type null struct{}

func (null) decode(*per.Reader) (any, error) {
	return nil, nil
}
