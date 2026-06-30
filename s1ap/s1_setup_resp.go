// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// S1SetupResponse is the S1 SETUP RESPONSE message (TS 36.413). An
// empty MMEName means the optional mMEname IE is absent; a nil
// CriticalityDiagnostics means that optional IE is absent.
type S1SetupResponse struct {
	MMEName                string
	ServedGUMMEIs          ServedGUMMEIs
	RelativeMMECapacity    uint8
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *S1SetupResponse) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	var fields []ieField

	if m.MMEName != "" {
		name := m.MMEName

		fields = append(fields, ieField{id: idMMEname, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeName(w, name)
		}})
	}

	fields = append(fields,
		ieField{id: idServedGUMMEIs, crit: CriticalityReject, enc: m.ServedGUMMEIs.encode},
		ieField{id: idRelativeMMECapacity, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return w.WriteConstrainedInt(int64(m.RelativeMMECapacity), 0, 255)
		}},
	)

	if m.CriticalityDiagnostics != nil {
		fields = append(fields, ieField{id: idCriticalityDiagnostics, crit: CriticalityIgnore, enc: m.CriticalityDiagnostics.encode})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *S1SetupResponse) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcS1Setup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseS1SetupResponse decodes an S1SetupResponse from the open-type payload of
// a successfulOutcome.
func ParseS1SetupResponse(value []byte) (*S1SetupResponse, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: S1SetupResponse preamble: %w", err)
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

	m := &S1SetupResponse{}

	var seenGUMMEIs, seenCapacity bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEname:
			m.MMEName, err = decodeName(sub)
		case idServedGUMMEIs:
			m.ServedGUMMEIs, err = decodeServedGUMMEIs(sub)
			seenGUMMEIs = true
		case idRelativeMMECapacity:
			var v int64

			v, err = sub.ReadConstrainedInt(0, 255)
			m.RelativeMMECapacity = uint8(v)
			seenCapacity = true
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			cd, err = decodeCriticalityDiagnostics(sub)
			m.CriticalityDiagnostics = &cd
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: S1SetupResponse IE %d: %w", f.id, err)
		}
	}

	if !seenGUMMEIs || !seenCapacity {
		return nil, fmt.Errorf("s1ap: S1SetupResponse missing mandatory IE")
	}

	return m, nil
}
