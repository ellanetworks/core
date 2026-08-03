// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// maxnoofErrors bounds CriticalityDiagnostics-IE-List (TS 38.413).
const maxnoofErrors = 256

// TypeOfError ::= ENUMERATED { not-understood, missing, ... } (extensible).
type TypeOfError uint8

const (
	TypeOfErrorNotUnderstood TypeOfError = iota
	TypeOfErrorMissing
)

// CriticalityDiagnosticsIEItem reports one offending IE (TS 38.413 §9.3.1.3).
type CriticalityDiagnosticsIEItem struct {
	_             [0]struct{}  `per:"extseq"`
	IECriticality Criticality  `per:"ENUMERATED,range:0..2"`
	IEID          ProtocolIEID `per:",range:0..65535"`
	TypeOfError   TypeOfError  `per:"ENUMERATED,range:0..1,..."`
	_             ieExtensions `per:",skip"`
}

// CriticalityDiagnostics ::= SEQUENCE (extensible) with five optional fields.
// iE-Extensions is never emitted and is skipped on decode.
type CriticalityDiagnostics struct {
	ProcedureCode             *ProcedureCode
	TriggeringMessage         *TriggeringMessage
	ProcedureCriticality      *Criticality
	IEsCriticalityDiagnostics []CriticalityDiagnosticsIEItem
}

func (d CriticalityDiagnostics) MarshalPER(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)
	w.WriteBit(d.ProcedureCode != nil)
	w.WriteBit(d.TriggeringMessage != nil)
	w.WriteBit(d.ProcedureCriticality != nil)
	w.WriteBit(len(d.IEsCriticalityDiagnostics) > 0)
	w.WriteBit(false)

	if d.ProcedureCode != nil {
		if err := per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true}, int64(*d.ProcedureCode)); err != nil {
			return err
		}
	}

	if d.TriggeringMessage != nil {
		if err := per.EncodeEnumerated(w, enc, triggeringMessageRootCount, false, int64(*d.TriggeringMessage)); err != nil {
			return err
		}
	}

	if d.ProcedureCriticality != nil {
		if err := per.EncodeEnumerated(w, enc, criticalityRootCount, false, int64(*d.ProcedureCriticality)); err != nil {
			return err
		}
	}

	if len(d.IEsCriticalityDiagnostics) > 0 {
		if len(d.IEsCriticalityDiagnostics) > maxnoofErrors {
			return fmt.Errorf("ngap: %d criticality-diagnostics items exceed maxnoofErrors", len(d.IEsCriticalityDiagnostics))
		}

		if err := marshalSeqOf(w, enc, 1, maxnoofErrors, d.IEsCriticalityDiagnostics); err != nil {
			return err
		}
	}

	return nil
}

func (d *CriticalityDiagnostics) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	var bits [6]bool

	for i := range bits {
		b, err := r.ReadBit()
		if err != nil {
			return fmt.Errorf("ngap: criticality diagnostics preamble: %w", err)
		}

		bits[i] = b
	}

	extBit := bits[0]

	*d = CriticalityDiagnostics{}

	if bits[1] {
		v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true})
		if err != nil {
			return fmt.Errorf("ngap: criticality diagnostics procedureCode: %w", err)
		}

		pc := ProcedureCode(v)
		d.ProcedureCode = &pc
	}

	if bits[2] {
		v, err := per.DecodeEnumerated(r, enc, triggeringMessageRootCount, false)
		if err != nil {
			return fmt.Errorf("ngap: criticality diagnostics triggeringMessage: %w", err)
		}

		tm := TriggeringMessage(v)
		d.TriggeringMessage = &tm
	}

	if bits[3] {
		v, err := per.DecodeEnumerated(r, enc, criticalityRootCount, false)
		if err != nil {
			return fmt.Errorf("ngap: criticality diagnostics procedureCriticality: %w", err)
		}

		c := Criticality(v)
		d.ProcedureCriticality = &c
	}

	if bits[4] {
		items, err := unmarshalSeqOf[CriticalityDiagnosticsIEItem](r, enc, 1, maxnoofErrors)
		if err != nil {
			return err
		}

		d.IEsCriticalityDiagnostics = items
	}

	return skipSequenceExtensionsPER(r, enc, bits[5], extBit)
}
