// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
)

type conformanceVector struct {
	Name      string                     `json:"name"`
	Hex       string                     `json:"hex"`
	RoundTrip bool                       `json:"roundTrip"`
	Value     json.RawMessage            `json:"value"`
	Ext       map[string]json.RawMessage `json:"ext"`
}

type conformanceVectors struct {
	Pycrate string              `json:"pycrate"`
	Vectors []conformanceVector `json:"vectors"`
}

func loadVectors(t *testing.T) []conformanceVector {
	t.Helper()

	raw, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}

	var file conformanceVectors
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}

	if len(file.Vectors) == 0 {
		t.Fatal("no vectors")
	}

	return file.Vectors
}

func TestConformanceVectors(t *testing.T) {
	for _, v := range loadVectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			data, err := hex.DecodeString(v.Hex)
			if err != nil {
				t.Fatalf("hex: %v", err)
			}

			var msg LPPMessage

			r := per.NewReader(data)
			if err := msg.UnmarshalPER(r, per.Unaligned); err != nil {
				t.Fatalf("decode: %v", err)
			}

			if r.Bits() >= 8 {
				t.Fatalf("decode left %d bits unread", r.Bits())
			}

			dec := json.NewDecoder(bytes.NewReader(v.Value))
			dec.UseNumber()

			var want any
			if err := dec.Decode(&want); err != nil {
				t.Fatalf("expected value: %v", err)
			}

			got := canonicalValue(reflect.ValueOf(msg))
			if !reflect.DeepEqual(got, want) {
				gotJSON, _ := json.Marshal(got)
				t.Fatalf("decoded value mismatch\n got: %s\nwant: %s", gotJSON, v.Value)
			}

			checkModelledExtensions(t, msg, v.Ext)

			if !v.RoundTrip {
				return
			}

			out, err := per.Marshal(&msg, per.Unaligned)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}

			if !bytes.Equal(out, data) {
				t.Fatalf("re-encoding mismatch\n got: %x\nwant: %x", out, data)
			}
		})
	}
}

var nullType = reflect.TypeFor[per.Null]()

func canonicalValue(v reflect.Value) any {
	if v.Type() == nullType {
		return "NULL"
	}

	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return nil
		}

		return canonicalValue(v.Elem())
	case reflect.Bool:
		return v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return json.Number(strconv.FormatInt(v.Int(), 10))
	case reflect.String:
		return v.String()
	case reflect.Slice:
		return canonicalSlice(v)
	case reflect.Struct:
		return canonicalStruct(v)
	default:
		panic("unsupported kind " + v.Kind().String())
	}
}

func canonicalSlice(v reflect.Value) any {
	switch v.Type().Elem().Kind() {
	case reflect.Bool:
		var sb strings.Builder

		for i := range v.Len() {
			if v.Index(i).Bool() {
				sb.WriteByte('1')
			} else {
				sb.WriteByte('0')
			}
		}

		return sb.String()
	case reflect.Uint8:
		return hex.EncodeToString(v.Bytes())
	}

	out := make([]any, v.Len())
	for i := range v.Len() {
		out[i] = canonicalValue(v.Index(i))
	}

	return out
}

type canonicalField struct {
	value    reflect.Value
	choice   int
	optional bool
}

func canonicalStruct(v reflect.Value) any {
	var (
		fields     []canonicalField
		extensible bool
		isChoice   = true
	)

	for i := range v.NumField() {
		sf := v.Type().Field(i)
		tag := sf.Tag.Get("per")

		if tag == "extseq" {
			extensible = true
			continue
		}

		if isExtensionAddition(sf) {
			continue
		}

		f := canonicalField{value: v.Field(i), choice: -1, optional: v.Field(i).Kind() == reflect.Pointer}

		for opt := range strings.SplitSeq(tag, ",") {
			switch {
			case strings.HasPrefix(opt, "choice:"):
				f.choice, _ = strconv.Atoi(strings.TrimPrefix(opt, "choice:"))
			case opt == "optional":
				f.optional = true
			}
		}

		if f.choice < 0 {
			isChoice = false
		}

		fields = append(fields, f)
	}

	if isChoice && len(fields) > 0 {
		for _, f := range fields {
			if !f.value.IsNil() {
				return []any{json.Number(strconv.Itoa(f.choice)), canonicalValue(f.value)}
			}
		}

		return []any{"ext"}
	}

	if !extensible && len(fields) == 1 && !fields[0].optional {
		return canonicalValue(fields[0].value)
	}

	out := make([]any, len(fields))

	for i, f := range fields {
		if f.optional && f.value.Kind() != reflect.Pointer && f.value.IsNil() {
			out[i] = nil
			continue
		}

		out[i] = canonicalValue(f.value)
	}

	return out
}

func checkModelledExtensions(t *testing.T, msg LPPMessage, want map[string]json.RawMessage) {
	t.Helper()

	got := map[string]any{}
	collectModelledExtensions(reflect.ValueOf(msg), got)

	wantNorm := map[string]any{}

	for name, raw := range want {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()

		var value any
		if err := dec.Decode(&value); err != nil {
			t.Fatalf("expected extension %s: %v", name, err)
		}

		wantNorm[normalizeName(name)] = value
	}

	if !reflect.DeepEqual(got, wantNorm) {
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(wantNorm)
		t.Fatalf("decoded extension additions mismatch\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

func collectModelledExtensions(v reflect.Value, out map[string]any) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			collectModelledExtensions(v.Elem(), out)
		}
	case reflect.Slice:
		for i := range v.Len() {
			collectModelledExtensions(v.Index(i), out)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			sf := v.Type().Field(i)
			if !sf.IsExported() {
				continue
			}

			field := v.Field(i)

			if isExtensionAddition(sf) && field.Kind() == reflect.Pointer && !field.IsNil() && field.Type().Elem() != unmodelledType {
				group := field.Elem()
				for j := range group.NumField() {
					member := group.Field(j)
					if member.IsNil() || member.Type().Elem() == unmodelledType {
						continue
					}

					out[normalizeName(group.Type().Field(j).Name)] = canonicalValue(member)
				}
			}

			collectModelledExtensions(field, out)
		}
	}
}
