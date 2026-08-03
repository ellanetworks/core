// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
)

func TestGlobalRANNodeIDRoundTrip(t *testing.T) {
	for _, g := range []GlobalRANNodeID{
		{Kind: RANNodeIDGNB, PLMNIdentity: goldPLMN(), Value: 0x000102, Bits: 24},
		// gNB-ID is SIZE(22..32), so both ends of the range must survive.
		{Kind: RANNodeIDGNB, PLMNIdentity: goldPLMN(), Value: 0x3fffff, Bits: 22},
		{Kind: RANNodeIDGNB, PLMNIdentity: goldPLMN(), Value: 0xffffffff, Bits: 32},
		{Kind: RANNodeIDMacroNgENB, PLMNIdentity: goldPLMN(), Value: 0xabcde, Bits: 20},
		{Kind: RANNodeIDShortMacroNgENB, PLMNIdentity: goldPLMN(), Value: 0x3ffff, Bits: 18},
		{Kind: RANNodeIDLongMacroNgENB, PLMNIdentity: goldPLMN(), Value: 0x1abcde, Bits: 21},
		{Kind: RANNodeIDN3IWF, PLMNIdentity: goldPLMN(), Value: 0xbeef, Bits: 16},
	} {
		w := per.NewWriter()
		if err := g.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("%+v: marshal: %v", g, err)
		}

		got, err := unmarshalPERValue[GlobalRANNodeID](perBytes(w))
		if err != nil {
			t.Fatalf("%+v: unmarshal: %v", g, err)
		}

		if got != g {
			t.Errorf("round trip = %+v, want %+v", got, g)
		}
	}
}

// Each kind but the gNB has exactly one legal BIT STRING length. Encoding
// rejects any other rather than padding or truncating to fit.
func TestGlobalRANNodeIDRejectsWrongBitLength(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   GlobalRANNodeID
	}{
		{"gNB below SIZE(22..32)", GlobalRANNodeID{Kind: RANNodeIDGNB, Bits: 21}},
		{"gNB above SIZE(22..32)", GlobalRANNodeID{Kind: RANNodeIDGNB, Bits: 33}},
		{"macroNgENB not 20", GlobalRANNodeID{Kind: RANNodeIDMacroNgENB, Bits: 21}},
		{"shortMacroNgENB not 18", GlobalRANNodeID{Kind: RANNodeIDShortMacroNgENB, Bits: 20}},
		{"longMacroNgENB not 21", GlobalRANNodeID{Kind: RANNodeIDLongMacroNgENB, Bits: 20}},
		{"n3IWF not 16", GlobalRANNodeID{Kind: RANNodeIDN3IWF, Bits: 32}},
		// An unset Bits is the zero value, not a request for the default.
		{"unset bit length", GlobalRANNodeID{Kind: RANNodeIDMacroNgENB}},
		{"unknown kind", GlobalRANNodeID{Kind: RANNodeIDKind(9), Bits: 20}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := tc.in.MarshalPER(w, per.Aligned); err == nil {
				t.Fatalf("marshal accepted %+v", tc.in)
			}
		})
	}
}

// TS 38.413 §9.3.1.5 closes GlobalRANNodeID and each nested node-id CHOICE
// with a choice-Extensions alternative. Selecting one names an alternative
// this version does not model, which must be an error and not a zero value.
func TestGlobalRANNodeIDChoiceExtensionIsRejected(t *testing.T) {
	tests := []struct {
		name  string
		build func(*per.Writer) error
		want  string
	}{
		{
			"outer GlobalRANNodeID choice-Extensions",
			func(w *per.Writer) error {
				return per.EncodeConstrainedWholeNumber(w, per.Aligned, 0,
					globalRANNodeIDAlternatives-1, globalRANNodeIDChoiceExtensions)
			},
			"unsupported GlobalRANNodeID alternative",
		},
		{
			"nested GNB-ID choice-Extensions",
			func(w *per.Writer) error {
				enc := per.Aligned
				if err := per.EncodeConstrainedWholeNumber(w, enc, 0, globalRANNodeIDAlternatives-1, globalGNBID); err != nil {
					return err
				}

				w.WriteBit(false)
				w.WriteBit(false)

				if err := goldPLMN().MarshalPER(w, enc); err != nil {
					return err
				}

				return per.EncodeConstrainedWholeNumber(w, enc, 0, int64(nodeIDAlternatives[globalGNBID]-1), 1)
			},
			"unsupported GNB-ID alternative",
		},
		{
			"nested NgENB-ID choice-Extensions",
			func(w *per.Writer) error {
				enc := per.Aligned
				if err := per.EncodeConstrainedWholeNumber(w, enc, 0, globalRANNodeIDAlternatives-1, globalNgENBID); err != nil {
					return err
				}

				w.WriteBit(false)
				w.WriteBit(false)

				if err := goldPLMN().MarshalPER(w, enc); err != nil {
					return err
				}

				return per.EncodeConstrainedWholeNumber(w, enc, 0, int64(nodeIDAlternatives[globalNgENBID]-1), 3)
			},
			"unsupported NgENB-ID alternative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := tt.build(w); err != nil {
				t.Fatal(err)
			}

			// The alternative's own ProtocolIE-SingleContainer payload.
			if err := encodeContainerField(w, per.Aligned, ieField{
				id:   idGlobalRANNodeID,
				crit: CriticalityReject,
				raw:  []byte{0x00},
			}); err != nil {
				t.Fatal(err)
			}

			got, err := unmarshalPERValue[GlobalRANNodeID](perBytes(w))
			if err == nil {
				t.Fatalf("decoded as %+v, want an error", got)
			}

			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err, tt.want)
			}

			if got != (GlobalRANNodeID{}) {
				t.Errorf("value = %+v, want it left untouched", got)
			}
		})
	}
}
