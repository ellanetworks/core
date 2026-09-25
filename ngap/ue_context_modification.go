// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

type UEContextModificationRequest struct {
	AMFUENGAPID               AMFUENGAPID
	RANUENGAPID               RANUENGAPID
	UEAggregateMaximumBitRate *UEAggregateMaximumBitRate

	messageMeta
}

var uEContextModificationRequestIEs = []ieSpec[UEContextModificationRequest]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UEContextModificationRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *UEContextModificationRequest) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UEContextModificationRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *UEContextModificationRequest) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: IDUEAggregateMaximumBitRate, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *UEContextModificationRequest, raw []byte, enc per.Encoding) error {
			var ambr UEAggregateMaximumBitRate

			if err := perIEDecode(raw, &ambr); err != nil {
				return err
			}

			m.UEAggregateMaximumBitRate = &ambr

			return nil
		},
		encode: func(m *UEContextModificationRequest) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
}

func (m *UEContextModificationRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUEContextModification, uEContextModificationRequestIEs, m)
}

func (m *UEContextModificationRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUEContextModification,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseUEContextModificationRequest(value []byte) (*UEContextModificationRequest, error) {
	return parseMessageBody[UEContextModificationRequest](ProcUEContextModification, TriggeringInitiatingMessage, uEContextModificationRequestIEs, value)
}

type UEContextModificationResponse struct {
	AMFUENGAPID            *AMFUENGAPID
	RANUENGAPID            *RANUENGAPID
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var uEContextModificationResponseIEs = []ieSpec[UEContextModificationResponse]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UEContextModificationResponse, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *UEContextModificationResponse) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UEContextModificationResponse, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *UEContextModificationResponse) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *UEContextModificationResponse, raw []byte, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		encode: func(m *UEContextModificationResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *UEContextModificationResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUEContextModification, uEContextModificationResponseIEs, m)
}

func (m *UEContextModificationResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcUEContextModification,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseUEContextModificationResponse(value []byte) (*UEContextModificationResponse, error) {
	return parseMessageBody[UEContextModificationResponse](ProcUEContextModification, TriggeringSuccessfulOutcome, uEContextModificationResponseIEs, value)
}

type UEContextModificationFailure struct {
	AMFUENGAPID            *AMFUENGAPID
	RANUENGAPID            *RANUENGAPID
	Cause                  *Cause
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var uEContextModificationFailureIEs = []ieSpec[UEContextModificationFailure]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UEContextModificationFailure, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *UEContextModificationFailure) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UEContextModificationFailure, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *UEContextModificationFailure) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: IDCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UEContextModificationFailure, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *UEContextModificationFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *UEContextModificationFailure, raw []byte, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		encode: func(m *UEContextModificationFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *UEContextModificationFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUEContextModification, uEContextModificationFailureIEs, m)
}

func (m *UEContextModificationFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcUEContextModification,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseUEContextModificationFailure(value []byte) (*UEContextModificationFailure, error) {
	return parseMessageBody[UEContextModificationFailure](ProcUEContextModification, TriggeringUnsuccessfulOutcome, uEContextModificationFailureIEs, value)
}
