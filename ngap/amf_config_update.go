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
	PLMNSupportList     PLMNSupportList

	AMFTNLAssociationToAddList    AMFTNLAssociationToAddList
	AMFTNLAssociationToRemoveList AMFTNLAssociationToRemoveList
	AMFTNLAssociationToUpdateList AMFTNLAssociationToUpdateList
	ExtendedAMFName               *ExtendedAMFName

	messageMeta
}

var aMFConfigurationUpdateIEs = []ieSpec[AMFConfigurationUpdate]{
	{
		ID: IDAMFName, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *AMFConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = decodeIE(r, &n); err == nil {
				name := string(n)
				m.AMFName = &name
			}

			return err
		},
		Encode: func(m *AMFConfigurationUpdate) (per.Marshaler, bool) {
			if m.AMFName == nil {
				return nil, false
			}

			return Name(*m.AMFName), true
		},
	},
	{
		ID: IDServedGUAMIList, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *AMFConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ServedGUAMIList)
		},
		Encode: func(m *AMFConfigurationUpdate) (per.Marshaler, bool) {
			if len(m.ServedGUAMIList) == 0 {
				return nil, false
			}

			return &m.ServedGUAMIList, true
		},
	},
	{
		ID: IDRelativeAMFCapacity, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *AMFConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			v, err := per.DecodeInteger(r, enc, relativeAMFCapacityBounds)
			if err != nil {
				return err
			}

			c := uint8(v)
			m.RelativeAMFCapacity = &c

			return nil
		},
		Encode: func(m *AMFConfigurationUpdate) (per.Marshaler, bool) {
			if m.RelativeAMFCapacity == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return per.EncodeInteger(w, enc, relativeAMFCapacityBounds, int64(*m.RelativeAMFCapacity))
			}), true
		},
	},
	{
		ID: IDPLMNSupportList, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *AMFConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PLMNSupportList)
		},
		Encode: func(m *AMFConfigurationUpdate) (per.Marshaler, bool) {
			if len(m.PLMNSupportList) == 0 {
				return nil, false
			}

			return m.PLMNSupportList, true
		},
	},
	{
		ID: IDAMFTNLAssociationToAddList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *AMFConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFTNLAssociationToAddList)
		},
		Encode: func(m *AMFConfigurationUpdate) (per.Marshaler, bool) {
			if len(m.AMFTNLAssociationToAddList) == 0 {
				return nil, false
			}

			return m.AMFTNLAssociationToAddList, true
		},
	},
	{
		ID: IDAMFTNLAssociationToRemoveList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *AMFConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFTNLAssociationToRemoveList)
		},
		Encode: func(m *AMFConfigurationUpdate) (per.Marshaler, bool) {
			if len(m.AMFTNLAssociationToRemoveList) == 0 {
				return nil, false
			}

			return m.AMFTNLAssociationToRemoveList, true
		},
	},
	{
		ID: IDAMFTNLAssociationToUpdateList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *AMFConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFTNLAssociationToUpdateList)
		},
		Encode: func(m *AMFConfigurationUpdate) (per.Marshaler, bool) {
			if len(m.AMFTNLAssociationToUpdateList) == 0 {
				return nil, false
			}

			return m.AMFTNLAssociationToUpdateList, true
		},
	},
	{
		ID: IDExtendedAMFName, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *AMFConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			var v ExtendedAMFName

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ExtendedAMFName = &v

			return nil
		},
		Encode: func(m *AMFConfigurationUpdate) (per.Marshaler, bool) {
			if m.ExtendedAMFName == nil {
				return nil, false
			}

			return m.ExtendedAMFName, true
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

func DecodeAMFConfigurationUpdate(value []byte) (*AMFConfigurationUpdate, error) {
	return decodeMessageBody[AMFConfigurationUpdate](ProcAMFConfigurationUpdate, aMFConfigurationUpdateIEs, value)
}

type AMFConfigurationUpdateAcknowledge struct {
	AMFTNLAssociationSetupList         AMFTNLAssociationSetupList
	AMFTNLAssociationFailedToSetupList TNLAssociationList
	CriticalityDiagnostics             *CriticalityDiagnostics

	messageMeta
}

var aMFConfigurationUpdateAcknowledgeIEs = []ieSpec[AMFConfigurationUpdateAcknowledge]{
	{
		ID: IDAMFTNLAssociationSetupList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *AMFConfigurationUpdateAcknowledge, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFTNLAssociationSetupList)
		},
		Encode: func(m *AMFConfigurationUpdateAcknowledge) (per.Marshaler, bool) {
			if len(m.AMFTNLAssociationSetupList) == 0 {
				return nil, false
			}

			return m.AMFTNLAssociationSetupList, true
		},
	},
	{
		ID: IDAMFTNLAssociationFailedToSetupList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *AMFConfigurationUpdateAcknowledge, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFTNLAssociationFailedToSetupList)
		},
		Encode: func(m *AMFConfigurationUpdateAcknowledge) (per.Marshaler, bool) {
			if len(m.AMFTNLAssociationFailedToSetupList) == 0 {
				return nil, false
			}

			return m.AMFTNLAssociationFailedToSetupList, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *AMFConfigurationUpdateAcknowledge, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *AMFConfigurationUpdateAcknowledge) (per.Marshaler, bool) {
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

func DecodeAMFConfigurationUpdateAcknowledge(value []byte) (*AMFConfigurationUpdateAcknowledge, error) {
	return decodeMessageBody[AMFConfigurationUpdateAcknowledge](ProcAMFConfigurationUpdate, aMFConfigurationUpdateAcknowledgeIEs, value)
}

type AMFConfigurationUpdateFailure struct {
	Cause                  *Cause
	TimeToWait             *TimeToWait
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var aMFConfigurationUpdateFailureIEs = []ieSpec[AMFConfigurationUpdateFailure]{
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *AMFConfigurationUpdateFailure, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *AMFConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDTimeToWait, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *AMFConfigurationUpdateFailure, r *per.Reader, enc per.Encoding) error {
			var ttw TimeToWait

			if err := decodeIE(r, &ttw); err != nil {
				return err
			}

			m.TimeToWait = &ttw

			return nil
		},
		Encode: func(m *AMFConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.TimeToWait == nil {
				return nil, false
			}

			return m.TimeToWait, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *AMFConfigurationUpdateFailure, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *AMFConfigurationUpdateFailure) (per.Marshaler, bool) {
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

func DecodeAMFConfigurationUpdateFailure(value []byte) (*AMFConfigurationUpdateFailure, error) {
	return decodeMessageBody[AMFConfigurationUpdateFailure](ProcAMFConfigurationUpdate, aMFConfigurationUpdateFailureIEs, value)
}
