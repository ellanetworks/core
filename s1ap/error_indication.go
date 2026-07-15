// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/aper"
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

func (m *ErrorIndication) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	var fields []ieField

	if m.MMEUES1APID != nil {
		id := *m.MMEUES1APID
		fields = append(fields, ieField{id: idMMEUES1APID, crit: CriticalityIgnore, enc: id.encode})
	}

	if m.ENBUES1APID != nil {
		id := *m.ENBUES1APID
		fields = append(fields, ieField{id: idENBUES1APID, crit: CriticalityIgnore, enc: id.encode})
	}

	if m.Cause != nil {
		c := *m.Cause
		fields = append(fields, ieField{id: idCause, crit: CriticalityIgnore, enc: c.encode})
	}

	if m.CriticalityDiagnostics != nil {
		d := *m.CriticalityDiagnostics
		fields = append(fields, ieField{id: idCriticalityDiagnostics, crit: CriticalityIgnore, enc: d.encode})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ErrorIndication) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcErrorIndication,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParseErrorIndication decodes the message from an initiatingMessage open-type
// payload.
func ParseErrorIndication(value []byte) (*ErrorIndication, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: ErrorIndication preamble: %w", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, err
		}
	}

	m := &ErrorIndication{}

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			v, err := decodeMMEUES1APID(sub)
			if err != nil {
				return nil, fmt.Errorf("s1ap: ErrorIndication MME-UE-S1AP-ID: %w", err)
			}

			m.MMEUES1APID = &v
		case idENBUES1APID:
			v, err := decodeENBUES1APID(sub)
			if err != nil {
				return nil, fmt.Errorf("s1ap: ErrorIndication eNB-UE-S1AP-ID: %w", err)
			}

			m.ENBUES1APID = &v
		case idCause:
			v, err := decodeCause(sub)
			if err != nil {
				return nil, fmt.Errorf("s1ap: ErrorIndication Cause: %w", err)
			}

			m.Cause = &v
		case idCriticalityDiagnostics:
			v, err := decodeCriticalityDiagnostics(sub)
			if err != nil {
				return nil, fmt.Errorf("s1ap: ErrorIndication CriticalityDiagnostics: %w", err)
			}

			m.CriticalityDiagnostics = &v
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}
	}

	return m, nil
}
