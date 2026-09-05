// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

type AMFConfigurationUpdate struct {
	AMFName             *string
	ServedGUAMIList     ServedGUAMIList
	RelativeAMFCapacity *uint8

	messageMeta
}

var aMFConfigurationUpdateIEs = []ieSpec[AMFConfigurationUpdate]{
	{
		id: IDAMFName, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *AMFConfigurationUpdate, raw []byte, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = perIEDecode(raw, &n); err == nil {
				name := string(n)
				m.AMFName = &name
			}

			return err
		},
		encode: func(m *AMFConfigurationUpdate) (per.Marshaler, bool) {
			if m.AMFName == nil {
				return nil, false
			}

			return Name(*m.AMFName), true
		},
	},
	{
		id: IDServedGUAMIList, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *AMFConfigurationUpdate, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ServedGUAMIList)
		},
		encode: func(m *AMFConfigurationUpdate) (per.Marshaler, bool) {
			if len(m.ServedGUAMIList) == 0 {
				return nil, false
			}

			return &m.ServedGUAMIList, true
		},
	},
	{
		id: IDRelativeAMFCapacity, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *AMFConfigurationUpdate, raw []byte, enc per.Encoding) error {
			v, err := per.DecodeInteger(per.NewReader(raw), enc, relativeAMFCapacityBounds)
			if err != nil {
				return err
			}

			c := uint8(v)
			m.RelativeAMFCapacity = &c

			return nil
		},
		encode: func(m *AMFConfigurationUpdate) (per.Marshaler, bool) {
			if m.RelativeAMFCapacity == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return per.EncodeInteger(w, enc, relativeAMFCapacityBounds, int64(*m.RelativeAMFCapacity))
			}), true
		},
	},
}

func (m *AMFConfigurationUpdate) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcAMFConfigurationUpdate, aMFConfigurationUpdateIEs, m)
}

func (m *AMFConfigurationUpdate) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcAMFConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseAMFConfigurationUpdate(value []byte) (*AMFConfigurationUpdate, error) {
	return parseMessageBody[AMFConfigurationUpdate](ProcAMFConfigurationUpdate, TriggeringInitiatingMessage, aMFConfigurationUpdateIEs, value)
}

type AMFConfigurationUpdateAcknowledge struct {
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var aMFConfigurationUpdateAcknowledgeIEs = []ieSpec[AMFConfigurationUpdateAcknowledge]{
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *AMFConfigurationUpdateAcknowledge, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *AMFConfigurationUpdateAcknowledge) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *AMFConfigurationUpdateAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcAMFConfigurationUpdate, aMFConfigurationUpdateAcknowledgeIEs, m)
}

func (m *AMFConfigurationUpdateAcknowledge) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcAMFConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseAMFConfigurationUpdateAcknowledge(value []byte) (*AMFConfigurationUpdateAcknowledge, error) {
	return parseMessageBody[AMFConfigurationUpdateAcknowledge](ProcAMFConfigurationUpdate, TriggeringSuccessfulOutcome, aMFConfigurationUpdateAcknowledgeIEs, value)
}

type AMFConfigurationUpdateFailure struct {
	Cause                  *Cause
	TimeToWait             *TimeToWait
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var aMFConfigurationUpdateFailureIEs = []ieSpec[AMFConfigurationUpdateFailure]{
	{
		id: IDCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *AMFConfigurationUpdateFailure, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *AMFConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: IDTimeToWait, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *AMFConfigurationUpdateFailure, raw []byte, enc per.Encoding) error {
			var ttw TimeToWait

			if err := perIEDecode(raw, &ttw); err != nil {
				return err
			}

			m.TimeToWait = &ttw

			return nil
		},
		encode: func(m *AMFConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.TimeToWait == nil {
				return nil, false
			}

			return m.TimeToWait, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *AMFConfigurationUpdateFailure, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *AMFConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *AMFConfigurationUpdateFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcAMFConfigurationUpdate, aMFConfigurationUpdateFailureIEs, m)
}

func (m *AMFConfigurationUpdateFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcAMFConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseAMFConfigurationUpdateFailure(value []byte) (*AMFConfigurationUpdateFailure, error) {
	return parseMessageBody[AMFConfigurationUpdateFailure](ProcAMFConfigurationUpdate, TriggeringUnsuccessfulOutcome, aMFConfigurationUpdateFailureIEs, value)
}
