// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// TS 38.413 §9.2.6.4. Every IE is optional: §8.7.2.2 leaves the corresponding
// configuration unchanged where one is absent.
type RANConfigurationUpdate struct {
	RANNodeName                     *string
	SupportedTAList                 SupportedTAList
	DefaultPagingDRX                *PagingDRX
	GlobalRANNodeID                 *GlobalRANNodeID
	NGRANTNLAssociationToRemoveList NGRANTNLAssociationToRemoveList
	NBIoTDefaultPagingDRX           *NBIoTDefaultPagingDRX
	ExtendedRANNodeName             *ExtendedRANNodeName
	AdditionalULI                   *AdditionalULI
	AIoTSupport                     *AIoTSupport

	messageMeta
}

var rANConfigurationUpdateIEs = []ieSpec[RANConfigurationUpdate]{
	{
		ID: IDRANNodeName, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *RANConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = decodeIE(r, &n); err == nil {
				name := string(n)
				m.RANNodeName = &name
			}

			return err
		},
		Encode: func(m *RANConfigurationUpdate) (per.Marshaler, bool) {
			if m.RANNodeName == nil {
				return nil, false
			}

			return Name(*m.RANNodeName), true
		},
	},
	{
		ID: IDSupportedTAList, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *RANConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SupportedTAList)
		},
		Encode: func(m *RANConfigurationUpdate) (per.Marshaler, bool) {
			if len(m.SupportedTAList) == 0 {
				return nil, false
			}

			return m.SupportedTAList, true
		},
	},
	{
		ID: IDDefaultPagingDRX, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *RANConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			var drx PagingDRX

			if err := decodeIE(r, &drx); err != nil {
				return err
			}

			m.DefaultPagingDRX = &drx

			return nil
		},
		Encode: func(m *RANConfigurationUpdate) (per.Marshaler, bool) {
			if m.DefaultPagingDRX == nil {
				return nil, false
			}

			return m.DefaultPagingDRX, true
		},
	},
	{
		ID: IDGlobalRANNodeID, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *RANConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			var v GlobalRANNodeID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.GlobalRANNodeID = &v

			return nil
		},
		Encode: func(m *RANConfigurationUpdate) (per.Marshaler, bool) {
			if m.GlobalRANNodeID == nil {
				return nil, false
			}

			return m.GlobalRANNodeID, true
		},
	},
	{
		// Modeled but not acted on: the IE is reject, so not comprehending it
		// would reject an otherwise valid update (§10.3.4.2).
		ID: IDNGRANTNLAssociationToRemoveList, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *RANConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.NGRANTNLAssociationToRemoveList)
		},
		Encode: func(m *RANConfigurationUpdate) (per.Marshaler, bool) {
			if len(m.NGRANTNLAssociationToRemoveList) == 0 {
				return nil, false
			}

			return m.NGRANTNLAssociationToRemoveList, true
		},
	},
	optionalExtension(IDNBIoTDefaultPagingDRX, CriticalityIgnore, func(m *RANConfigurationUpdate) **NBIoTDefaultPagingDRX { return &m.NBIoTDefaultPagingDRX }),
	optionalExtension(IDExtendedRANNodeName, CriticalityIgnore, func(m *RANConfigurationUpdate) **ExtendedRANNodeName { return &m.ExtendedRANNodeName }),
	optionalExtension(IDAdditionalULI, CriticalityIgnore, func(m *RANConfigurationUpdate) **AdditionalULI { return &m.AdditionalULI }),
	optionalExtension(IDAIoTSupport, CriticalityReject, func(m *RANConfigurationUpdate) **AIoTSupport { return &m.AIoTSupport }),
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

func DecodeRANConfigurationUpdate(value []byte) (*RANConfigurationUpdate, error) {
	return decodeMessageBody[RANConfigurationUpdate](ProcRANConfigurationUpdate, rANConfigurationUpdateIEs, value)
}

// TS 38.413 §9.2.6.5.
type RANConfigurationUpdateAcknowledge struct {
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var rANConfigurationUpdateAcknowledgeIEs = []ieSpec[RANConfigurationUpdateAcknowledge]{
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *RANConfigurationUpdateAcknowledge, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *RANConfigurationUpdateAcknowledge) (per.Marshaler, bool) {
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

func DecodeRANConfigurationUpdateAcknowledge(value []byte) (*RANConfigurationUpdateAcknowledge, error) {
	return decodeMessageBody[RANConfigurationUpdateAcknowledge](ProcRANConfigurationUpdate, rANConfigurationUpdateAcknowledgeIEs, value)
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
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *RANConfigurationUpdateFailure, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *RANConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDTimeToWait, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *RANConfigurationUpdateFailure, r *per.Reader, enc per.Encoding) error {
			var ttw TimeToWait

			if err := decodeIE(r, &ttw); err != nil {
				return err
			}

			m.TimeToWait = &ttw

			return nil
		},
		Encode: func(m *RANConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.TimeToWait == nil {
				return nil, false
			}

			return m.TimeToWait, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *RANConfigurationUpdateFailure, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *RANConfigurationUpdateFailure) (per.Marshaler, bool) {
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

func DecodeRANConfigurationUpdateFailure(value []byte) (*RANConfigurationUpdateFailure, error) {
	return decodeMessageBody[RANConfigurationUpdateFailure](ProcRANConfigurationUpdate, rANConfigurationUpdateFailureIEs, value)
}
