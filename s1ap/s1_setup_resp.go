// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// S1SetupResponse is the S1 SETUP RESPONSE message (TS 36.413). An
// nil MMEName means the optional mMEname IE is absent; a nil
// CriticalityDiagnostics means that optional IE is absent.
type S1SetupResponse struct {
	MMEName                *string
	ServedGUMMEIs          ServedGUMMEIs
	RelativeMMECapacity    uint8
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *S1SetupResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	var fields []ieField

	if m.MMEName != nil {
		fields = append(fields, ieField{id: idMMEname, crit: CriticalityIgnore, val: Name(*m.MMEName)})
	}

	fields = append(fields,
		ieField{id: idServedGUMMEIs, crit: CriticalityReject, val: &m.ServedGUMMEIs},
		ieField{id: idRelativeMMECapacity, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true}, int64(m.RelativeMMECapacity))
		})},
	)

	if m.CriticalityDiagnostics != nil {
		fields = append(fields, ieField{id: idCriticalityDiagnostics, crit: CriticalityIgnore, val: m.CriticalityDiagnostics})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *S1SetupResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcS1Setup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseS1SetupResponse decodes an S1SetupResponse from the open-type payload of
// a successfulOutcome.
func ParseS1SetupResponse(value []byte) (*S1SetupResponse, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: S1SetupResponse preamble: %w", err)
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

	m := &S1SetupResponse{}

	var seenGUMMEIs, seenCapacity bool

	for _, f := range fields {
		switch f.id {
		case idMMEname:
			var n Name

			if err = perIEDecode(f.value, &n); err == nil {
				name := string(n)
				m.MMEName = &name
			}
		case idServedGUMMEIs:
			err = perIEDecode(f.value, &m.ServedGUMMEIs)
			seenGUMMEIs = true
		case idRelativeMMECapacity:
			var v int64

			v, err = per.DecodeInteger(per.NewReader(f.value), enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true})
			m.RelativeMMECapacity = uint8(v)
			seenCapacity = true
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			err = perIEDecode(f.value, &cd)
			m.CriticalityDiagnostics = &cd
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: S1SetupResponse IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcS1Setup,
		ieCheck{idServedGUMMEIs, CriticalityReject, seenGUMMEIs},
		ieCheck{idRelativeMMECapacity, CriticalityIgnore, seenCapacity},
	); err != nil {
		return nil, err
	}

	return m, nil
}
