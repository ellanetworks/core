// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// TimeToWait ::= ENUMERATED { v1s, v2s, v5s, v10s, v20s, v60s, ... } (extensible).
type TimeToWait uint8

const (
	TimeToWaitV1s TimeToWait = iota
	TimeToWaitV2s
	TimeToWaitV5s
	TimeToWaitV10s
	TimeToWaitV20s
	TimeToWaitV60s

	timeToWaitRootCount = 6
)

func (t TimeToWait) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeEnumerated(w, enc, timeToWaitRootCount, true, int64(t))
}

func (t *TimeToWait) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeEnumerated(r, enc, timeToWaitRootCount, true)
	if err != nil {
		return err
	}

	*t = TimeToWait(idx)

	return nil
}

// S1SetupFailure is the S1 SETUP FAILURE message (TS 36.413). TimeToWait
// and CriticalityDiagnostics are optional (nil = absent).
type S1SetupFailure struct {
	Cause                  Cause
	TimeToWait             *TimeToWait
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *S1SetupFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idCause, crit: CriticalityIgnore, val: &m.Cause},
	}

	if m.TimeToWait != nil {
		ttw := *m.TimeToWait
		fields = append(fields, ieField{id: idTimeToWait, crit: CriticalityIgnore, val: &ttw})
	}

	if m.CriticalityDiagnostics != nil {
		fields = append(fields, ieField{id: idCriticalityDiagnostics, crit: CriticalityIgnore, val: m.CriticalityDiagnostics})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *S1SetupFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcS1Setup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseS1SetupFailure decodes an S1SetupFailure from the open-type payload of an
// unsuccessfulOutcome.
func ParseS1SetupFailure(value []byte) (*S1SetupFailure, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: S1SetupFailure preamble: %w", err)
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

	m := &S1SetupFailure{}

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
			return nil, fmt.Errorf("s1ap: S1SetupFailure IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcS1Setup,
		ieCheck{idCause, CriticalityIgnore, seenCause},
	); err != nil {
		return nil, err
	}

	return m, nil
}
