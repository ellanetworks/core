// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/aper"
)

func TestCriticalityDiagnosticsEmpty(t *testing.T) {
	var w aper.Writer

	if err := (CriticalityDiagnostics{}).encode(&w); err != nil {
		t.Fatal(err)
	}

	// ext bit 0 + five absent presence bits = 6 zero bits -> 0x00.
	if want := []byte{0x00}; !bytes.Equal(w.Bytes(), want) {
		t.Fatalf("empty = % x, want % x", w.Bytes(), want)
	}

	d, err := decodeCriticalityDiagnostics(aper.NewReader(w.Bytes()))
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

	var w aper.Writer
	if err := in.encode(&w); err != nil {
		t.Fatal(err)
	}

	out, err := decodeCriticalityDiagnostics(aper.NewReader(w.Bytes()))
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
	var w aper.Writer

	// Preamble: extensible, extension additions present, only iE-Extensions set.
	w.WriteSequencePreamble(true, true, []bool{false, false, false, false, true})

	// iE-Extensions: ProtocolExtensionContainer with one field.
	if err := w.WriteConstrainedLength(1, 1, maxProtocolExtensions); err != nil {
		t.Fatal(err)
	}

	if err := w.WriteConstrainedInt(100, 0, maxProtocolIEs); err != nil {
		t.Fatal(err)
	}

	if err := w.WriteEnum(int(CriticalityIgnore), criticalityRootCount, false, false); err != nil {
		t.Fatal(err)
	}

	if err := w.WriteOpenType([]byte{0xaa}); err != nil {
		t.Fatal(err)
	}

	// One SEQUENCE extension addition, present.
	if err := w.WriteNSLength(1); err != nil {
		t.Fatal(err)
	}

	w.WriteBool(true)

	if err := w.WriteOpenType([]byte{0xbb, 0xcc}); err != nil {
		t.Fatal(err)
	}

	d, err := decodeCriticalityDiagnostics(aper.NewReader(w.Bytes()))
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

	var w aper.Writer
	if err := (CriticalityDiagnostics{ProcedureCode: &pc}).encode(&w); err != nil {
		t.Fatal(err)
	}

	out, err := decodeCriticalityDiagnostics(aper.NewReader(w.Bytes()))
	if err != nil {
		t.Fatal(err)
	}

	if out.ProcedureCode == nil || *out.ProcedureCode != pc || out.TriggeringMessage != nil {
		t.Fatalf("got %+v", out)
	}
}
