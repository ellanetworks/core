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
	Name      string          `json:"name"`
	Hex       string          `json:"hex"`
	RoundTrip bool            `json:"roundTrip"`
	Value     json.RawMessage `json:"value"`
}

func loadVectors(t *testing.T) []conformanceVector {
	t.Helper()

	raw, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}

	var vectors []conformanceVector
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}

	if len(vectors) == 0 {
		t.Fatal("no vectors")
	}

	return vectors
}

func TestConformanceVectors(t *testing.T) {
	for _, v := range loadVectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			data, err := hex.DecodeString(v.Hex)
			if err != nil {
				t.Fatalf("hex: %v", err)
			}

			var msg LPPMessage
			if err := per.Unmarshal(data, &msg, per.Unaligned); err != nil {
				t.Fatalf("decode: %v", err)
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
	tag      string
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

		f := canonicalField{value: v.Field(i), tag: tag, choice: -1}

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

	if !extensible && len(fields) == 1 && fields[0].value.Kind() != reflect.Pointer && !fields[0].optional {
		return canonicalValue(fields[0].value)
	}

	out := make([]any, len(fields))

	for i, f := range fields {
		if f.optional && f.value.Kind() == reflect.Slice && f.value.IsNil() {
			out[i] = nil
			continue
		}

		out[i] = canonicalValue(f.value)
	}

	return out
}
