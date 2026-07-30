// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// ENBConfigurationUpdate is the ENB CONFIGURATION UPDATE message (TS 36.413),
// sent by a running eNB to update its configuration without redoing
// S1 Setup. Every IE is optional; the eNB sends only what changed.
type ENBConfigurationUpdate struct {
	ENBName          string       // "" = absent
	SupportedTAs     SupportedTAs // nil = absent
	DefaultPagingDRX *PagingDRX   // nil = absent

	unmodeledIEs
}

func (m *ENBConfigurationUpdate) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	var fields []ieField

	if m.ENBName != "" {
		name := m.ENBName

		fields = append(fields, ieField{id: idENBname, crit: CriticalityIgnore, val: Name(name)})
	}

	if len(m.SupportedTAs) > 0 {
		fields = append(fields, ieField{id: idSupportedTAs, crit: CriticalityReject, val: &m.SupportedTAs})
	}

	if m.DefaultPagingDRX != nil {
		drx := *m.DefaultPagingDRX
		fields = append(fields, ieField{id: idDefaultPagingDRX, crit: CriticalityIgnore, val: &drx})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ENBConfigurationUpdate) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcENBConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseENBConfigurationUpdate decodes the message from an initiatingMessage
// open-type payload.
func ParseENBConfigurationUpdate(value []byte) (*ENBConfigurationUpdate, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: ENBConfigurationUpdate preamble: %w", err)
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

	m := &ENBConfigurationUpdate{}

	for _, f := range fields {
		switch f.id {
		case idENBname:
			var n Name

			err = perIEDecode(f.value, &n)
			m.ENBName = string(n)
		case idSupportedTAs:
			err = perIEDecode(f.value, &m.SupportedTAs)
		case idDefaultPagingDRX:
			var drx PagingDRX

			err = perIEDecode(f.value, &drx)
			m.DefaultPagingDRX = &drx
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: ENBConfigurationUpdate IE %d: %w", f.id, err)
		}
	}

	return m, nil
}

// ENBConfigurationUpdateAcknowledge is the ENB CONFIGURATION UPDATE ACKNOWLEDGE
// message (TS 36.413), the MME's success response.
type ENBConfigurationUpdateAcknowledge struct {
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *ENBConfigurationUpdateAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	var fields []ieField

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
func (m *ENBConfigurationUpdateAcknowledge) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcENBConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseENBConfigurationUpdateAcknowledge decodes the message from a
// successfulOutcome open-type payload.
func ParseENBConfigurationUpdateAcknowledge(value []byte) (*ENBConfigurationUpdateAcknowledge, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: ENBConfigurationUpdateAcknowledge preamble: %w", err)
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

	m := &ENBConfigurationUpdateAcknowledge{}

	for _, f := range fields {
		switch f.id {
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			err = perIEDecode(f.value, &cd)
			m.CriticalityDiagnostics = &cd
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: ENBConfigurationUpdateAcknowledge IE %d: %w", f.id, err)
		}
	}

	return m, nil
}

// ENBConfigurationUpdateFailure is the ENB CONFIGURATION UPDATE FAILURE message
// (TS 36.413), the MME's rejection (e.g. the updated TAs broadcast no
// served PLMN).
type ENBConfigurationUpdateFailure struct {
	Cause                  Cause
	TimeToWait             *TimeToWait
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *ENBConfigurationUpdateFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idCause, crit: CriticalityIgnore, val: &m.Cause},
	}

	if m.TimeToWait != nil {
		ttw := *m.TimeToWait
		fields = append(fields, ieField{id: idTimeToWait, crit: CriticalityIgnore, val: &ttw})
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
func (m *ENBConfigurationUpdateFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcENBConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseENBConfigurationUpdateFailure decodes the message from an
// unsuccessfulOutcome open-type payload.
func ParseENBConfigurationUpdateFailure(value []byte) (*ENBConfigurationUpdateFailure, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: ENBConfigurationUpdateFailure preamble: %w", err)
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

	m := &ENBConfigurationUpdateFailure{}

	var seenCause bool

	for _, f := range fields {
		switch f.id {
		case idCause:
			err = perIEDecode(f.value, &m.Cause)
			seenCause = true
		case idTimeToWait:
			var ttw TimeToWait

			err = perIEDecode(f.value, &ttw)
			m.TimeToWait = &ttw
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			err = perIEDecode(f.value, &cd)
			m.CriticalityDiagnostics = &cd
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: ENBConfigurationUpdateFailure IE %d: %w", f.id, err)
		}
	}

	if !seenCause {
		return nil, fmt.Errorf("s1ap: ENBConfigurationUpdateFailure missing mandatory Cause IE")
	}

	return m, nil
}
