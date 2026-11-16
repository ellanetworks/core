// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

func TestUEAggregateMaximumBitRateExtendedBitRates(t *testing.T) {
	dl, ul := ExtendedBitRate(25_000_000_000), ExtendedBitRate(12_000_000_000)

	w := per.NewWriter()
	w.WriteBit(false)
	w.WriteBit(true)
	mustNil(t, BitRate(10_000_000_000).MarshalPER(w, per.Aligned))
	mustNil(t, BitRate(10_000_000_000).MarshalPER(w, per.Aligned))
	mustNil(t, encodeExtensionContainer(w, []ieField{
		{ID: 259, Criticality: CriticalityIgnore, Value: dl},
		{ID: 260, Criticality: CriticalityIgnore, Value: ul},
	}))
	w.AlignToByte()

	raw := w.Bytes()

	var got UEAggregateMaximumBitRate
	mustNil(t, got.UnmarshalPER(per.NewReader(raw), per.Aligned))

	if got.ExtendedDL == nil || *got.ExtendedDL != dl || got.ExtendedUL == nil || *got.ExtendedUL != ul {
		t.Fatalf("extended UE-AMBR = %v/%v, want %d/%d", got.ExtendedDL, got.ExtendedUL, dl, ul)
	}

	if d := ap.Diagnose[ProtocolIEID](&got); !d.Empty() {
		t.Fatalf("diagnostics = %+v, want none for spec-defined extensions", d)
	}

	out, err := per.Marshal(&got, per.Aligned)
	mustNil(t, err)

	if !bytes.Equal(out, raw) {
		t.Fatalf("re-encoded %x, want %x", out, raw)
	}

	built, err := per.Marshal(&UEAggregateMaximumBitRate{DL: 10_000_000_000, UL: 10_000_000_000, ExtendedDL: &dl, ExtendedUL: &ul}, per.Aligned)
	mustNil(t, err)

	if !bytes.Equal(built, raw) {
		t.Fatalf("built %x, want %x", built, raw)
	}
}
