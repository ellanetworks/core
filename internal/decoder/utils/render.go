// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package utils

import (
	"bytes"
	"cmp"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"unicode"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

type Opaque struct {
	Hex     string `json:"hex"`
	Bits    int    `json:"bits,omitempty"`
	Decoded any    `json:"decoded,omitempty"`
	Error   string `json:"error,omitempty"`
}

func Octets(b []byte) Opaque {
	return Opaque{Hex: hex.EncodeToString(b)}
}

func BitString(b []byte, bits int) Opaque {
	return Opaque{Hex: hex.EncodeToString(b), Bits: bits}
}

func BitStringUint(v uint64, bits int) Opaque {
	b := make([]byte, (bits+7)/8)

	for i := range bits {
		if v&(1<<uint(bits-1-i)) != 0 {
			b[i/8] |= 1 << uint(7-i%8)
		}
	}

	return BitString(b, bits)
}

type ExtensionAlternative struct {
	Index int64  `json:"index"`
	Hex   string `json:"hex"`
}

type Member struct {
	Key   string
	Value any
}

type Object []Member

func (o Object) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteByte('{')

	for i, m := range o {
		if i > 0 {
			buf.WriteByte(',')
		}

		k, err := json.Marshal(m.Key)
		if err != nil {
			return nil, err
		}

		v, err := json.Marshal(m.Value)
		if err != nil {
			return nil, err
		}

		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(v)
	}

	buf.WriteByte('}')

	return buf.Bytes(), nil
}

type Leaf func(v any) (any, bool)

func Present(v any) bool {
	return present(reflect.ValueOf(v))
}

func present(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Invalid:
		return false
	case reflect.Pointer, reflect.Interface, reflect.Map:
		return !v.IsNil()
	case reflect.Slice:
		return v.Len() > 0
	default:
		return true
	}
}

func Render(v any, leaf Leaf) any {
	return render(reflect.ValueOf(v), leaf)
}

func render(v reflect.Value, leaf Leaf) any {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}

		v = v.Elem()
	}

	if !v.IsValid() {
		return nil
	}

	if v.CanInterface() {
		if out, ok := builtin(v, leaf); ok {
			return out
		}
	}

	switch v.Kind() {
	case reflect.Bool:
		return v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	case reflect.String:
		return v.String()
	case reflect.Slice, reflect.Array:
		if v.Type().Elem() == reflect.TypeFor[byte]() {
			b := make([]byte, v.Len())
			for i := range b {
				b[i] = byte(v.Index(i).Uint())
			}

			return Octets(b)
		}

		out := make([]any, 0, v.Len())
		for i := range v.Len() {
			out = append(out, render(v.Index(i), leaf))
		}

		return out
	case reflect.Struct:
		return renderStruct(v, leaf)
	default:
		return nil
	}
}

func builtin(v reflect.Value, leaf Leaf) (any, bool) {
	x := v.Interface()

	if leaf != nil {
		if out, ok := leaf(x); ok {
			return out, true
		}
	}

	switch t := x.(type) {
	case ap.TransportLayerAddress:
		return TransportLayerAddressText(t), true
	case per.ExtensionAlternative:
		return ExtensionAlternative{Index: t.Index, Hex: hex.EncodeToString([]byte(t.Value))}, true
	case per.Null:
		return true, true
	case interface {
		Bits() int
		Bytes() []byte
	}:
		return BitString(t.Bytes(), t.Bits()), true
	case interface{ Name() string }:
		if v.CanInt() {
			return NamedEnum(v.Int(), t.Name()), true
		}

		if v.CanUint() {
			return NamedEnum(v.Uint(), t.Name()), true
		}
	}

	return nil, false
}

func renderStruct(v reflect.Value, leaf Leaf) Object {
	out := Object{}
	t := v.Type()

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		fv := v.Field(i)
		if !present(fv) {
			continue
		}

		out = append(out, Member{Key: FieldName(f.Name), Value: render(fv, leaf)})
	}

	return out
}

func TransportLayerAddressText(a ap.TransportLayerAddress) string {
	ipv4, ipv6 := a.IPv4(), a.IPv6()

	switch {
	case ipv4.IsValid() && ipv6.IsValid():
		return ipv4.String() + "+" + ipv6.String()
	case ipv4.IsValid():
		return ipv4.String()
	case ipv6.IsValid():
		return ipv6.String()
	default:
		return hex.EncodeToString(a.Bytes())
	}
}

var goAliases = map[string]string{"choiceextension": "choiceextensions"}

func FieldName(goName string) string {
	key := strings.ToLower(goName)
	if alias, ok := goAliases[key]; ok {
		key = alias
	}

	if name, ok := asn1Names[key]; ok {
		return name
	}

	return SnakeCase(goName)
}

func SnakeCase(name string) string {
	name = strings.NewReplacer("QoS", "Qos", "AIoT", "Aiot").Replace(name)
	r := []rune(name)

	var words []string

	for i := 0; i < len(r); {
		j := i + 1

		switch {
		case unicode.IsUpper(r[i]):
			for j < len(r) && (unicode.IsUpper(r[j]) || unicode.IsDigit(r[j])) {
				j++
			}

			k := j
			for k < len(r) && (unicode.IsLower(r[k]) || unicode.IsDigit(r[k])) {
				k++
			}

			switch {
			case j-i == 1 || k == j:
				j = k
			case j-i == 2 || connectives[string(r[j:k])]:
				words = append(words, string(r[i:j]))
				i, j = j, k
			default:
				words = append(words, string(r[i:j-1]))
				i, j = j-1, k
			}
		default:
			for j < len(r) && !unicode.IsUpper(r[j]) {
				j++
			}
		}

		words = append(words, string(r[i:j]))
		i = j
	}

	return strings.ToLower(strings.Join(words, "_"))
}

var connectives = map[string]bool{"to": true, "for": true, "of": true, "in": true, "and": true, "or": true, "per": true}

func WireOrder[T any](wire []int64, items []T, id func(T) int64) []T {
	positions := map[int64][]int{}

	for i, w := range wire {
		positions[w] = append(positions[w], i)
	}

	type ranked struct {
		rank int
		item T
	}

	out := make([]ranked, len(items))

	for i, item := range items {
		out[i] = ranked{rank: len(wire) + i, item: item}

		if p := positions[id(item)]; len(p) > 0 {
			out[i].rank = p[0]
			positions[id(item)] = p[1:]
		}
	}

	slices.SortStableFunc(out, func(a, b ranked) int { return cmp.Compare(a.rank, b.rank) })

	for i, r := range out {
		items[i] = r.item
	}

	return items
}
