// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// TS 36.413 §9.1.8.3.
type ErrorIndication struct {
	MMEUES1APID            *MMEUES1APID
	ENBUES1APID            *ENBUES1APID
	Cause                  *Cause
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var errorIndicationIEs = []ieSpec[ErrorIndication]{
	{
		id: idMMEUES1APID, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ErrorIndication, raw []byte, enc per.Encoding) error {
			var (
				err error
				v   MMEUES1APID
			)
			if err := perIEDecode(raw, &v); err != nil {
				return fmt.Errorf("s1ap: ErrorIndication MME-UE-S1AP-ID: %w", err)
			}

			m.MMEUES1APID = &v

			return err
		},
		encode: func(m *ErrorIndication) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		id: idENBUES1APID, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ErrorIndication, raw []byte, enc per.Encoding) error {
			var (
				err error
				v   ENBUES1APID
			)
			if err := perIEDecode(raw, &v); err != nil {
				return fmt.Errorf("s1ap: ErrorIndication eNB-UE-S1AP-ID: %w", err)
			}

			m.ENBUES1APID = &v

			return err
		},
		encode: func(m *ErrorIndication) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		id: idCause, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ErrorIndication, raw []byte, enc per.Encoding) error {
			var (
				err error
				v   Cause
			)
			if err := perIEDecode(raw, &v); err != nil {
				return fmt.Errorf("s1ap: ErrorIndication Cause: %w", err)
			}

			m.Cause = &v

			return err
		},
		encode: func(m *ErrorIndication) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ErrorIndication, raw []byte, enc per.Encoding) error {
			var (
				err error
				v   CriticalityDiagnostics
			)
			if err := perIEDecode(raw, &v); err != nil {
				return fmt.Errorf("s1ap: ErrorIndication CriticalityDiagnostics: %w", err)
			}

			m.CriticalityDiagnostics = &v

			return err
		},
		encode: func(m *ErrorIndication) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *ErrorIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcErrorIndication, errorIndicationIEs, m)
}

func (m *ErrorIndication) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcErrorIndication,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseErrorIndication(value []byte) (*ErrorIndication, error) {
	return parseMessageBody[ErrorIndication](ProcErrorIndication, TriggeringInitiatingMessage, errorIndicationIEs, value)
}
