// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package impl

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/per"
)

type ListItem[ID ~uint16] struct {
	crit   WireCriticality
	before string
	after  string
}

type uncomprehendedValueError[ID ~uint16] struct {
	inner []Field[ID]
}

func (e *uncomprehendedValueError[ID]) Error() string {
	return "ap: single-container list holds no item of its own IE"
}

func ListItemCriticality[ID ~uint16](wire Criticality) ListItem[ID] {
	return ListItem[ID]{crit: WireCriticality(wire) + 1}
}

func (it *ListItem[ID]) listItem() *ListItem[ID] { return it }

type listItem[ID ~uint16] interface {
	listItem() *ListItem[ID]
}

func (it ListItem[ID]) ForeignIEs() []Field[ID] {
	return append(decodeForeign[ID](it.before), decodeForeign[ID](it.after)...)
}

func appendForeign[ID ~uint16](b []byte, f Field[ID]) []byte {
	b = binary.BigEndian.AppendUint16(b, uint16(f.ID))
	b = append(b, byte(f.Criticality))
	b = binary.AppendUvarint(b, uint64(len(f.Value)))

	return append(b, f.Value...)
}

func decodeForeign[ID ~uint16](s string) []Field[ID] {
	var out []Field[ID]

	for s != "" {
		var f Field[ID]

		f, s = nextPreserved[ID](s)
		out = append(out, f)
	}

	return out
}

func asListItem[ID ~uint16](v any) (*ListItem[ID], error) {
	h, ok := v.(listItem[ID])
	if !ok {
		return nil, fmt.Errorf("ap: %T does not embed ListItem", v)
	}

	return h.listItem(), nil
}

func EncodeItemList[ID ~uint16, T any](p *Profile, w *per.Writer, enc per.Encoding, ub int64, id ID, crit Criticality, items []T) error {
	var fields []FieldValue[ID]

	for i := range items {
		it, err := asListItem[ID](&items[i])
		if err != nil {
			return err
		}

		for _, f := range decodeForeign[ID](it.before) {
			fields = append(fields, f.Encodable())
		}

		m, ok := any(&items[i]).(per.Marshaler)
		if !ok {
			return fmt.Errorf("ap: %T does not implement per.Marshaler", items[i])
		}

		fields = append(fields, FieldValue[ID]{ID: id, Criticality: it.crit.Of(crit), Value: m})

		for _, f := range decodeForeign[ID](it.after) {
			fields = append(fields, f.Encodable())
		}
	}

	if len(fields) < 1 || int64(len(fields)) > ub {
		return p.errorf("single-container list length %d outside [1, %d]", len(fields), ub)
	}

	off := 0

	return per.EncodeLength(w, enc, 1, ub, true, int64(len(fields)), func(count int64) error {
		end := off + int(count)
		for _, f := range fields[off:end] {
			if err := EncodeField(p, w, enc, f); err != nil {
				return err
			}
		}

		off = end

		return nil
	})
}

func DecodeItemList[ID ~uint16, T any](p *Profile, r *per.Reader, enc per.Encoding, ub int64, id ID, crit Criticality) ([]T, error) {
	var (
		items   []T
		pending []byte
		all     []Field[ID]
	)

	err := per.DecodeLength(r, enc, 1, ub, true, func(count int64) error {
		for range count {
			f, err := decodeField[ID](p, r, enc, false)
			if err != nil {
				return err
			}

			all = append(all, f)

			if f.ID != id {
				pending = appendForeign(pending, f)

				continue
			}

			var item T

			u, ok := any(&item).(per.Unmarshaler)
			if !ok {
				return fmt.Errorf("ap: %T does not implement per.Unmarshaler", item)
			}

			if err := DecodeValue(f.Value, u); err != nil {
				if _, raw := asUncomprehended[ID](err); raw {
					pending = appendForeign(pending, f)

					continue
				}

				return err
			}

			it, err := asListItem[ID](&item)
			if err != nil {
				return err
			}

			it.crit = Wire(f.Criticality, crit)
			it.before = string(pending)
			pending = nil

			items = append(items, item)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, &uncomprehendedValueError[ID]{inner: all}
	}

	if len(pending) > 0 {
		it, err := asListItem[ID](&items[len(items)-1])
		if err != nil {
			return nil, err
		}

		it.after = string(pending)
	}

	return items, nil
}

func asUncomprehended[ID ~uint16](err error) ([]Field[ID], bool) {
	var e *uncomprehendedValueError[ID]
	if errors.As(err, &e) {
		return e.inner, true
	}

	return nil, false
}

func DiagnoseItemList[ID ~uint16, T any](d *Diagnosis[ID], id ID, crit Criticality, items *[]T) bool {
	list := *items
	kept := list[:0]
	comprehended := 0
	pruned := false

	var pending []byte

	for i := range list {
		it, err := asListItem[ID](&list[i])
		if err != nil {
			return false
		}

		for _, f := range it.ForeignIEs() {
			diagnoseRawIE[ID, T](d, id, f)
		}

		wire := it.crit.Of(crit)
		notComprehended := d.IE(id, wire, &list[i])

		if !notComprehended {
			comprehended++
		}

		if notComprehended && d.prune {
			if raw, ok := marshalItem(&list[i]); ok {
				pending = append(pending, it.before...)
				pending = appendForeign(pending, Field[ID]{ID: id, Criticality: wire, Value: raw})
				pending = append(pending, it.after...)
				pruned = true

				continue
			}
		}

		if len(pending) > 0 {
			it.before = string(pending) + it.before
			pending = nil
		}

		kept = append(kept, list[i])
	}

	if !pruned || len(kept) == 0 {
		return len(list) > 0 && comprehended == 0
	}

	if len(pending) > 0 {
		it, _ := asListItem[ID](&kept[len(kept)-1])
		it.after += string(pending)
	}

	*items = kept

	return false
}

func diagnoseRawIE[ID ~uint16, T any](d *Diagnosis[ID], id ID, f Field[ID]) {
	if f.ID == id {
		var item T

		if u, ok := any(&item).(per.Unmarshaler); ok {
			if inner, raw := asUncomprehended[ID](DecodeValue(f.Value, u)); raw {
				d.uncomprehended(f.ID, f.Criticality, inner)

				return
			}
		}
	}

	d.Unknown(f.ID, f.Criticality)
}

func marshalItem(v any) ([]byte, bool) {
	m, ok := v.(per.Marshaler)
	if !ok {
		return nil, false
	}

	b, err := per.Marshal(m, per.Aligned)

	return b, err == nil
}
