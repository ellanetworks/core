// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// TS 38.413 §9.2.6.13. Every IE is optional and ignore criticality.
type ErrorIndication struct {
	AMFUENGAPID            *AMFUENGAPID
	RANUENGAPID            *RANUENGAPID
	Cause                  *Cause
	CriticalityDiagnostics *CriticalityDiagnostics
	FiveGSTMSI             *FiveGSTMSI

	messageMeta
}

var errorIndicationIEs = []ieSpec[ErrorIndication]{
	{
		id: IDAMFUENGAPID, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ErrorIndication, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return fmt.Errorf("ngap: ErrorIndication AMF-UE-NGAP-ID: %w", err)
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *ErrorIndication) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: IDRANUENGAPID, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ErrorIndication, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return fmt.Errorf("ngap: ErrorIndication RAN-UE-NGAP-ID: %w", err)
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *ErrorIndication) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: IDCause, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ErrorIndication, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return fmt.Errorf("ngap: ErrorIndication Cause: %w", err)
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *ErrorIndication) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ErrorIndication, raw []byte, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := perIEDecode(raw, &v); err != nil {
				return fmt.Errorf("ngap: ErrorIndication CriticalityDiagnostics: %w", err)
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		encode: func(m *ErrorIndication) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		id: IDFiveGSTMSI, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ErrorIndication, raw []byte, enc per.Encoding) error {
			var v FiveGSTMSI

			if err := perIEDecode(raw, &v); err != nil {
				return fmt.Errorf("ngap: ErrorIndication FiveG-S-TMSI: %w", err)
			}

			m.FiveGSTMSI = &v

			return nil
		},
		encode: func(m *ErrorIndication) (per.Marshaler, bool) {
			if m.FiveGSTMSI == nil {
				return nil, false
			}

			return m.FiveGSTMSI, true
		},
	},
}

func (m *ErrorIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcErrorIndication, errorIndicationIEs, m)
}

func (m *ErrorIndication) Marshal() ([]byte, error) {
	// §8.7.5.2 requires at least one of Cause and Criticality Diagnostics.
	// Every IE is optional, so the IE table cannot enforce it.
	if m.Cause == nil && m.CriticalityDiagnostics == nil {
		return nil, fmt.Errorf("ngap: ErrorIndication needs at least a Cause or Criticality Diagnostics")
	}

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
