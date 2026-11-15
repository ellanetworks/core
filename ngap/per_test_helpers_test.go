// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

func unmarshalPERValue[T any](b []byte) (T, error) {
	var v T

	err := any(&v).(per.Unmarshaler).UnmarshalPER(per.NewReader(b), per.Aligned)

	return v, err
}

// ieRaw encodes one IE value, as it appears inside a ProtocolIE-Field open type.
func ieRaw(t *testing.T, m per.Marshaler) []byte {
	t.Helper()

	w := per.NewWriter()
	if err := m.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	return perBytes(w)
}

// perBytes applies the padding implicit in a complete PER encoding.
func perBytes(w *per.Writer) []byte {
	w.AlignToByte()

	return w.Bytes()
}

func deref[T any](p *T) T {
	var zero T

	if p == nil {
		return zero
	}

	return *p
}

//nolint:unparam
func encodeIEContainer(w *per.Writer, enc per.Encoding, fields []ieField) error {
	return ap.EncodeIEContainer(profile, w, enc, fields)
}

func typeOfError(d DiagnosticIE) TypeOfError {
	if d.Kind == ap.KindMissing {
		return TypeOfErrorMissing
	}

	return TypeOfErrorNotUnderstood
}

const paddingIEs = 64

func firstIndex(p *RawPDU, id ProtocolIEID) int {
	i, err := p.Index(id)
	if err != nil {
		return -1
	}

	return i
}

func swapTable[M any](table *[]ieSpec[M], rows ...ieSpec[M]) func() {
	saved := *table
	*table = rows

	return func() { *table = saved }
}

func rawRow[M any](id ProtocolIEID, crit Criticality, value []byte) ieSpec[M] {
	return ieSpec[M]{
		ID: id, Presence: presenceOptional, Criticality: crit,
		Decode: func(*M, *per.Reader, per.Encoding) error { return nil },
		Encode: func(*M) (per.Marshaler, bool) { return ap.RawValue(value), true },
	}
}

func tableRow[M any](table []ieSpec[M], id ProtocolIEID, crit Criticality) ieSpec[M] {
	for _, r := range table {
		if r.ID == id {
			r.Criticality = crit

			return r
		}
	}

	panic("no row")
}

func encodeContainerField(w *per.Writer, f ieField) error {
	return ap.EncodeField(profile, w, per.Aligned, f)
}
