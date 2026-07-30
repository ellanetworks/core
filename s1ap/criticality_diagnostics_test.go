// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
)

func TestCriticalityDiagnosticsEmpty(t *testing.T) {
	w := per.NewWriter()

	if err := (CriticalityDiagnostics{}).MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	// ext bit 0 + five absent presence bits = 6 zero bits -> 0x00.
	if want := []byte{0x00}; !bytes.Equal(perBytes(w), want) {
		t.Fatalf("empty = % x, want % x", perBytes(w), want)
	}

	d, err := unmarshalPERValue[CriticalityDiagnostics](perBytes(w))
	if err != nil {
		t.Fatal(err)
	}

	if d.ProcedureCode != nil || d.TriggeringMessage != nil ||
		d.ProcedureCriticality != nil || len(d.IEsCriticalityDiagnostics) != 0 {
		t.Fatalf("expected empty, got %+v", d)
	}
}

func TestCriticalityDiagnosticsRoundTrip(t *testing.T) {
	pc := ProcS1Setup
	tm := TriggeringInitiatingMessage
	cr := CriticalityReject

	in := CriticalityDiagnostics{
		ProcedureCode:        &pc,
		TriggeringMessage:    &tm,
		ProcedureCriticality: &cr,
		IEsCriticalityDiagnostics: []CriticalityDiagnosticsIEItem{
			{IECriticality: CriticalityReject, IEID: 59, TypeOfError: TypeOfErrorMissing},
			{IECriticality: CriticalityIgnore, IEID: 1, TypeOfError: TypeOfErrorNotUnderstood},
		},
	}

	w := per.NewWriter()
	if err := in.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	out, err := unmarshalPERValue[CriticalityDiagnostics](perBytes(w))
	if err != nil {
		t.Fatal(err)
	}

	if out.ProcedureCode == nil || *out.ProcedureCode != pc {
		t.Fatalf("procedureCode = %v", out.ProcedureCode)
	}

	if out.TriggeringMessage == nil || *out.TriggeringMessage != tm {
		t.Fatalf("triggeringMessage = %v", out.TriggeringMessage)
	}

	if out.ProcedureCriticality == nil || *out.ProcedureCriticality != cr {
		t.Fatalf("procedureCriticality = %v", out.ProcedureCriticality)
	}

	if len(out.IEsCriticalityDiagnostics) != len(in.IEsCriticalityDiagnostics) {
		t.Fatalf("list length = %d", len(out.IEsCriticalityDiagnostics))
	}

	for i := range in.IEsCriticalityDiagnostics {
		if out.IEsCriticalityDiagnostics[i] != in.IEsCriticalityDiagnostics[i] {
			t.Fatalf("item %d: got %+v, want %+v", i,
				out.IEsCriticalityDiagnostics[i], in.IEsCriticalityDiagnostics[i])
		}
	}
}

// TestCriticalityDiagnosticsSkipsExtensions hand-builds a message carrying both
// an iE-Extensions ProtocolExtensionContainer and a SEQUENCE extension
// addition, as a future/peer encoder might. The decoder must step over both and
// still succeed.
func TestCriticalityDiagnosticsSkipsExtensions(t *testing.T) {
	w := per.NewWriter()

	// Preamble: extensible, extension additions present, only iE-Extensions set.
	for _, bit := range []bool{true, false, false, false, false, true} {
		w.WriteBit(bit)
	}

	// iE-Extensions: ProtocolExtensionContainer with one field.
	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 1, maxProtocolExtensions, 1); err != nil {
		t.Fatal(err)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, 100); err != nil {
		t.Fatal(err)
	}

	if err := per.EncodeEnumerated(w, per.Aligned, criticalityRootCount, false, int64(int(CriticalityIgnore))); err != nil {
		t.Fatal(err)
	}

	if err := per.EncodeOpenTypeBytes(w, per.Aligned, []byte{0xaa}); err != nil {
		t.Fatal(err)
	}

	// One SEQUENCE extension addition, present.
	if err := per.EncodeNormallySmallLength(w, per.Aligned, 1, func(int64) error {
		w.WriteBit(true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := per.EncodeOpenTypeBytes(w, per.Aligned, []byte{0xbb, 0xcc}); err != nil {
		t.Fatal(err)
	}

	d, err := unmarshalPERValue[CriticalityDiagnostics](perBytes(w))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if d.ProcedureCode != nil || len(d.IEsCriticalityDiagnostics) != 0 {
		t.Fatalf("expected all modeled fields absent, got %+v", d)
	}
}

func TestCriticalityDiagnosticsPartial(t *testing.T) {
	// Only procedureCode set; exercises a sparse presence bitmap.
	pc := ProcErrorIndication

	w := per.NewWriter()
	if err := (CriticalityDiagnostics{ProcedureCode: &pc}).MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	out, err := unmarshalPERValue[CriticalityDiagnostics](perBytes(w))
	if err != nil {
		t.Fatal(err)
	}

	if out.ProcedureCode == nil || *out.ProcedureCode != pc || out.TriggeringMessage != nil {
		t.Fatalf("got %+v", out)
	}
}
