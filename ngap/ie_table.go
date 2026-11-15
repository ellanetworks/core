// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

// One row of a message's TS 38.413 §9.2/§9.3 IE table.
type ieSpec[M any] = ap.IESpec[ProtocolIEID, M]

const (
	presenceOptional    = ap.PresenceOptional
	presenceConditional = ap.PresenceConditional
	presenceMandatory   = ap.PresenceMandatory
)

type messageMeta struct {
	ap.Message[ProtocolIEID]
}

type extensionMeta = ap.Preserved[ProtocolIEID]

func (u *messageMeta) meta() *messageMeta { return u }

func (u messageMeta) Diagnostics() Diagnostics { return Diagnostics{u.Message.Diagnostics()} }

type message interface {
	meta() *messageMeta
}

func encodeMessageBody[M any, PM interface {
	*M
	message
}](w *per.Writer, enc per.Encoding, procedure ProcedureCode, table []ieSpec[M], m PM,
) error {
	if err := ap.EncodeMessage(profile, w, enc, table, (*M)(m), &m.meta().Message); err != nil {
		return fmt.Errorf("ngap: %s: %w", procedure, err)
	}

	return nil
}

func decodeMessageBody[M any, PM interface {
	*M
	message
}](procedure ProcedureCode, table []ieSpec[M], value []byte,
) (PM, error) {
	m := PM(new(M))

	if err := ap.DecodeMessage(profile, table, value, (*M)(m), &m.meta().Message); err != nil {
		return nil, &TransferSyntaxError{Protocol: profile.Name, Procedure: procedure, Err: err}
	}

	return m, nil
}

func parseMessageBody[M any, PM interface {
	*M
	message
}](procedure ProcedureCode, trigger TriggeringMessage, table []ieSpec[M], value []byte,
) (PM, error) {
	m, err := decodeMessageBody[M, PM](procedure, table, value)
	if err != nil {
		return nil, err
	}

	if err := m.meta().Message.Diagnostics().Check(); err != nil {
		return nil, newAbstractSyntaxError(procedure, trigger, err, table, value)
	}

	return m, nil
}

func newAbstractSyntaxError[M any](procedure ProcedureCode, trigger TriggeringMessage, err error, table []ieSpec[M], value []byte) error {
	var e *ap.AbstractSyntaxError[ProtocolIEID]
	if !errors.As(err, &e) {
		return err
	}

	cause := CauseProtocolAbstractSyntaxErrorReject
	if e.FalselyConstructed {
		cause = CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage
	}

	return &AbstractSyntaxError{
		Procedure: procedure,
		Trigger:   trigger,
		Cause:     cause.Cause(),
		IEs:       criticalityDiagnosticsItems(e.IEs),
		decoded:   modeledIEs(table, value),
	}
}

func criticalityDiagnosticsItems(ds []DiagnosticIE) []CriticalityDiagnosticsIEItem {
	var out []CriticalityDiagnosticsIEItem

	for _, d := range ds {
		if len(out) >= maxnoofErrors {
			break
		}

		kind := TypeOfErrorNotUnderstood
		if d.Kind == ap.KindMissing {
			kind = TypeOfErrorMissing
		}

		out = append(out, CriticalityDiagnosticsIEItem{IECriticality: d.Criticality, IEID: d.ID, TypeOfError: kind})
	}

	return out
}

func modeledIEs[M any](table []ieSpec[M], value []byte) []RawIE {
	r := per.NewReader(value)

	if _, err := r.ReadBit(); err != nil {
		return nil
	}

	fields, err := decodeIEContainer(r, per.Aligned)
	if err != nil {
		return nil
	}

	var out []RawIE

	seen := make(map[ProtocolIEID]bool, len(table))

	for _, f := range fields {
		if seen[f.ID] || !inTable(table, f.ID) {
			continue
		}

		seen[f.ID] = true

		out = append(out, f)
	}

	return out
}

func inTable[M any](table []ieSpec[M], id ProtocolIEID) bool {
	for i := range table {
		if table[i].ID == id {
			return true
		}
	}

	return false
}

func decodeTransfer[T any, PT interface {
	*T
	per.Unmarshaler
}](b []byte,
) (PT, error) {
	t := PT(new(T))

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return t, nil
}

func parseTransfer[T any, PT interface {
	*T
	per.Unmarshaler
}](b []byte,
) (PT, error) {
	t, err := decodeTransfer[T, PT](b)
	if err != nil {
		return nil, err
	}

	if err := diagnosticsOf(t).Check(); err != nil {
		return nil, fmt.Errorf("ngap: %w", err)
	}

	return t, nil
}

func diagnosticsOf(v any) Diagnostics {
	dv, ok := v.(ap.Diagnosable[ProtocolIEID])
	if !ok {
		return Diagnostics{}
	}

	return Diagnostics{ap.Diagnose(dv)}
}
