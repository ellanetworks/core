// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
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

func (t TimeToWait) encode(w *aper.Writer) error {
	return w.WriteEnum(int(t), timeToWaitRootCount, true, false)
}

func decodeTimeToWait(r *aper.Reader) (TimeToWait, error) {
	idx, _, err := r.ReadEnum(timeToWaitRootCount, true)
	if err != nil {
		return 0, err
	}

	return TimeToWait(idx), nil
}

// S1SetupFailure is the S1 SETUP FAILURE message (TS 36.413). TimeToWait
// and CriticalityDiagnostics are optional (nil = absent).
type S1SetupFailure struct {
	Cause                  Cause
	TimeToWait             *TimeToWait
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *S1SetupFailure) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idCause, crit: CriticalityIgnore, enc: m.Cause.encode},
	}

	if m.TimeToWait != nil {
		ttw := *m.TimeToWait
		fields = append(fields, ieField{id: idTimeToWait, crit: CriticalityIgnore, enc: ttw.encode})
	}

	if m.CriticalityDiagnostics != nil {
		fields = append(fields, ieField{id: idCriticalityDiagnostics, crit: CriticalityIgnore, enc: m.CriticalityDiagnostics.encode})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *S1SetupFailure) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcS1Setup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseS1SetupFailure decodes an S1SetupFailure from the open-type payload of an
// unsuccessfulOutcome.
func ParseS1SetupFailure(value []byte) (*S1SetupFailure, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: S1SetupFailure preamble: %w", err)
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

	m := &S1SetupFailure{}

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
			return nil, fmt.Errorf("s1ap: S1SetupFailure IE %d: %w", f.id, err)
		}
	}

	if !seenCause {
		return nil, fmt.Errorf("s1ap: S1SetupFailure missing mandatory Cause IE")
	}

	return m, nil
}
