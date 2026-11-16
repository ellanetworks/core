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
)

// TS 38.413 §9.2.6.3.
type NGSetupFailure struct {
	Cause                  *Cause
	TimeToWait             *TimeToWait
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var nGSetupFailureIEs = []ieSpec[NGSetupFailure]{
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *NGSetupFailure, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *NGSetupFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDTimeToWait, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *NGSetupFailure, r *per.Reader, enc per.Encoding) error {
			var ttw TimeToWait

			if err := decodeIE(r, &ttw); err != nil {
				return err
			}

			m.TimeToWait = &ttw

			return nil
		},
		Encode: func(m *NGSetupFailure) (per.Marshaler, bool) {
			if m.TimeToWait == nil {
				return nil, false
			}

			return m.TimeToWait, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *NGSetupFailure, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *NGSetupFailure) (per.Marshaler, bool) {
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

func DecodeNGSetupFailure(value []byte) (*NGSetupFailure, error) {
	return decodeMessageBody[NGSetupFailure](ProcNGSetup, nGSetupFailureIEs, value)
}
