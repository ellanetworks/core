// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

var sampleValues = map[reflect.Type]any{
	reflect.TypeFor[TransportLayerAddress]():           goldTLA(),
	reflect.TypeFor[PLMNIdentity]():                    goldPLMN(),
	reflect.TypeFor[EngNBID]():                         EngNBID{BitStringFrom([]byte{0x12, 0x34, 0x54}, 22)},
	reflect.TypeFor[NRencryptionAlgorithms]():          NRencryptionAlgorithms{BitStringFrom([]byte{0xe0, 0x00}, 16)},
	reflect.TypeFor[NRintegrityProtectionAlgorithms](): NRintegrityProtectionAlgorithms{BitStringFrom([]byte{0xe0, 0x00}, 16)},
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

		tag := f.Tag.Get("per")
		if !strings.Contains(tag, "choice:") && !strings.Contains(tag, "extalt") {
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
				tag := v.Type().Field(i).Tag.Get("per")
				if v.Type().Field(i).IsExported() && strings.Contains(tag, "choice:") && !strings.Contains(tag, "extie") && !strings.Contains(tag, ",ext") {
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
		if v.Type().Elem() == reflect.TypeFor[byte]() {
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
	case reflect.Array:
		for i := range v.Len() {
			fillSample(t, v.Index(i), depth+1)
		}
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

func sampleMessage(t *testing.T, typ reflect.Type) reflect.Value {
	t.Helper()

	in := reflect.New(typ)

	for i := range typ.NumField() {
		if f := typ.Field(i); f.IsExported() && !f.Anonymous {
			fillSample(t, in.Elem().Field(i), 0)
		}
	}

	switch m := in.Interface().(type) {
	case *HandoverCommand:
		m.HandoverType = HandoverTypeLTEtoUTRAN
	case *HandoverRequest:
		m.HandoverType = HandoverTypeUTRANtoLTE
	case *HandoverRequired:
		m.HandoverType = HandoverTypeLTEtoGERAN
	case *InitialContextSetupRequest:
		m.CSFallbackIndicator = Ptr(CSFallbackIndicatorCSFallbackHighPriority)
	case *UEContextModificationRequest:
		m.CSFallbackIndicator = Ptr(CSFallbackIndicatorCSFallbackHighPriority)
	case *OverloadStart:
		m.TrafficLoadReductionIndication = Ptr(uint8(1))
	case *WriteReplaceWarningRequest:
		m.ExtendedRepetitionPeriod = Ptr(uint32(4096))
	}

	return in
}

func TestEveryMessageIERoundTrips(t *testing.T) {
	for _, c := range messageCases() {
		t.Run(c.asn1, func(t *testing.T) {
			in := sampleMessage(t, c.typ)

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
