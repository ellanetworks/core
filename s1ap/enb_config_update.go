// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
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

func (m *ENBConfigurationUpdate) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	var fields []ieField

	if m.ENBName != "" {
		name := m.ENBName

		fields = append(fields, ieField{id: idENBname, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeName(w, name)
		}})
	}

	if len(m.SupportedTAs) > 0 {
		fields = append(fields, ieField{id: idSupportedTAs, crit: CriticalityReject, enc: m.SupportedTAs.encode})
	}

	if m.DefaultPagingDRX != nil {
		drx := *m.DefaultPagingDRX
		fields = append(fields, ieField{id: idDefaultPagingDRX, crit: CriticalityIgnore, enc: drx.encode})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ENBConfigurationUpdate) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcENBConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseENBConfigurationUpdate decodes the message from an initiatingMessage
// open-type payload.
func ParseENBConfigurationUpdate(value []byte) (*ENBConfigurationUpdate, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: ENBConfigurationUpdate preamble: %w", err)
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

	m := &ENBConfigurationUpdate{}

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idENBname:
			m.ENBName, err = decodeName(sub)
		case idSupportedTAs:
			m.SupportedTAs, err = decodeSupportedTAs(sub)
		case idDefaultPagingDRX:
			var drx PagingDRX

			drx, err = decodePagingDRX(sub)
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

func (m *ENBConfigurationUpdateAcknowledge) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	var fields []ieField

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
func (m *ENBConfigurationUpdateAcknowledge) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcENBConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseENBConfigurationUpdateAcknowledge decodes the message from a
// successfulOutcome open-type payload.
func ParseENBConfigurationUpdateAcknowledge(value []byte) (*ENBConfigurationUpdateAcknowledge, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: ENBConfigurationUpdateAcknowledge preamble: %w", err)
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

	m := &ENBConfigurationUpdateAcknowledge{}

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			cd, err = decodeCriticalityDiagnostics(sub)
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

func (m *ENBConfigurationUpdateFailure) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idCause, crit: CriticalityIgnore, enc: m.Cause.encode},
	}

	if m.TimeToWait != nil {
		ttw := *m.TimeToWait
		fields = append(fields, ieField{id: idTimeToWait, crit: CriticalityIgnore, enc: ttw.encode})
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
func (m *ENBConfigurationUpdateFailure) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcENBConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseENBConfigurationUpdateFailure decodes the message from an
// unsuccessfulOutcome open-type payload.
func ParseENBConfigurationUpdateFailure(value []byte) (*ENBConfigurationUpdateFailure, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: ENBConfigurationUpdateFailure preamble: %w", err)
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

	m := &ENBConfigurationUpdateFailure{}

	var seenCause bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idCause:
			m.Cause, err = decodeCause(sub)
			seenCause = true
		case idTimeToWait:
			var ttw TimeToWait

			ttw, err = decodeTimeToWait(sub)
			m.TimeToWait = &ttw
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			cd, err = decodeCriticalityDiagnostics(sub)
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
