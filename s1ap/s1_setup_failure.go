// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

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

// TS 36.413 §9.1.8.6.
type S1SetupFailure struct {
	Cause                  *Cause
	TimeToWait             *TimeToWait
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var s1SetupFailureIEs = []ieSpec[S1SetupFailure]{
	{
		id: idCause, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *S1SetupFailure, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *S1SetupFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: idTimeToWait, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *S1SetupFailure, raw []byte, enc per.Encoding) error {
			var (
				err error
				ttw TimeToWait
			)

			err = perIEDecode(raw, &ttw)
			m.TimeToWait = &ttw

			return err
		},
		encode: func(m *S1SetupFailure) (per.Marshaler, bool) {
			if m.TimeToWait == nil {
				return nil, false
			}

			return m.TimeToWait, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *S1SetupFailure, raw []byte, enc per.Encoding) error {
			var (
				err error
				cd  CriticalityDiagnostics
			)

			err = perIEDecode(raw, &cd)
			m.CriticalityDiagnostics = &cd

			return err
		},
		encode: func(m *S1SetupFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *S1SetupFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcS1Setup, s1SetupFailureIEs, m)
}

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

func ParseS1SetupFailure(value []byte) (*S1SetupFailure, error) {
	return parseMessageBody[S1SetupFailure](ProcS1Setup, TriggeringUnsuccessfulOutcome, s1SetupFailureIEs, value)
}
