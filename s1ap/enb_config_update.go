// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// TS 36.413 §9.1.8.7.
type ENBConfigurationUpdate struct {
	ENBName                    *string
	SupportedTAs               SupportedTAs
	DefaultPagingDRX           *PagingDRX
	CSGIdList                  CSGIdList
	NBIoTDefaultPagingDRX      *NBIoTDefaultPagingDRX
	ConnectedengNBToAddList    ConnectedengNBList
	ConnectedengNBToRemoveList ConnectedengNBList

	messageMeta
}

var eNBConfigurationUpdateIEs = []ieSpec[ENBConfigurationUpdate]{
	{
		ID: IDENBname, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ENBConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = decodeIE(r, &n); err == nil {
				name := string(n)
				m.ENBName = &name
			}

			return err
		},
		Encode: func(m *ENBConfigurationUpdate) (per.Marshaler, bool) {
			if m.ENBName == nil {
				return nil, false
			}

			return Name(*m.ENBName), true
		},
	},
	{
		ID: IDSupportedTAs, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *ENBConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SupportedTAs)
		},
		Encode: func(m *ENBConfigurationUpdate) (per.Marshaler, bool) {
			if len(m.SupportedTAs) == 0 {
				return nil, false
			}

			return &m.SupportedTAs, true
		},
	},
	apimpl.ListIE(IDCSGIdList, CriticalityReject, func(m *ENBConfigurationUpdate) *CSGIdList { return &m.CSGIdList }),
	{
		ID: IDDefaultPagingDRX, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ENBConfigurationUpdate, r *per.Reader, enc per.Encoding) error {
			var drx PagingDRX

			if err := decodeIE(r, &drx); err != nil {
				return err
			}

			m.DefaultPagingDRX = &drx

			return nil
		},
		Encode: func(m *ENBConfigurationUpdate) (per.Marshaler, bool) {
			if m.DefaultPagingDRX == nil {
				return nil, false
			}

			return m.DefaultPagingDRX, true
		},
	},
	apimpl.OptionalIE(IDNBIoTDefaultPagingDRX, CriticalityIgnore, func(m *ENBConfigurationUpdate) **NBIoTDefaultPagingDRX { return &m.NBIoTDefaultPagingDRX }),
	apimpl.ListIE(IDConnectedengNBToAddList, CriticalityIgnore, func(m *ENBConfigurationUpdate) *ConnectedengNBList { return &m.ConnectedengNBToAddList }),
	apimpl.ListIE(IDConnectedengNBToRemoveList, CriticalityIgnore, func(m *ENBConfigurationUpdate) *ConnectedengNBList { return &m.ConnectedengNBToRemoveList }),
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

func DecodeENBConfigurationUpdate(value []byte) (*ENBConfigurationUpdate, error) {
	return decodeMessageBody[ENBConfigurationUpdate](ProcENBConfigurationUpdate, eNBConfigurationUpdateIEs, value)
}

// TS 36.413 §9.1.8.8.
type ENBConfigurationUpdateAcknowledge struct {
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var eNBConfigurationUpdateAcknowledgeIEs = []ieSpec[ENBConfigurationUpdateAcknowledge]{
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ENBConfigurationUpdateAcknowledge, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *ENBConfigurationUpdateAcknowledge) (per.Marshaler, bool) {
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

func DecodeENBConfigurationUpdateAcknowledge(value []byte) (*ENBConfigurationUpdateAcknowledge, error) {
	return decodeMessageBody[ENBConfigurationUpdateAcknowledge](ProcENBConfigurationUpdate, eNBConfigurationUpdateAcknowledgeIEs, value)
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
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *ENBConfigurationUpdateFailure, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *ENBConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDTimeToWait, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ENBConfigurationUpdateFailure, r *per.Reader, enc per.Encoding) error {
			var ttw TimeToWait

			if err := decodeIE(r, &ttw); err != nil {
				return err
			}

			m.TimeToWait = &ttw

			return nil
		},
		Encode: func(m *ENBConfigurationUpdateFailure) (per.Marshaler, bool) {
			if m.TimeToWait == nil {
				return nil, false
			}

			return m.TimeToWait, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ENBConfigurationUpdateFailure, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *ENBConfigurationUpdateFailure) (per.Marshaler, bool) {
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

func DecodeENBConfigurationUpdateFailure(value []byte) (*ENBConfigurationUpdateFailure, error) {
	return decodeMessageBody[ENBConfigurationUpdateFailure](ProcENBConfigurationUpdate, eNBConfigurationUpdateFailureIEs, value)
}
