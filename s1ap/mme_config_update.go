// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

type MMEConfigurationUpdate struct {
	MMEName             *string
	ServedGUMMEIs       ServedGUMMEIs
	RelativeMMECapacity *uint8

	messageMeta
}

var mMEConfigurationUpdateIEs = []ieSpec[MMEConfigurationUpdate]{
	{
		id: IDMMEname, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *MMEConfigurationUpdate, raw []byte, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = perIEDecode(raw, &n); err == nil {
				name := string(n)
				m.MMEName = &name
			}

			return err
		},
		encode: func(m *MMEConfigurationUpdate) (per.Marshaler, bool) {
			if m.MMEName == nil {
				return nil, false
			}

			return Name(*m.MMEName), true
		},
	},
	{
		id: IDServedGUMMEIs, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *MMEConfigurationUpdate, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ServedGUMMEIs)
		},
		encode: func(m *MMEConfigurationUpdate) (per.Marshaler, bool) {
			if len(m.ServedGUMMEIs) == 0 {
				return nil, false
			}

			return &m.ServedGUMMEIs, true
		},
	},
	{
		id: IDRelativeMMECapacity, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *MMEConfigurationUpdate, raw []byte, enc per.Encoding) error {
			v, err := per.DecodeInteger(per.NewReader(raw), enc, relativeMMECapacityBounds)
			if err != nil {
				return err
			}

			c := uint8(v)
			m.RelativeMMECapacity = &c

			return nil
		},
		encode: func(m *MMEConfigurationUpdate) (per.Marshaler, bool) {
			if m.RelativeMMECapacity == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return per.EncodeInteger(w, enc, relativeMMECapacityBounds, int64(*m.RelativeMMECapacity))
			}), true
		},
	},
}

func (m *MMEConfigurationUpdate) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcMMEConfigurationUpdate, mMEConfigurationUpdateIEs, m)
}

func (m *MMEConfigurationUpdate) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcMMEConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseMMEConfigurationUpdate(value []byte) (*MMEConfigurationUpdate, error) {
	return parseMessageBody[MMEConfigurationUpdate](ProcMMEConfigurationUpdate, TriggeringInitiatingMessage, mMEConfigurationUpdateIEs, value)
}

type MMEConfigurationUpdateAcknowledge struct {
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var mMEConfigurationUpdateAcknowledgeIEs = []ieSpec[MMEConfigurationUpdateAcknowledge]{
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *MMEConfigurationUpdateAcknowledge, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *MMEConfigurationUpdateAcknowledge) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *MMEConfigurationUpdateAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcMMEConfigurationUpdate, mMEConfigurationUpdateAcknowledgeIEs, m)
}

func (m *MMEConfigurationUpdateAcknowledge) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcMMEConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseMMEConfigurationUpdateAcknowledge(value []byte) (*MMEConfigurationUpdateAcknowledge, error) {
	return parseMessageBody[MMEConfigurationUpdateAcknowledge](ProcMMEConfigurationUpdate, TriggeringSuccessfulOutcome, mMEConfigurationUpdateAcknowledgeIEs, value)
}

type MMEConfigurationUpdateFailure struct {
	Cause                  *Cause
	TimeToWait             *TimeToWait
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var mMEConfigurationUpdateFailureIEs = []ieSpec[MMEConfigurationUpdateFailure]{
	{
		id: IDCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *MMEConfigurationUpdateFailure, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *MMEConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: IDTimeToWait, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *MMEConfigurationUpdateFailure, raw []byte, enc per.Encoding) error {
			var ttw TimeToWait

			if err := perIEDecode(raw, &ttw); err != nil {
				return err
			}

			m.TimeToWait = &ttw

			return nil
		},
		encode: func(m *MMEConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.TimeToWait == nil {
				return nil, false
			}

			return m.TimeToWait, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *MMEConfigurationUpdateFailure, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *MMEConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *MMEConfigurationUpdateFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcMMEConfigurationUpdate, mMEConfigurationUpdateFailureIEs, m)
}

func (m *MMEConfigurationUpdateFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcMMEConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseMMEConfigurationUpdateFailure(value []byte) (*MMEConfigurationUpdateFailure, error) {
	return parseMessageBody[MMEConfigurationUpdateFailure](ProcMMEConfigurationUpdate, TriggeringUnsuccessfulOutcome, mMEConfigurationUpdateFailureIEs, value)
}
