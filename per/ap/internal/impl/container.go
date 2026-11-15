// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package impl

import (
	"github.com/ellanetworks/core/per"
)

const minFieldBits = 34

type Field[ID ~uint16] struct {
	ID          ID
	Criticality Criticality
	Value       []byte
}

type FieldValue[ID ~uint16] struct {
	ID          ID
	Criticality Criticality
	Value       per.Marshaler
}

type RawValue []byte

func (c RawValue) MarshalPER(w *per.Writer, _ per.Encoding) error {
	w.WriteBitString(c, len(c)*8)

	return nil
}

func EncodeField[ID ~uint16](p *Profile, w *per.Writer, enc per.Encoding, f FieldValue[ID]) error {
	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, p.MaxProtocolIEs, int64(f.ID)); err != nil {
		return err
	}

	if err := per.EncodeEnumerated(w, enc, criticalityCount, false, int64(f.Criticality)); err != nil {
		return err
	}

	if f.Value == nil {
		return p.errorf("%s has no value", p.ieName(uint16(f.ID)))
	}

	if raw, ok := f.Value.(RawValue); ok {
		return per.EncodeOpenTypeBytes(w, enc, raw)
	}

	if err := per.EncodeOpenType(w, enc, f.Value); err != nil {
		return p.errorf("encode %s: %w", p.ieName(uint16(f.ID)), err)
	}

	return nil
}

func EncodeIEContainer[ID ~uint16](p *Profile, w *per.Writer, enc per.Encoding, fields []FieldValue[ID]) error {
	if int64(len(fields)) > p.MaxProtocolIEs {
		return p.errorf("%d IEs exceed maxProtocolIEs", len(fields))
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, p.MaxProtocolIEs, int64(len(fields))); err != nil {
		return err
	}

	for _, f := range fields {
		if err := EncodeField(p, w, enc, f); err != nil {
			return err
		}
	}

	return nil
}

func EncodeExtensionContainer[ID ~uint16](p *Profile, w *per.Writer, enc per.Encoding, fields []FieldValue[ID]) error {
	if int64(len(fields)) > p.MaxProtocolExtensions {
		return p.errorf("%d extensions exceed maxProtocolExtensions", len(fields))
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 1, p.MaxProtocolExtensions, int64(len(fields))); err != nil {
		return err
	}

	for _, f := range fields {
		if err := EncodeField(p, w, enc, f); err != nil {
			return err
		}
	}

	return nil
}

func DecodeIEContainer[ID ~uint16](p *Profile, r *per.Reader, enc per.Encoding) ([]Field[ID], error) {
	n, err := per.DecodeConstrainedWholeNumber(r, enc, 0, p.MaxProtocolIEs)
	if err != nil {
		return nil, p.errorf("IE container length: %w", err)
	}

	return decodeFields[ID](p, r, enc, n, "IE", false)
}

func decodeExtensionFields[ID ~uint16](p *Profile, r *per.Reader, enc per.Encoding, allowEmpty bool) ([]Field[ID], error) {
	n, err := per.DecodeConstrainedWholeNumber(r, enc, 1, p.MaxProtocolExtensions)
	if err != nil {
		return nil, p.errorf("extension container length: %w", err)
	}

	return decodeFields[ID](p, r, enc, n, "extension", allowEmpty)
}

func decodeFields[ID ~uint16](p *Profile, r *per.Reader, enc per.Encoding, n int64, what string, allowEmpty bool) ([]Field[ID], error) {
	if maxBits := int64(r.Bits()); n > maxBits/minFieldBits {
		return nil, p.errorf("%s container declares %d fields in %d bits", what, n, maxBits)
	}

	fields := make([]Field[ID], 0, n)

	for i := range n {
		f, err := decodeField[ID](p, r, enc, allowEmpty)
		if err != nil {
			return nil, p.errorf("%s %d: %w", what, i, err)
		}

		fields = append(fields, f)
	}

	return fields, nil
}

func decodeField[ID ~uint16](p *Profile, r *per.Reader, enc per.Encoding, allowEmpty bool) (Field[ID], error) {
	id, err := per.DecodeConstrainedWholeNumber(r, enc, 0, p.MaxProtocolIEs)
	if err != nil {
		return Field[ID]{}, err
	}

	crit, err := per.DecodeEnumerated(r, enc, criticalityCount, false)
	if err != nil {
		return Field[ID]{}, err
	}

	val, err := per.DecodeOpenTypeBytes(r, enc)
	if err != nil {
		return Field[ID]{}, err
	}

	if len(val) == 0 && !allowEmpty {
		return Field[ID]{}, p.errorf("%s has an empty open type", p.ieName(uint16(id)))
	}

	return Field[ID]{ID: ID(id), Criticality: Criticality(crit), Value: val}, nil
}

func DecodeValue(b []byte, u per.Unmarshaler) error {
	r := per.NewReader(b)

	if err := u.UnmarshalPER(r, per.Aligned); err != nil {
		return err
	}

	return r.Complete()
}

type SingleContainer[ID ~uint16] struct {
	ID          ID
	Criticality Criticality
	Value       string
}

func DecodeSingleContainerValue[ID ~uint16](p *Profile, r *per.Reader, enc per.Encoding) (SingleContainer[ID], error) {
	f, err := decodeField[ID](p, r, enc, false)
	if err != nil {
		return SingleContainer[ID]{}, err
	}

	return SingleContainer[ID]{ID: f.ID, Criticality: f.Criticality, Value: string(f.Value)}, nil
}

func (c SingleContainer[ID]) Field() FieldValue[ID] {
	return FieldValue[ID]{ID: c.ID, Criticality: c.Criticality, Value: RawValue(c.Value)}
}

type WireCriticality uint8

func Wire(wire, spec Criticality) WireCriticality {
	if wire == spec {
		return 0
	}

	return WireCriticality(wire) + 1
}

func (w WireCriticality) Of(spec Criticality) Criticality {
	if w == 0 {
		return spec
	}

	return Criticality(w - 1)
}

func (f Field[ID]) Encodable() FieldValue[ID] {
	return FieldValue[ID]{ID: f.ID, Criticality: f.Criticality, Value: RawValue(f.Value)}
}
