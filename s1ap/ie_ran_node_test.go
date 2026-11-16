// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

func TestENBIDAllKinds(t *testing.T) {
	cases := []ENBID{
		{MacroENBID: Ptr(MacroENBID(0xabcde & 0xfffff))},
		{HomeENBID: Ptr(HomeENBID(0x54f6401))},
		{ShortMacroENBID: Ptr(ShortMacroENBID(0x3abcd & 0x3ffff))},
		{LongMacroENBID: Ptr(LongMacroENBID(0x1abcde & 0x1fffff))},
	}
	for _, in := range cases {
		w := per.NewWriter()

		if err := in.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("%v: encode: %v", in, err)
		}

		got, err := unmarshalPERValue[ENBID](perBytes(w))
		if err != nil {
			t.Fatalf("%v: decode: %v", in, err)
		}

		if !got.Equal(in) {
			t.Fatalf("decoded %v, want %v", got, in)
		}
	}
}

// ENB-ID ::= CHOICE { macroENB-ID, homeENB-ID, ..., short-macroENB-ID,
// long-macroENB-ID } defines exactly two extension additions. A later one is
// kept verbatim and reported as not comprehended (TS 36.413 §10.3.1).
func TestENBIDKeepsUndefinedExtensionAlternative(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want ENBID
	}{
		{"short-macroENB-ID", []byte{0x80, 0x03, 0xde, 0xad, 0xbe}, ENBID{ShortMacroENBID: Ptr(ShortMacroENBID(0x37ab6))}},
		{"long-macroENB-ID", []byte{0x81, 0x03, 0xde, 0xad, 0xbe}, ENBID{LongMacroENBID: Ptr(LongMacroENBID(0x1bd5b7))}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e ENBID
			if err := (&e).UnmarshalPER(per.NewReader(tt.in), per.Aligned); err != nil {
				t.Fatalf("decode: %v", err)
			}

			if !e.Equal(tt.want) {
				t.Fatalf("got %v, want %v", e, tt.want)
			}
		})
	}

	t.Run("undefined extension alternative", func(t *testing.T) {
		in := []byte{0x82, 0x03, 0xde, 0xad, 0xbe}

		var e ENBID
		if err := (&e).UnmarshalPER(per.NewReader(in), per.Aligned); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if e.Ext == nil || e.Ext.Index != 2 || e.Ext.Value != "\xde\xad\xbe" {
			t.Fatalf("Ext = %+v", e.Ext)
		}

		if !e.Diagnose(&ap.Diagnosis[ProtocolIEID]{}) {
			t.Fatal("an undefined ENB-ID alternative must not be comprehended")
		}

		if out := ieRaw(t, &e); !bytes.Equal(out, in) {
			t.Fatalf("re-encoded % x, want % x", out, in)
		}
	})
}
