// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// ERABToBeSetupItemBearerSUReq ::= SEQUENCE { e-RAB-ID, e-RABlevelQoSParameters,
// transportLayerAddress, gTP-TEID, nAS-PDU, iE-Extensions OPTIONAL }
// (extensible). The NAS-PDU is mandatory: it carries the ACTIVATE DEFAULT EPS
// BEARER CONTEXT REQUEST for an additional PDN connection.
type ERABToBeSetupItemBearerSUReq struct {
	ERABID                ERABID
	QoS                   ERABLevelQoSParameters
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	NASPDU                NASPDU
	CorrelationID         *CorrelationID         `per:",extie:IDCorrelationID,crit:ignore"`
	SIPTOCorrelationID    *CorrelationID         `per:",extie:IDSIPTOCorrelationID,crit:ignore"`
	BearerType            *BearerType            `per:",extie:IDBearerType,crit:reject"`
	EthernetType          *EthernetType          `per:",extie:IDEthernetType,crit:ignore"`
	SecurityIndication    *SecurityIndication    `per:",extie:IDSecurityIndication,crit:reject"`
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
	containerCrit         `per:"-"`
}

// ERABSetupItemBearerSURes ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// gTP-TEID }: the eNB endpoint the UPF sends downlink traffic to.
type ERABSetupItemBearerSURes = ERABSetupItemCtxtSURes

// TS 36.413 §9.1.3.1.
type ERABSetupRequest struct {
	MMEUES1APID               MMEUES1APID
	ENBUES1APID               ENBUES1APID
	UEAggregateMaximumBitRate *UEAggregateMaximumBitRate
	ERABToBeSetup             ERABToBeSetupListBearerSUReq

	messageMeta
}

var eRABSetupRequestIEs = []ieSpec[ERABSetupRequest]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ERABSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *ERABSetupRequest) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ERABSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *ERABSetupRequest) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDUEAggregateMaximumBitrate, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *ERABSetupRequest, r *per.Reader, enc per.Encoding) error {
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
		Encode: func(m *ERABSetupRequest) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
	{
		ID: IDERABToBeSetupListBearerSUReq, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ERABSetupRequest, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABToBeSetup, err = decodeItemList[ERABToBeSetupItemBearerSUReq](r, enc, maxnoofERABs, IDERABToBeSetupItemBearerSUReq, CriticalityReject)

			return err
		},
		Encode: func(m *ERABSetupRequest) (per.Marshaler, bool) {
			return containerList(maxnoofERABs, IDERABToBeSetupItemBearerSUReq, CriticalityReject, m.ERABToBeSetup), true
		},
	},
}

func (m *ERABSetupRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcERABSetup, eRABSetupRequestIEs, m)
}

func (m *ERABSetupRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcERABSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseERABSetupRequest(value []byte) (*ERABSetupRequest, error) {
	return parseMessageBody[ERABSetupRequest](ProcERABSetup, TriggeringInitiatingMessage, eRABSetupRequestIEs, value)
}

func DecodeERABSetupRequest(value []byte) (*ERABSetupRequest, error) {
	return decodeMessageBody[ERABSetupRequest](ProcERABSetup, eRABSetupRequestIEs, value)
}

// TS 36.413 §9.1.3.2.
type ERABSetupResponse struct {
	MMEUES1APID             *MMEUES1APID
	ENBUES1APID             *ENBUES1APID
	ERABSetup               ERABSetupListBearerSURes
	ERABFailedToSetup       []ERABItem
	CriticalityDiagnostics  *CriticalityDiagnostics
	UserLocationInformation *UserLocationInformation

	messageMeta
}

var eRABSetupResponseIEs = []ieSpec[ERABSetupResponse]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *ERABSetupResponse, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		Encode: func(m *ERABSetupResponse) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *ERABSetupResponse, r *per.Reader, enc per.Encoding) error {
			var v ENBUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		Encode: func(m *ERABSetupResponse) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		ID: IDERABSetupListBearerSURes, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABSetupResponse, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABSetup, err = decodeItemList[ERABSetupItemBearerSURes](r, enc, maxnoofERABs, IDERABSetupItemBearerSURes, CriticalityIgnore)

			return err
		},
		Encode: func(m *ERABSetupResponse) (per.Marshaler, bool) {
			if len(m.ERABSetup) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABSetupItemBearerSURes, CriticalityIgnore, m.ERABSetup), true
		},
	},
	{
		ID: IDERABFailedToSetupListBearerSURes, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABSetupResponse, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABFailedToSetup, err = decodeItemList[ERABItem](r, enc, maxnoofERABs, IDERABItem, CriticalityIgnore)

			return err
		},
		Encode: func(m *ERABSetupResponse) (per.Marshaler, bool) {
			if len(m.ERABFailedToSetup) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABItem, CriticalityIgnore, m.ERABFailedToSetup), true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABSetupResponse, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *ERABSetupResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		ID: IDUserLocationInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABSetupResponse, r *per.Reader, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := decodeIE(r, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		Encode: func(m *ERABSetupResponse) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
}

func (m *ERABSetupResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcERABSetup, eRABSetupResponseIEs, m)
}

func (m *ERABSetupResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcERABSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseERABSetupResponse(value []byte) (*ERABSetupResponse, error) {
	return parseMessageBody[ERABSetupResponse](ProcERABSetup, TriggeringSuccessfulOutcome, eRABSetupResponseIEs, value)
}

func DecodeERABSetupResponse(value []byte) (*ERABSetupResponse, error) {
	return decodeMessageBody[ERABSetupResponse](ProcERABSetup, eRABSetupResponseIEs, value)
}
