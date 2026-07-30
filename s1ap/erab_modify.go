// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// ERABToBeModifiedItemBearerModReq ::= SEQUENCE { e-RAB-ID,
// e-RABLevelQoSParameters, nAS-PDU, iE-Extensions OPTIONAL } (extensible).
// The NAS-PDU carries the MODIFY EPS BEARER CONTEXT REQUEST (TS 36.413).
type ERABToBeModifiedItemBearerModReq struct {
	_      [0]struct{} `per:"extseq"`
	ERABID ERABID
	QoS    ERABLevelQoSParameters
	NASPDU NASPDU
	_      ieExtensions `per:",skip"`
}

// ERABModifyItemBearerModRes ::= SEQUENCE { e-RAB-ID, iE-Extensions OPTIONAL }
// (extensible) (TS 36.413).
type ERABModifyItemBearerModRes struct {
	_      [0]struct{} `per:"extseq"`
	ERABID ERABID
	_      ieExtensions `per:",skip"`
}

// TS 36.413 §9.1.3.3.
type ERABModifyRequest struct {
	MMEUES1APID               MMEUES1APID
	ENBUES1APID               ENBUES1APID
	UEAggregateMaximumBitRate *UEAggregateMaximumBitRate
	ERABToBeModified          []ERABToBeModifiedItemBearerModReq

	messageMeta
}

var eRABModifyRequestIEs = []ieSpec[ERABModifyRequest]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABModifyRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *ERABModifyRequest) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABModifyRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *ERABModifyRequest) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idUEAggregateMaximumBitrate, presence: PresenceOptional, crit: CriticalityReject,
		decode: func(m *ERABModifyRequest, raw []byte, enc per.Encoding) error {
			var (
				err  error
				ambr UEAggregateMaximumBitRate
			)

			err = perIEDecode(raw, &ambr)
			m.UEAggregateMaximumBitRate = &ambr

			return err
		},
		encode: func(m *ERABModifyRequest) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
	{
		id: idERABToBeModifiedListBearerModReq, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABModifyRequest, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABToBeModified, err = decodeItemList[ERABToBeModifiedItemBearerModReq](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *ERABModifyRequest) (per.Marshaler, bool) {
			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABToBeModifiedItemBearerModReq, CriticalityReject, m.ERABToBeModified)
			}), true
		},
	},
}

func (m *ERABModifyRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcERABModify, eRABModifyRequestIEs, m)
}

func (m *ERABModifyRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcERABModify,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseERABModifyRequest(value []byte) (*ERABModifyRequest, error) {
	return parseMessageBody[ERABModifyRequest](ProcERABModify, TriggeringInitiatingMessage, eRABModifyRequestIEs, value)
}

// TS 36.413 §9.1.3.4.
type ERABModifyResponse struct {
	MMEUES1APID             *MMEUES1APID
	ENBUES1APID             *ENBUES1APID
	ERABModify              []ERABModifyItemBearerModRes
	ERABFailedToModify      []ERABItem
	CriticalityDiagnostics  *CriticalityDiagnostics
	UserLocationInformation *UserLocationInformation

	messageMeta
}

var eRABModifyResponseIEs = []ieSpec[ERABModifyResponse]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *ERABModifyResponse, raw []byte, enc per.Encoding) error {
			var v MMEUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		encode: func(m *ERABModifyResponse) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *ERABModifyResponse, raw []byte, enc per.Encoding) error {
			var v ENBUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		encode: func(m *ERABModifyResponse) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		id: idERABModifyListBearerModRes, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABModifyResponse, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABModify, err = decodeItemList[ERABModifyItemBearerModRes](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *ERABModifyResponse) (per.Marshaler, bool) {
			if len(m.ERABModify) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABModifyItemBearerModRes, CriticalityIgnore, m.ERABModify)
			}), true
		},
	},
	{
		id: idERABFailedToModifyList, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABModifyResponse, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABFailedToModify, err = decodeItemList[ERABItem](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *ERABModifyResponse) (per.Marshaler, bool) {
			if len(m.ERABFailedToModify) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABFailedToModify)
			}), true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABModifyResponse, raw []byte, enc per.Encoding) error {
			var (
				err error
				cd  CriticalityDiagnostics
			)

			err = perIEDecode(raw, &cd)
			m.CriticalityDiagnostics = &cd

			return err
		},
		encode: func(m *ERABModifyResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		id: idUserLocationInformation, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABModifyResponse, raw []byte, enc per.Encoding) error {
			var (
				err error
				uli UserLocationInformation
			)

			err = perIEDecode(raw, &uli)
			m.UserLocationInformation = &uli

			return err
		},
		encode: func(m *ERABModifyResponse) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
}

func (m *ERABModifyResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcERABModify, eRABModifyResponseIEs, m)
}

func (m *ERABModifyResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcERABModify,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseERABModifyResponse(value []byte) (*ERABModifyResponse, error) {
	return parseMessageBody[ERABModifyResponse](ProcERABModify, TriggeringSuccessfulOutcome, eRABModifyResponseIEs, value)
}
