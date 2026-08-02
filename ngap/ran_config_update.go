// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// TS 38.413 §9.2.6.4.
//
// Every IE is optional: TS 38.413 §8.7.2.2 leaves the corresponding
// configuration unchanged where one is absent, so a name-only update is a
// well-formed message.
type RANConfigurationUpdate struct {
	RANNodeName                     *string
	SupportedTAList                 SupportedTAList
	DefaultPagingDRX                *PagingDRX
	GlobalRANNodeID                 *GlobalRANNodeID
	NGRANTNLAssociationToRemoveList NGRANTNLAssociationToRemoveList

	messageMeta
}

var rANConfigurationUpdateIEs = []ieSpec[RANConfigurationUpdate]{
	{
		id: idRANNodeName, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *RANConfigurationUpdate, raw []byte, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = perIEDecode(raw, &n); err == nil {
				name := string(n)
				m.RANNodeName = &name
			}

			return err
		},
		encode: func(m *RANConfigurationUpdate) (per.Marshaler, bool) {
			if m.RANNodeName == nil {
				return nil, false
			}

			return Name(*m.RANNodeName), true
		},
	},
	{
		id: idSupportedTAList, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *RANConfigurationUpdate, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SupportedTAList)
		},
		encode: func(m *RANConfigurationUpdate) (per.Marshaler, bool) {
			if len(m.SupportedTAList) == 0 {
				return nil, false
			}

			return m.SupportedTAList, true
		},
	},
	{
		id: idDefaultPagingDRX, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *RANConfigurationUpdate, raw []byte, enc per.Encoding) error {
			var (
				err error
				drx PagingDRX
			)

			err = perIEDecode(raw, &drx)
			m.DefaultPagingDRX = &drx

			return err
		},
		encode: func(m *RANConfigurationUpdate) (per.Marshaler, bool) {
			if m.DefaultPagingDRX == nil {
				return nil, false
			}

			return m.DefaultPagingDRX, true
		},
	},
	{
		id: idGlobalRANNodeID, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *RANConfigurationUpdate, raw []byte, enc per.Encoding) error {
			var (
				err error
				v   GlobalRANNodeID
			)

			err = perIEDecode(raw, &v)
			m.GlobalRANNodeID = &v

			return err
		},
		encode: func(m *RANConfigurationUpdate) (per.Marshaler, bool) {
			if m.GlobalRANNodeID == nil {
				return nil, false
			}

			return m.GlobalRANNodeID, true
		},
	},
	{
		// Modeled although Ella Core does not act on it: the IE is marked
		// reject, so a decoder that did not comprehend it would have to reject
		// an otherwise valid update (TS 38.413 §10.3.4.2).
		id: idNGRANTNLAssociationToRemoveList, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *RANConfigurationUpdate, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.NGRANTNLAssociationToRemoveList)
		},
		encode: func(m *RANConfigurationUpdate) (per.Marshaler, bool) {
			if len(m.NGRANTNLAssociationToRemoveList) == 0 {
				return nil, false
			}

			return m.NGRANTNLAssociationToRemoveList, true
		},
	},
}

func (m *RANConfigurationUpdate) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcRANConfigurationUpdate, rANConfigurationUpdateIEs, m)
}

func (m *RANConfigurationUpdate) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcRANConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseRANConfigurationUpdate(value []byte) (*RANConfigurationUpdate, error) {
	return parseMessageBody[RANConfigurationUpdate](ProcRANConfigurationUpdate, TriggeringInitiatingMessage, rANConfigurationUpdateIEs, value)
}

// TS 38.413 §9.2.6.5.
type RANConfigurationUpdateAcknowledge struct {
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var rANConfigurationUpdateAcknowledgeIEs = []ieSpec[RANConfigurationUpdateAcknowledge]{
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *RANConfigurationUpdateAcknowledge, raw []byte, enc per.Encoding) error {
			var (
				err error
				cd  CriticalityDiagnostics
			)

			err = perIEDecode(raw, &cd)
			m.CriticalityDiagnostics = &cd

			return err
		},
		encode: func(m *RANConfigurationUpdateAcknowledge) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *RANConfigurationUpdateAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcRANConfigurationUpdate, rANConfigurationUpdateAcknowledgeIEs, m)
}

func (m *RANConfigurationUpdateAcknowledge) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcRANConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseRANConfigurationUpdateAcknowledge(value []byte) (*RANConfigurationUpdateAcknowledge, error) {
	return parseMessageBody[RANConfigurationUpdateAcknowledge](ProcRANConfigurationUpdate, TriggeringSuccessfulOutcome, rANConfigurationUpdateAcknowledgeIEs, value)
}

// TS 38.413 §9.2.6.6.
type RANConfigurationUpdateFailure struct {
	Cause                  *Cause
	TimeToWait             *TimeToWait
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var rANConfigurationUpdateFailureIEs = []ieSpec[RANConfigurationUpdateFailure]{
	{
		id: idCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *RANConfigurationUpdateFailure, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *RANConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: idTimeToWait, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *RANConfigurationUpdateFailure, raw []byte, enc per.Encoding) error {
			var (
				err error
				ttw TimeToWait
			)

			err = perIEDecode(raw, &ttw)
			m.TimeToWait = &ttw

			return err
		},
		encode: func(m *RANConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.TimeToWait == nil {
				return nil, false
			}

			return m.TimeToWait, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *RANConfigurationUpdateFailure, raw []byte, enc per.Encoding) error {
			var (
				err error
				cd  CriticalityDiagnostics
			)

			err = perIEDecode(raw, &cd)
			m.CriticalityDiagnostics = &cd

			return err
		},
		encode: func(m *RANConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *RANConfigurationUpdateFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcRANConfigurationUpdate, rANConfigurationUpdateFailureIEs, m)
}

func (m *RANConfigurationUpdateFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcRANConfigurationUpdate,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseRANConfigurationUpdateFailure(value []byte) (*RANConfigurationUpdateFailure, error) {
	return parseMessageBody[RANConfigurationUpdateFailure](ProcRANConfigurationUpdate, TriggeringUnsuccessfulOutcome, rANConfigurationUpdateFailureIEs, value)
}
