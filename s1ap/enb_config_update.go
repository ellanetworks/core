// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// ENBConfigurationUpdate is the ENB CONFIGURATION UPDATE message (TS 36.413),
// sent by a running eNB to update its configuration without redoing
// S1 Setup. Every IE is optional; the eNB sends only what changed.
type ENBConfigurationUpdate struct {
	ENBName          *string
	SupportedTAs     SupportedTAs // nil = absent
	DefaultPagingDRX *PagingDRX   // nil = absent

	unmodeledIEs
}

// eNBConfigurationUpdateIEs is the ENBConfigurationUpdate IE table (TS 36.413).
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
	return encodeMessageBody(w, enc, eNBConfigurationUpdateIEs, m)
}

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseENBConfigurationUpdate decodes the message from an initiatingMessage
// open-type payload.
func ParseENBConfigurationUpdate(value []byte) (*ENBConfigurationUpdate, error) {
	return parseMessageBody[ENBConfigurationUpdate](ProcENBConfigurationUpdate, eNBConfigurationUpdateIEs, value)
}

// ENBConfigurationUpdateAcknowledge is the ENB CONFIGURATION UPDATE ACKNOWLEDGE
// message (TS 36.413), the MME's success response.
type ENBConfigurationUpdateAcknowledge struct {
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

// eNBConfigurationUpdateAcknowledgeIEs is the ENBConfigurationUpdateAcknowledge IE table (TS 36.413).
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
	return encodeMessageBody(w, enc, eNBConfigurationUpdateAcknowledgeIEs, m)
}

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseENBConfigurationUpdateAcknowledge decodes the message from a
// successfulOutcome open-type payload.
func ParseENBConfigurationUpdateAcknowledge(value []byte) (*ENBConfigurationUpdateAcknowledge, error) {
	return parseMessageBody[ENBConfigurationUpdateAcknowledge](ProcENBConfigurationUpdate, eNBConfigurationUpdateAcknowledgeIEs, value)
}

// ENBConfigurationUpdateFailure is the ENB CONFIGURATION UPDATE FAILURE message
// (TS 36.413), the MME's rejection (e.g. the updated TAs broadcast no
// served PLMN).
type ENBConfigurationUpdateFailure struct {
	Cause                  Cause
	TimeToWait             *TimeToWait
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

// eNBConfigurationUpdateFailureIEs is the ENBConfigurationUpdateFailure IE table (TS 36.413).
var eNBConfigurationUpdateFailureIEs = []ieSpec[ENBConfigurationUpdateFailure]{
	{
		id: idCause, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *ENBConfigurationUpdateFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.Cause)
		},
		encode: func(m *ENBConfigurationUpdateFailure) (per.Marshaler, bool) { return &m.Cause, true },
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
	return encodeMessageBody(w, enc, eNBConfigurationUpdateFailureIEs, m)
}

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseENBConfigurationUpdateFailure decodes the message from an
// unsuccessfulOutcome open-type payload.
func ParseENBConfigurationUpdateFailure(value []byte) (*ENBConfigurationUpdateFailure, error) {
	return parseMessageBody[ENBConfigurationUpdateFailure](ProcENBConfigurationUpdate, eNBConfigurationUpdateFailureIEs, value)
}
