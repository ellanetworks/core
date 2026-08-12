// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
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
	return encodeRootEnumerated(w, enc, timeToWaitRootCount, int64(t), "TimeToWait")
}

func (t *TimeToWait) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, timeToWaitRootCount, "TimeToWait")
	if err != nil {
		return err
	}

	*t = TimeToWait(idx)

	return nil
}

// TS 38.413 §9.2.6.3.
type NGSetupFailure struct {
	Cause                  *Cause
	TimeToWait             *TimeToWait
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var nGSetupFailureIEs = []ieSpec[NGSetupFailure]{
	{
		id: IDCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *NGSetupFailure, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *NGSetupFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: IDTimeToWait, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *NGSetupFailure, raw []byte, enc per.Encoding) error {
			var ttw TimeToWait

			if err := perIEDecode(raw, &ttw); err != nil {
				return err
			}

			m.TimeToWait = &ttw

			return nil
		},
		encode: func(m *NGSetupFailure) (per.Marshaler, bool) {
			if m.TimeToWait == nil {
				return nil, false
			}

			return m.TimeToWait, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *NGSetupFailure, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *NGSetupFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *NGSetupFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcNGSetup, nGSetupFailureIEs, m)
}

func (m *NGSetupFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcNGSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseNGSetupFailure(value []byte) (*NGSetupFailure, error) {
	return parseMessageBody[NGSetupFailure](ProcNGSetup, TriggeringUnsuccessfulOutcome, nGSetupFailureIEs, value)
}
