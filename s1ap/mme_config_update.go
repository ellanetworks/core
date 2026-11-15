// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

type MMEConfigurationUpdate struct {
	MMEName             *string
	ServedGUMMEIs       ServedGUMMEIs
	RelativeMMECapacity *uint8
	ServedDCNs          *ServedDCNs

	messageMeta
}

var mMEConfigurationUpdateIEs = []ieSpec[MMEConfigurationUpdate]{
	{
		ID: IDMMEname, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *MMEConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = decodeIE(r, &n); err == nil {
				name := string(n)
				m.MMEName = &name
			}

			return err
		},
		Encode: func(m *MMEConfigurationUpdate) (per.Marshaler, bool) {
			if m.MMEName == nil {
				return nil, false
			}

			return Name(*m.MMEName), true
		},
	},
	{
		ID: IDServedGUMMEIs, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *MMEConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ServedGUMMEIs)
		},
		Encode: func(m *MMEConfigurationUpdate) (per.Marshaler, bool) {
			if len(m.ServedGUMMEIs) == 0 {
				return nil, false
			}

			return &m.ServedGUMMEIs, true
		},
	},
	{
		ID: IDRelativeMMECapacity, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *MMEConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			v, err := per.DecodeInteger(r, enc, relativeMMECapacityBounds)
			if err != nil {
				return err
			}

			c := uint8(v)
			m.RelativeMMECapacity = &c

			return nil
		},
		Encode: func(m *MMEConfigurationUpdate) (per.Marshaler, bool) {
			if m.RelativeMMECapacity == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return per.EncodeInteger(w, enc, relativeMMECapacityBounds, int64(*m.RelativeMMECapacity))
			}), true
		},
	},
	apimpl.OptionalIE(IDServedDCNs, CriticalityIgnore, func(m *MMEConfigurationUpdate) **ServedDCNs { return &m.ServedDCNs }),
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

func DecodeMMEConfigurationUpdate(value []byte) (*MMEConfigurationUpdate, error) {
	return decodeMessageBody[MMEConfigurationUpdate](ProcMMEConfigurationUpdate, mMEConfigurationUpdateIEs, value)
}

type MMEConfigurationUpdateAcknowledge struct {
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var mMEConfigurationUpdateAcknowledgeIEs = []ieSpec[MMEConfigurationUpdateAcknowledge]{
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *MMEConfigurationUpdateAcknowledge, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *MMEConfigurationUpdateAcknowledge) (per.Marshaler, bool) {
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

func DecodeMMEConfigurationUpdateAcknowledge(value []byte) (*MMEConfigurationUpdateAcknowledge, error) {
	return decodeMessageBody[MMEConfigurationUpdateAcknowledge](ProcMMEConfigurationUpdate, mMEConfigurationUpdateAcknowledgeIEs, value)
}

type MMEConfigurationUpdateFailure struct {
	Cause                  *Cause
	TimeToWait             *TimeToWait
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var mMEConfigurationUpdateFailureIEs = []ieSpec[MMEConfigurationUpdateFailure]{
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *MMEConfigurationUpdateFailure, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *MMEConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDTimeToWait, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *MMEConfigurationUpdateFailure, r *per.Reader, enc per.Encoding) error {
			var ttw TimeToWait

			if err := decodeIE(r, &ttw); err != nil {
				return err
			}

			m.TimeToWait = &ttw

			return nil
		},
		Encode: func(m *MMEConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.TimeToWait == nil {
				return nil, false
			}

			return m.TimeToWait, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *MMEConfigurationUpdateFailure, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *MMEConfigurationUpdateFailure) (per.Marshaler, bool) {
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

func DecodeMMEConfigurationUpdateFailure(value []byte) (*MMEConfigurationUpdateFailure, error) {
	return decodeMessageBody[MMEConfigurationUpdateFailure](ProcMMEConfigurationUpdate, mMEConfigurationUpdateFailureIEs, value)
}
