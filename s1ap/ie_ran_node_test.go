// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

func TestENBIDAllKinds(t *testing.T) {
	cases := []ENBID{
		{Kind: ENBIDMacro, Value: 0xabcde & 0xfffff},       // 20 bits
		{Kind: ENBIDHome, Value: 0x54f6401},                // 28 bits
		{Kind: ENBIDShortMacro, Value: 0x3abcd & 0x3ffff},  // 18 bits, extension
		{Kind: ENBIDLongMacro, Value: 0x1abcde & 0x1fffff}, // 21 bits, extension
	}
	for _, in := range cases {
		w := per.NewWriter()

		if err := in.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("%+v: encode: %v", in, err)
		}

		got, err := unmarshalPERValue[ENBID](perBytes(w))
		if err != nil {
			t.Fatalf("%+v: decode: %v", in, err)
		}

		if got != in {
			t.Fatalf("decoded %+v, want %+v", got, in)
		}
	}
}

// ENB-ID ::= CHOICE { macroENB-ID, homeENB-ID, ..., short-macroENB-ID,
// long-macroENB-ID } defines exactly two extension additions. A later one has
// no width the decoder can know, so it is reported as not comprehended and
// handled on the IE's criticality (TS 36.413 §10.3.1) instead of being read as
// a short-macroENB-ID.
func TestENBIDRejectsUndefinedExtensionAlternative(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		kind ENBIDKind
		want uint32
	}{
		{"short-macroENB-ID", []byte{0x80, 0x03, 0xde, 0xad, 0xbe}, ENBIDShortMacro, 0x37ab6},
		{"long-macroENB-ID", []byte{0x81, 0x03, 0xde, 0xad, 0xbe}, ENBIDLongMacro, 0x1bd5b7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e ENBID
			if err := (&e).UnmarshalPER(per.NewReader(tt.in), per.Aligned); err != nil {
				t.Fatalf("decode: %v", err)
			}

			if e.Kind != tt.kind || e.Value != tt.want {
				t.Fatalf("got kind %d value %#x, want kind %d value %#x", e.Kind, e.Value, tt.kind, tt.want)
			}
		})
	}

	t.Run("undefined extension alternative", func(t *testing.T) {
		var e ENBID

		err := (&e).UnmarshalPER(per.NewReader([]byte{0x82, 0x03, 0xde, 0xad, 0xbe}), per.Aligned)
		if !errors.Is(err, errNotComprehended) {
			t.Fatalf("err = %v, want errNotComprehended", err)
		}
	})
}
