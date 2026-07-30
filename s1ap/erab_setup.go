// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// ERABToBeSetupItemBearerSUReq ::= SEQUENCE { e-RAB-ID, e-RABlevelQoSParameters,
// transportLayerAddress, gTP-TEID, nAS-PDU, iE-Extensions OPTIONAL }
// (extensible). The NAS-PDU is mandatory:
// the E-RAB Setup carries the ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST for an
// additional PDN connection (TS 36.413).
type ERABToBeSetupItemBearerSUReq struct {
	_                     [0]struct{} `per:"extseq"`
	ERABID                ERABID
	QoS                   ERABLevelQoSParameters
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	NASPDU                NASPDU
	_                     ieExtensions `per:",skip"`
}

// ERABSetupItemBearerSURes has the same structure as ERABSetupItemCtxtSURes
// (e-RAB-ID, transportLayerAddress, gTP-TEID): the eNB endpoint the UPF sends
// downlink traffic to (TS 36.413). The two decode identically.
type ERABSetupItemBearerSURes = ERABSetupItemCtxtSURes

// TS 36.413 §9.1.3.1.
type ERABSetupRequest struct {
	MMEUES1APID               MMEUES1APID
	ENBUES1APID               ENBUES1APID
	UEAggregateMaximumBitRate *UEAggregateMaximumBitRate
	ERABToBeSetup             []ERABToBeSetupItemBearerSUReq

	messageMeta
}

var eRABSetupRequestIEs = []ieSpec[ERABSetupRequest]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *ERABSetupRequest) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *ERABSetupRequest) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idUEAggregateMaximumBitrate, presence: PresenceOptional, crit: CriticalityReject,
		decode: func(m *ERABSetupRequest, raw []byte, enc per.Encoding) error {
			var (
				err  error
				ambr UEAggregateMaximumBitRate
			)

			err = perIEDecode(raw, &ambr)
			m.UEAggregateMaximumBitRate = &ambr

			return err
		},
		encode: func(m *ERABSetupRequest) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
	{
		id: idERABToBeSetupListBearerSUReq, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABSetupRequest, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABToBeSetup, err = decodeItemList[ERABToBeSetupItemBearerSUReq](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *ERABSetupRequest) (per.Marshaler, bool) {
			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABToBeSetupItemBearerSUReq, CriticalityReject, m.ERABToBeSetup)
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
	ERABSetup               []ERABSetupItemBearerSURes
	ERABFailedToSetup       []ERABItem
	CriticalityDiagnostics  *CriticalityDiagnostics
	UserLocationInformation *UserLocationInformation

	messageMeta
}

var eRABSetupResponseIEs = []ieSpec[ERABSetupResponse]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
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
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
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
		id: idERABSetupListBearerSURes, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABSetupResponse, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABSetup, err = decodeItemList[ERABSetupItemBearerSURes](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *ERABSetupResponse) (per.Marshaler, bool) {
			if len(m.ERABSetup) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABSetupItemBearerSURes, CriticalityIgnore, m.ERABSetup)
			}), true
		},
	},
	{
		id: idERABFailedToSetupListBearerSURes, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABSetupResponse, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABFailedToSetup, err = decodeItemList[ERABItem](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *ERABSetupResponse) (per.Marshaler, bool) {
			if len(m.ERABFailedToSetup) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABFailedToSetup)
			}), true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABSetupResponse, raw []byte, enc per.Encoding) error {
			var (
				err error
				cd  CriticalityDiagnostics
			)

			err = perIEDecode(raw, &cd)
			m.CriticalityDiagnostics = &cd

			return err
		},
		encode: func(m *ERABSetupResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		id: idUserLocationInformation, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABSetupResponse, raw []byte, enc per.Encoding) error {
			var (
				err error
				uli UserLocationInformation
			)

			err = perIEDecode(raw, &uli)
			m.UserLocationInformation = &uli

			return err
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
