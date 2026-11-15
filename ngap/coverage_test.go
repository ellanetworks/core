// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unsafe"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

var sampleValues = map[reflect.Type]any{
	reflect.TypeFor[GNBIDValue]():            GNBIDValue{Value: 0x12345, Bits: 22},
	reflect.TypeFor[TransportLayerAddress](): tlaOf(10, 0, 0, 1),
	reflect.TypeFor[PLMNIdentity]():          PLMNIdentity{0x00, 0xf1, 0x10},
}

func isChoiceStruct(t reflect.Type) bool {
	if t.Kind() != reflect.Struct {
		return false
	}

	alternatives := 0

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		if !strings.Contains(f.Tag.Get("per"), "choice:") {
			return false
		}

		alternatives++
	}

	return alternatives > 0
}

func usable(v reflect.Value) bool {
	if d, ok := v.Addr().Interface().(ap.Diagnosable[ProtocolIEID]); ok && d.Diagnose(&ap.Diagnosis[ProtocolIEID]{}) {
		return false
	}

	m, ok := v.Addr().Interface().(per.Marshaler)
	if !ok {
		return true
	}

	_, err := per.Marshal(m)

	return err == nil
}

func fillSample(t *testing.T, v reflect.Value, depth int) {
	t.Helper()

	if depth > 40 {
		t.Fatalf("sample for %v nests too deep", v.Type())
	}

	if s, ok := sampleValues[v.Type()]; ok {
		v.Set(reflect.ValueOf(s))

		return
	}

	switch v.Kind() {
	case reflect.Pointer:
		p := reflect.New(v.Type().Elem())
		fillSample(t, p.Elem(), depth+1)
		v.Set(p)
	case reflect.Struct:
		if isChoiceStruct(v.Type()) {
			for i := range v.NumField() {
				f := v.Type().Field(i)
				if f.IsExported() && !strings.Contains(f.Tag.Get("per"), "extie") && !strings.Contains(f.Tag.Get("per"), ",ext") {
					fillSample(t, v.Field(i), depth+1)

					return
				}
			}

			t.Fatalf("%v has no root alternative", v.Type())
		}

		for i := range v.NumField() {
			f := v.Type().Field(i)
			tag := f.Tag.Get("per")

			if !f.IsExported() || strings.Contains(tag, "optional") || strings.Contains(tag, "extie") {
				continue
			}

			fillSample(t, v.Field(i), depth+1)
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			for n := 1; n <= 1024; n++ {
				v.Set(reflect.ValueOf(bytes.Repeat([]byte{0x01}, n)).Convert(v.Type()))

				if usable(v) {
					return
				}
			}

			t.Fatalf("no usable %v", v.Type())
		}

		s := reflect.MakeSlice(v.Type(), 1, 1)
		fillSample(t, s.Index(0), depth+1)
		v.Set(s)
	case reflect.String:
		v.SetString("a")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		for _, n := range []int64{0, 1, 2, 3, 4096, 65536} {
			v.SetInt(n)

			if usable(v) {
				return
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		for _, n := range []uint64{0, 1, 2, 3, 4096, 65536} {
			v.SetUint(n)

			if usable(v) {
				return
			}
		}
	}
}

func roundTripValue(t *testing.T, in reflect.Value) reflect.Value {
	t.Helper()

	b, err := per.Marshal(in.Addr().Interface().(per.Marshaler))
	if err != nil {
		t.Fatalf("marshal %v: %v", in.Type(), err)
	}

	out := reflect.New(in.Type())
	if err := per.Unmarshal(b, out.Interface().(per.Unmarshaler)); err != nil {
		t.Fatalf("unmarshal %v: %v", in.Type(), err)
	}

	re, err := per.Marshal(out.Interface().(per.Marshaler))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(re, b) {
		t.Fatalf("%v re-encode = % x, want % x", in.Type(), re, b)
	}

	return out.Elem()
}

func TestExtensionTablesMatchASN1(t *testing.T) {
	for _, c := range extTableCases() {
		t.Run(c.asn1, func(t *testing.T) {
			if want := asn1Rows(t, c.asn1, "ProtocolExtensionContainer"); !slices.Equal(c.rows, want) {
				t.Fatalf("rows\n  %v\nwant\n  %v", c.rows, want)
			}
		})
	}
}

func TestEveryTypedExtensionRoundTrips(t *testing.T) {
	for _, c := range extTableCases() {
		for i := range c.typ.NumField() {
			f := c.typ.Field(i)
			if !strings.Contains(f.Tag.Get("per"), "extie:") {
				continue
			}

			t.Run(c.asn1+"/"+f.Name, func(t *testing.T) {
				in := reflect.New(c.typ).Elem()
				fillSample(t, in, 0)
				fillSample(t, in.Field(i), 0)

				fillConditions(t, in)

				out := roundTripValue(t, in)

				if out.Field(i).IsZero() {
					t.Fatalf("%s decoded empty", f.Name)
				}

				if !out.Addr().MethodByName("Equal").IsValid() {
					t.Fatalf("%v has no Equal", c.typ)
				}

				if !out.MethodByName("Equal").Call([]reflect.Value{in})[0].Bool() {
					t.Fatalf("round trip = %+v, want %+v", out.Interface(), in.Interface())
				}

				if d := ap.Diagnose[ProtocolIEID](out.Addr().Interface().(ap.Diagnosable[ProtocolIEID])); !d.Empty() {
					t.Fatalf("diagnostics = %+v", d)
				}
			})
		}
	}
}

func fillConditions(t *testing.T, v reflect.Value) {
	t.Helper()

	m, ok := v.Addr().Interface().(*M1Configuration)
	if !ok {
		return
	}

	if m.BeamMeasurementsReportConfiguration != nil || m.IncludeBeamMeasurementsIndication != nil {
		m.IncludeBeamMeasurementsIndication = Ptr(IncludeBeamMeasurementsIndicationTrue)
		fillSample(t, reflect.ValueOf(&m.BeamMeasurementsReportConfiguration).Elem(), 0)
	}
}

func TestEveryMessageIERoundTrips(t *testing.T) {
	for _, c := range messageCases() {
		t.Run(c.asn1, func(t *testing.T) {
			in := reflect.New(c.typ)

			for i := range c.typ.NumField() {
				if f := c.typ.Field(i); f.IsExported() && !f.Anonymous {
					fillSample(t, in.Elem().Field(i), 0)
				}
			}

			if m, ok := in.Interface().(*HandoverCommand); ok {
				m.HandoverType = HandoverTypeFiveGSToEPS
			}

			b, err := in.Interface().(interface{ Marshal() ([]byte, error) }).Marshal()
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			pdu, err := Unmarshal(b)
			if err != nil {
				t.Fatal(err)
			}

			out, err := c.parse(pdu.value())
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if !equalByMethod(out, in.Interface()) {
				t.Fatalf("decoded\n  %+v\nwant\n  %+v", out, in.Interface())
			}

			re, err := out.(interface{ Marshal() ([]byte, error) }).Marshal()
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(re, b) {
				t.Fatalf("re-encode = % x, want % x", re, b)
			}
		})
	}
}

func preservedOf(t *testing.T, v reflect.Value) *extensionMeta {
	t.Helper()

	f := v.FieldByName("iEExtensions")
	if !f.IsValid() {
		t.Fatalf("%v has no iEExtensions", v.Type())
	}

	return (*extensionMeta)(unsafe.Pointer(f.UnsafeAddr()))
}

func typedExtensionFields(typ reflect.Type) []int {
	var out []int

	for i := range typ.NumField() {
		if strings.Contains(typ.Field(i).Tag.Get("per"), "extie:") && !strings.Contains(typ.Field(i).Tag.Get("per"), "conditional") {
			out = append(out, i)
		}
	}

	return out
}

func extensionID(t *testing.T, f reflect.StructField) (ProtocolIEID, Criticality) {
	t.Helper()

	var (
		id   ProtocolIEID
		crit Criticality
	)

	for _, opt := range strings.Split(f.Tag.Get("per"), ",") {
		switch {
		case strings.HasPrefix(opt, "extie:"):
			name := strings.TrimPrefix(opt, "extie:")
			for i, n := range protocolIENames {
				if "ID"+strings.NewReplacer("-", "", "_", "").Replace(n) == name {
					id = i
				}
			}
		case opt == "crit:reject":
			crit = CriticalityReject
		case opt == "crit:notify":
			crit = CriticalityNotify
		case opt == "crit:ignore":
			crit = CriticalityIgnore
		}
	}

	if id == 0 && !strings.Contains(f.Tag.Get("per"), "extie:IDAllowedNSSAI") {
		t.Fatalf("%s: no IE id for %q", f.Name, f.Tag.Get("per"))
	}

	return id, crit
}

func encodeMarshaler(t *testing.T, v reflect.Value) []byte {
	t.Helper()

	m, ok := v.Interface().(per.Marshaler)
	if !ok {
		m = v.Addr().Interface().(per.Marshaler)
	}

	b, err := per.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func TestExtensionContainersAreLossless(t *testing.T) {
	for _, c := range extTableCases() {
		fields := typedExtensionFields(c.typ)
		if len(fields) == 0 {
			continue
		}

		t.Run(c.asn1, func(t *testing.T) {
			sample := reflect.New(c.typ).Elem()
			fillSample(t, sample, 0)

			var raws []rawIE

			for _, i := range fields {
				fv := reflect.New(c.typ.Field(i).Type).Elem()
				fillSample(t, fv, 0)

				id, crit := extensionID(t, c.typ.Field(i))
				raws = append(raws, rawIE{ID: id, Criticality: crit, Value: encodeMarshaler(t, fv)})
			}

			cases := map[string][]rawIE{
				"unknown ignore and reject": {
					{ID: 40000, Criticality: CriticalityIgnore, Value: []byte{0x01}},
					{ID: 40001, Criticality: CriticalityReject, Value: []byte{0x02, 0x03}},
				},
				"known out of order": slices.Concat(raws[len(raws)/2:], raws[:len(raws)/2]),
				"duplicate":          {raws[0], raws[0]},
			}

			for name, extra := range cases {
				in := reflect.New(c.typ).Elem()
				in.Set(sample)

				for _, r := range extra {
					preservedOf(t, in).Append(r)
				}

				wire := encodeMarshaler(t, in)

				out := reflect.New(c.typ)
				if err := per.Unmarshal(wire, out.Interface().(per.Unmarshaler)); err != nil {
					t.Fatalf("%s: decode: %v", name, err)
				}

				if re := encodeMarshaler(t, out.Elem()); !bytes.Equal(re, wire) {
					t.Fatalf("%s: re-encode = % x, want % x", name, re, wire)
				}

				d := ap.Diagnose[ProtocolIEID](out.Interface().(ap.Diagnosable[ProtocolIEID]))

				switch name {
				case "unknown ignore and reject":
					if len(d.IEs) != 2 || d.IEs[1].ID != 40001 || d.IEs[1].Criticality != CriticalityReject {
						t.Errorf("%s: diagnostics = %+v", name, d.IEs)
					}

					if d.Check() == nil {
						t.Errorf("%s: Check accepted a reject-criticality unknown extension", name)
					}
				case "duplicate":
					if d.Empty() {
						t.Errorf("%s: a duplicate extension went unreported", name)
					}
				}
			}
		})
	}
}
