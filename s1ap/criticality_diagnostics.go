// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// maxnoofErrors bounds CriticalityDiagnostics-IE-List (TS 36.413 §9.3).
const maxnoofErrors = 256

// TypeOfError ::= ENUMERATED { not-understood, missing, ... } (extensible).
type TypeOfError uint8

const (
	TypeOfErrorNotUnderstood TypeOfError = iota
	TypeOfErrorMissing

	typeOfErrorRootCount = 2
)

// CriticalityDiagnosticsIEItem reports one offending IE (TS 36.413 §9.2.1.4).
type CriticalityDiagnosticsIEItem struct {
	IECriticality Criticality
	IEID          ProtocolIEID
	TypeOfError   TypeOfError
}

// CriticalityDiagnostics ::= SEQUENCE (extensible) with five optional fields.
// A nil scalar pointer or empty list means the field is absent. The optional
// iE-Extensions field is never emitted and is skipped on decode.
type CriticalityDiagnostics struct {
	ProcedureCode             *ProcedureCode
	TriggeringMessage         *TriggeringMessage
	ProcedureCriticality      *Criticality
	IEsCriticalityDiagnostics []CriticalityDiagnosticsIEItem
}

func (d CriticalityDiagnostics) encode(w *aper.Writer) error {
	optionals := []bool{
		d.ProcedureCode != nil,
		d.TriggeringMessage != nil,
		d.ProcedureCriticality != nil,
		len(d.IEsCriticalityDiagnostics) > 0,
		false, // iE-Extensions: not emitted
	}
	w.WriteSequencePreamble(true, false, optionals)

	if d.ProcedureCode != nil {
		if err := w.WriteConstrainedInt(int64(*d.ProcedureCode), 0, 255); err != nil {
			return err
		}
	}

	if d.TriggeringMessage != nil {
		if err := w.WriteEnum(int(*d.TriggeringMessage), triggeringMessageRootCount, false, false); err != nil {
			return err
		}
	}

	if d.ProcedureCriticality != nil {
		if err := w.WriteEnum(int(*d.ProcedureCriticality), criticalityRootCount, false, false); err != nil {
			return err
		}
	}

	if len(d.IEsCriticalityDiagnostics) > 0 {
		if err := encodeCritDiagIEList(w, d.IEsCriticalityDiagnostics); err != nil {
			return err
		}
	}

	return nil
}

func decodeCriticalityDiagnostics(r *aper.Reader) (CriticalityDiagnostics, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 5)
	if err != nil {
		return CriticalityDiagnostics{}, fmt.Errorf("s1ap: criticality diagnostics preamble: %w", err)
	}

	var d CriticalityDiagnostics

	if opt[0] {
		v, err := r.ReadConstrainedInt(0, 255)
		if err != nil {
			return d, fmt.Errorf("s1ap: criticality diagnostics procedureCode: %w", err)
		}

		pc := ProcedureCode(v)
		d.ProcedureCode = &pc
	}

	if opt[1] {
		v, _, err := r.ReadEnum(triggeringMessageRootCount, false)
		if err != nil {
			return d, fmt.Errorf("s1ap: criticality diagnostics triggeringMessage: %w", err)
		}

		tm := TriggeringMessage(v)
		d.TriggeringMessage = &tm
	}

	if opt[2] {
		v, _, err := r.ReadEnum(criticalityRootCount, false)
		if err != nil {
			return d, fmt.Errorf("s1ap: criticality diagnostics procedureCriticality: %w", err)
		}

		c := Criticality(v)
		d.ProcedureCriticality = &c
	}

	if opt[3] {
		list, err := decodeCritDiagIEList(r)
		if err != nil {
			return d, err
		}

		d.IEsCriticalityDiagnostics = list
	}

	if err := skipSequenceExtensions(r, opt[4], extPresent); err != nil {
		return d, err
	}

	return d, nil
}

// encodeCritDiagIEList writes CriticalityDiagnostics-IE-List ::= SEQUENCE
// (SIZE(1..maxnoofErrors)) OF CriticalityDiagnostics-IE-Item.
func encodeCritDiagIEList(w *aper.Writer, items []CriticalityDiagnosticsIEItem) error {
	if len(items) > maxnoofErrors {
		return fmt.Errorf("s1ap: %d criticality-diagnostics items exceed maxnoofErrors", len(items))
	}

	if err := w.WriteConstrainedLength(len(items), 1, maxnoofErrors); err != nil {
		return err
	}

	for _, item := range items {
		// Item SEQUENCE: extensible, one optional field (iE-Extensions, absent).
		w.WriteSequencePreamble(true, false, []bool{false})

		if err := w.WriteEnum(int(item.IECriticality), criticalityRootCount, false, false); err != nil {
			return err
		}

		if err := w.WriteConstrainedInt(int64(item.IEID), 0, maxProtocolIEs); err != nil {
			return err
		}

		if err := w.WriteEnum(int(item.TypeOfError), typeOfErrorRootCount, true, false); err != nil {
			return err
		}
	}

	return nil
}

func decodeCritDiagIEList(r *aper.Reader) ([]CriticalityDiagnosticsIEItem, error) {
	n, err := r.ReadConstrainedLength(1, maxnoofErrors)
	if err != nil {
		return nil, fmt.Errorf("s1ap: criticality diagnostics list length: %w", err)
	}

	items := make([]CriticalityDiagnosticsIEItem, 0, minInt(n, 16))

	for i := 0; i < n; i++ {
		extPresent, opt, err := r.ReadSequencePreamble(true, 1)
		if err != nil {
			return nil, fmt.Errorf("s1ap: criticality diagnostics item %d preamble: %w", i, err)
		}

		crit, _, err := r.ReadEnum(criticalityRootCount, false)
		if err != nil {
			return nil, fmt.Errorf("s1ap: criticality diagnostics item %d criticality: %w", i, err)
		}

		id, err := r.ReadConstrainedInt(0, maxProtocolIEs)
		if err != nil {
			return nil, fmt.Errorf("s1ap: criticality diagnostics item %d id: %w", i, err)
		}

		toe, _, err := r.ReadEnum(typeOfErrorRootCount, true)
		if err != nil {
			return nil, fmt.Errorf("s1ap: criticality diagnostics item %d typeOfError: %w", i, err)
		}

		if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
			return nil, err
		}

		items = append(items, CriticalityDiagnosticsIEItem{
			IECriticality: Criticality(crit),
			IEID:          ProtocolIEID(id),
			TypeOfError:   TypeOfError(toe),
		})
	}

	return items, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}
