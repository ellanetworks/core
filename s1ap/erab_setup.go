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
	CorrelationID         *CorrelationID         `per:"-"`
	SIPTOCorrelationID    *CorrelationID         `per:"-"`
	BearerType            *BearerType            `per:"-"`
	EthernetType          *EthernetType          `per:"-"`
	SecurityIndication    *SecurityIndication    `per:"-"`
	iEExtensions          messageMeta            `per:",exttable:eRABToBeSetupItemBearerSUReqExtIEs"`
	extensions            per.ExtensionAdditions `per:",extraw"`
	containerCrit         `per:"-"`
}

var eRABToBeSetupItemBearerSUReqExtIEs = []ieSpec[ERABToBeSetupItemBearerSUReq]{
	optionalExtension(IDCorrelationID, CriticalityIgnore, func(m *ERABToBeSetupItemBearerSUReq) **CorrelationID {
		return &m.CorrelationID
	}),
	optionalExtension(IDSIPTOCorrelationID, CriticalityIgnore, func(m *ERABToBeSetupItemBearerSUReq) **CorrelationID {
		return &m.SIPTOCorrelationID
	}),
	optionalExtension(IDBearerType, CriticalityReject, func(m *ERABToBeSetupItemBearerSUReq) **BearerType {
		return &m.BearerType
	}),
	optionalExtension(IDEthernetType, CriticalityIgnore, func(m *ERABToBeSetupItemBearerSUReq) **EthernetType {
		return &m.EthernetType
	}),
	optionalExtension(IDSecurityIndication, CriticalityReject, func(m *ERABToBeSetupItemBearerSUReq) **SecurityIndication {
		return &m.SecurityIndication
	}),
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
		id: IDMMEUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *ERABSetupRequest) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: IDENBUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *ERABSetupRequest) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: IDUEAggregateMaximumBitrate, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *ERABSetupRequest, raw []byte, enc per.Encoding) error {
			var (
				err  error
				ambr UEAggregateMaximumBitRate
			)

			err = perIEDecode(raw, &ambr)
			if err != nil {
				return err
			}

			m.UEAggregateMaximumBitRate = &ambr

			return nil
		},
		encode: func(m *ERABSetupRequest) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
	{
		id: IDERABToBeSetupListBearerSUReq, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABSetupRequest, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABToBeSetup, err = decodeItemList[ERABToBeSetupItemBearerSUReq](per.NewReader(raw), enc, maxnoofERABs, IDERABToBeSetupItemBearerSUReq, CriticalityReject)

			return err
		},
		encode: func(m *ERABSetupRequest) (per.Marshaler, bool) {
			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABToBeSetupItemBearerSUReq, CriticalityReject, m.ERABToBeSetup)
			}), true
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
		id: IDMMEUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *ERABSetupResponse, raw []byte, enc per.Encoding) error {
			var v MMEUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		encode: func(m *ERABSetupResponse) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		id: IDENBUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *ERABSetupResponse, raw []byte, enc per.Encoding) error {
			var v ENBUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		encode: func(m *ERABSetupResponse) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		id: IDERABSetupListBearerSURes, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABSetupResponse, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABSetup, err = decodeItemList[ERABSetupItemBearerSURes](per.NewReader(raw), enc, maxnoofERABs, IDERABSetupItemBearerSURes, CriticalityIgnore)

			return err
		},
		encode: func(m *ERABSetupResponse) (per.Marshaler, bool) {
			if len(m.ERABSetup) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABSetupItemBearerSURes, CriticalityIgnore, m.ERABSetup)
			}), true
		},
	},
	{
		id: IDERABFailedToSetupListBearerSURes, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABSetupResponse, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABFailedToSetup, err = decodeItemList[ERABItem](per.NewReader(raw), enc, maxnoofERABs, IDERABItem, CriticalityIgnore)

			return err
		},
		encode: func(m *ERABSetupResponse) (per.Marshaler, bool) {
			if len(m.ERABFailedToSetup) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABItem, CriticalityIgnore, m.ERABFailedToSetup)
			}), true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABSetupResponse, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *ERABSetupResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		id: IDUserLocationInformation, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABSetupResponse, raw []byte, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := perIEDecode(raw, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		encode: func(m *ERABSetupResponse) (per.Marshaler, bool) {
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
