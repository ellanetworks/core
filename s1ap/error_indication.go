// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// ErrorIndication is the ERROR INDICATION message (TS 36.413). It
// reports a protocol error not handled by a procedure-specific failure message.
// All IEs are optional.
type ErrorIndication struct {
	MMEUES1APID            *MMEUES1APID
	ENBUES1APID            *ENBUES1APID
	Cause                  *Cause
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *ErrorIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	var fields []ieField

	if m.MMEUES1APID != nil {
		id := *m.MMEUES1APID
		fields = append(fields, ieField{id: idMMEUES1APID, crit: CriticalityIgnore, val: &id})
	}

	if m.ENBUES1APID != nil {
		id := *m.ENBUES1APID
		fields = append(fields, ieField{id: idENBUES1APID, crit: CriticalityIgnore, val: &id})
	}

	if m.Cause != nil {
		c := *m.Cause
		fields = append(fields, ieField{id: idCause, crit: CriticalityIgnore, val: &c})
	}

	if m.CriticalityDiagnostics != nil {
		d := *m.CriticalityDiagnostics
		fields = append(fields, ieField{id: idCriticalityDiagnostics, crit: CriticalityIgnore, val: &d})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ErrorIndication) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcErrorIndication,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParseErrorIndication decodes the message from an initiatingMessage open-type
// payload.
func ParseErrorIndication(value []byte) (*ErrorIndication, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: ErrorIndication preamble: %w", err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, err
		}
	}

	m := &ErrorIndication{}

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			var v MMEUES1APID

			if err := perIEDecode(f.value, &v); err != nil {
				return nil, fmt.Errorf("s1ap: ErrorIndication MME-UE-S1AP-ID: %w", err)
			}

			m.MMEUES1APID = &v
		case idENBUES1APID:
			var v ENBUES1APID

			if err := perIEDecode(f.value, &v); err != nil {
				return nil, fmt.Errorf("s1ap: ErrorIndication eNB-UE-S1AP-ID: %w", err)
			}

			m.ENBUES1APID = &v
		case idCause:
			var v Cause

			if err := perIEDecode(f.value, &v); err != nil {
				return nil, fmt.Errorf("s1ap: ErrorIndication Cause: %w", err)
			}

			m.Cause = &v
		case idCriticalityDiagnostics:
			var v CriticalityDiagnostics

			if err := perIEDecode(f.value, &v); err != nil {
				return nil, fmt.Errorf("s1ap: ErrorIndication CriticalityDiagnostics: %w", err)
			}

			m.CriticalityDiagnostics = &v
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}
	}

	return m, nil
}
