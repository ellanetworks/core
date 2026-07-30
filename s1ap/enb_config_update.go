// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// TS 36.413 §9.1.8.7.
type ENBConfigurationUpdate struct {
	ENBName          *string
	SupportedTAs     SupportedTAs
	DefaultPagingDRX *PagingDRX

	messageMeta
}

var eNBConfigurationUpdateIEs = []ieSpec[ENBConfigurationUpdate]{
	{
		id: idENBname, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ENBConfigurationUpdate, raw []byte, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = perIEDecode(raw, &n); err == nil {
				name := string(n)
				m.ENBName = &name
			}

			return err
		},
		encode: func(m *ENBConfigurationUpdate) (per.Marshaler, bool) {
			if m.ENBName == nil {
				return nil, false
			}

			return Name(*m.ENBName), true
		},
	},
	{
		id: idSupportedTAs, presence: PresenceOptional, crit: CriticalityReject,
		decode: func(m *ENBConfigurationUpdate, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SupportedTAs)
		},
		encode: func(m *ENBConfigurationUpdate) (per.Marshaler, bool) {
			if len(m.SupportedTAs) == 0 {
				return nil, false
			}

			return &m.SupportedTAs, true
		},
	},
	{
		id: idDefaultPagingDRX, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ENBConfigurationUpdate, raw []byte, enc per.Encoding) error {
			var (
				err error
				drx PagingDRX
			)

			err = perIEDecode(raw, &drx)
			m.DefaultPagingDRX = &drx

			return err
		},
		encode: func(m *ENBConfigurationUpdate) (per.Marshaler, bool) {
			if m.DefaultPagingDRX == nil {
				return nil, false
			}

			return m.DefaultPagingDRX, true
		},
	},
}

func (m *ENBConfigurationUpdate) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcENBConfigurationUpdate, eNBConfigurationUpdateIEs, m)
}

func (m *ENBConfigurationUpdate) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcENBConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseENBConfigurationUpdate(value []byte) (*ENBConfigurationUpdate, error) {
	return parseMessageBody[ENBConfigurationUpdate](ProcENBConfigurationUpdate, TriggeringInitiatingMessage, eNBConfigurationUpdateIEs, value)
}

// TS 36.413 §9.1.8.8.
type ENBConfigurationUpdateAcknowledge struct {
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var eNBConfigurationUpdateAcknowledgeIEs = []ieSpec[ENBConfigurationUpdateAcknowledge]{
	{
		id: idCriticalityDiagnostics, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ENBConfigurationUpdateAcknowledge, raw []byte, enc per.Encoding) error {
			var (
				err error
				cd  CriticalityDiagnostics
			)

			err = perIEDecode(raw, &cd)
			m.CriticalityDiagnostics = &cd

			return err
		},
		encode: func(m *ENBConfigurationUpdateAcknowledge) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *ENBConfigurationUpdateAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcENBConfigurationUpdate, eNBConfigurationUpdateAcknowledgeIEs, m)
}

func (m *ENBConfigurationUpdateAcknowledge) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcENBConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseENBConfigurationUpdateAcknowledge(value []byte) (*ENBConfigurationUpdateAcknowledge, error) {
	return parseMessageBody[ENBConfigurationUpdateAcknowledge](ProcENBConfigurationUpdate, TriggeringSuccessfulOutcome, eNBConfigurationUpdateAcknowledgeIEs, value)
}

// TS 36.413 §9.1.8.9.
type ENBConfigurationUpdateFailure struct {
	Cause                  *Cause
	TimeToWait             *TimeToWait
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var eNBConfigurationUpdateFailureIEs = []ieSpec[ENBConfigurationUpdateFailure]{
	{
		id: idCause, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *ENBConfigurationUpdateFailure, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *ENBConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: idTimeToWait, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ENBConfigurationUpdateFailure, raw []byte, enc per.Encoding) error {
			var (
				err error
				ttw TimeToWait
			)

			err = perIEDecode(raw, &ttw)
			m.TimeToWait = &ttw

			return err
		},
		encode: func(m *ENBConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.TimeToWait == nil {
				return nil, false
			}

			return m.TimeToWait, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ENBConfigurationUpdateFailure, raw []byte, enc per.Encoding) error {
			var (
				err error
				cd  CriticalityDiagnostics
			)

			err = perIEDecode(raw, &cd)
			m.CriticalityDiagnostics = &cd

			return err
		},
		encode: func(m *ENBConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *ENBConfigurationUpdateFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcENBConfigurationUpdate, eNBConfigurationUpdateFailureIEs, m)
}

func (m *ENBConfigurationUpdateFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcENBConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseENBConfigurationUpdateFailure(value []byte) (*ENBConfigurationUpdateFailure, error) {
	return parseMessageBody[ENBConfigurationUpdateFailure](ProcENBConfigurationUpdate, TriggeringUnsuccessfulOutcome, eNBConfigurationUpdateFailureIEs, value)
}
