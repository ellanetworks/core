// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// ERABToBeModifiedItemBearerModReq ::= SEQUENCE { e-RAB-ID,
// e-RABLevelQoSParameters, nAS-PDU, iE-Extensions OPTIONAL } (extensible).
// The NAS-PDU carries the MODIFY EPS BEARER CONTEXT REQUEST (TS 36.413).
type ERABToBeModifiedItemBearerModReq struct {
	ERABID               ERABID
	QoS                  ERABLevelQoSParameters
	NASPDU               NASPDU
	TransportInformation *TransportInformation  `per:",extie:IDTransportInformation,crit:reject"`
	iEExtensions         extensionMeta          `per:",exttable"`
	extensions           per.ExtensionAdditions `per:",extraw"`
	containerCrit        `per:"-"`
}

type TransportInformation struct {
	TransportLayerAddress TransportLayerAddress
	ULGTPTEID             GTPTEID
	extensions            per.ExtensionAdditions `per:",extraw"`
}

// ERABModifyItemBearerModRes ::= SEQUENCE { e-RAB-ID, iE-Extensions OPTIONAL }
// (extensible) (TS 36.413).
type ERABModifyItemBearerModRes struct {
	extensions    per.ExtensionAdditions `per:",extraw"`
	ERABID        ERABID
	iEExtensions  extensionMeta `per:",exttable"`
	containerCrit `per:"-"`
}

// TS 36.413 §9.1.3.3.
type ERABModifyRequest struct {
	MMEUES1APID                  MMEUES1APID
	ENBUES1APID                  ENBUES1APID
	UEAggregateMaximumBitRate    *UEAggregateMaximumBitRate
	ERABToBeModified             ERABToBeModifiedListBearerModReq
	SecondaryRATDataUsageRequest *SecondaryRATDataUsageRequest

	messageMeta
}

var eRABModifyRequestIEs = []ieSpec[ERABModifyRequest]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ERABModifyRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *ERABModifyRequest) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ERABModifyRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *ERABModifyRequest) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDUEAggregateMaximumBitrate, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *ERABModifyRequest, r *per.Reader, enc per.Encoding) error {
			var (
				err  error
				ambr UEAggregateMaximumBitRate
			)

			err = decodeIE(r, &ambr)
			if err != nil {
				return err
			}

			m.UEAggregateMaximumBitRate = &ambr

			return nil
		},
		Encode: func(m *ERABModifyRequest) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
	{
		ID: IDERABToBeModifiedListBearerModReq, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ERABModifyRequest, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABToBeModified, err = decodeItemList[ERABToBeModifiedItemBearerModReq](r, enc, maxnoofERABs, IDERABToBeModifiedItemBearerModReq, CriticalityReject)

			return err
		},
		Encode: func(m *ERABModifyRequest) (per.Marshaler, bool) {
			return containerList(maxnoofERABs, IDERABToBeModifiedItemBearerModReq, CriticalityReject, &m.ERABToBeModified), true
		},
	},
	apimpl.OptionalIE(IDSecondaryRATDataUsageRequest, CriticalityIgnore, func(m *ERABModifyRequest) **SecondaryRATDataUsageRequest { return &m.SecondaryRATDataUsageRequest }),
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

func DecodeERABModifyRequest(value []byte) (*ERABModifyRequest, error) {
	return decodeMessageBody[ERABModifyRequest](ProcERABModify, eRABModifyRequestIEs, value)
}

// TS 36.413 §9.1.3.4.
type ERABModifyResponse struct {
	MMEUES1APID                     *MMEUES1APID
	ENBUES1APID                     *ENBUES1APID
	ERABModify                      []ERABModifyItemBearerModRes
	ERABFailedToModify              []ERABItem
	CriticalityDiagnostics          *CriticalityDiagnostics
	UserLocationInformation         *UserLocationInformation
	SecondaryRATDataUsageReportList SecondaryRATDataUsageReportList

	messageMeta
}

var eRABModifyResponseIEs = []ieSpec[ERABModifyResponse]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *ERABModifyResponse, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		Encode: func(m *ERABModifyResponse) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *ERABModifyResponse, r *per.Reader, enc per.Encoding) error {
			var v ENBUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		Encode: func(m *ERABModifyResponse) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		ID: IDERABModifyListBearerModRes, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABModifyResponse, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABModify, err = decodeItemList[ERABModifyItemBearerModRes](r, enc, maxnoofERABs, IDERABModifyItemBearerModRes, CriticalityIgnore)

			return err
		},
		Encode: func(m *ERABModifyResponse) (per.Marshaler, bool) {
			if len(m.ERABModify) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABModifyItemBearerModRes, CriticalityIgnore, &m.ERABModify), true
		},
	},
	{
		ID: IDERABFailedToModifyList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABModifyResponse, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABFailedToModify, err = decodeItemList[ERABItem](r, enc, maxnoofERABs, IDERABItem, CriticalityIgnore)

			return err
		},
		Encode: func(m *ERABModifyResponse) (per.Marshaler, bool) {
			if len(m.ERABFailedToModify) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABItem, CriticalityIgnore, &m.ERABFailedToModify), true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABModifyResponse, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *ERABModifyResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	apimpl.ListIE(IDSecondaryRATDataUsageReportList, CriticalityIgnore, func(m *ERABModifyResponse) *SecondaryRATDataUsageReportList {
		return &m.SecondaryRATDataUsageReportList
	}),
	{
		ID: IDUserLocationInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABModifyResponse, r *per.Reader, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := decodeIE(r, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		Encode: func(m *ERABModifyResponse) (per.Marshaler, bool) {
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

func DecodeERABModifyResponse(value []byte) (*ERABModifyResponse, error) {
	return decodeMessageBody[ERABModifyResponse](ProcERABModify, eRABModifyResponseIEs, value)
}
