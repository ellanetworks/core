// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// ERABToBeSwitchedDLItem ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// gTP-TEID, iE-Extensions OPTIONAL } (extensible). The target eNB's S1-U
// downlink endpoint the GTP tunnel is switched to.
type ERABToBeSwitchedDLItem struct {
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	SecurityIndication    *SecurityIndication    `per:",extie:IDSecurityIndication,crit:ignore"`
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
	containerCrit         `per:"-"`
}

// TS 36.413 §9.1.5.8.
type PathSwitchRequest struct {
	ENBUES1APID             ENBUES1APID
	ERABToBeSwitchedDL      ERABToBeSwitchedDLList
	SourceMMEUES1APID       MMEUES1APID
	EUTRANCGI               *EUTRANCGI
	TAI                     *TAI
	UESecurityCapabilities  *UESecurityCapabilities
	SourceMMEGUMMEI         *GUMMEI
	TunnelInformationForBBF *TunnelInformation

	messageMeta
}

var pathSwitchRequestIEs = []ieSpec[PathSwitchRequest]{
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PathSwitchRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *PathSwitchRequest) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDERABToBeSwitchedDLList, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PathSwitchRequest, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABToBeSwitchedDL, err = decodeItemList[ERABToBeSwitchedDLItem](r, enc, maxnoofERABs, IDERABToBeSwitchedDLItem, CriticalityReject)

			return err
		},
		Encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			return containerList(maxnoofERABs, IDERABToBeSwitchedDLItem, CriticalityReject, m.ERABToBeSwitchedDL), true
		},
	},
	{
		ID: IDSourceMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PathSwitchRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SourceMMEUES1APID)
		},
		Encode: func(m *PathSwitchRequest) (per.Marshaler, bool) { return &m.SourceMMEUES1APID, true },
	},
	{
		ID: IDEUTRANCGI, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequest, r *per.Reader, enc per.Encoding) error {
			var v EUTRANCGI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.EUTRANCGI = &v

			return nil
		},
		Encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.EUTRANCGI == nil {
				return nil, false
			}

			return m.EUTRANCGI, true
		},
	},
	{
		ID: IDTAI, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequest, r *per.Reader, enc per.Encoding) error {
			var v TAI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.TAI = &v

			return nil
		},
		Encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.TAI == nil {
				return nil, false
			}

			return m.TAI, true
		},
	},
	{
		ID: IDUESecurityCapabilities, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequest, r *per.Reader, enc per.Encoding) error {
			var v UESecurityCapabilities

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UESecurityCapabilities = &v

			return nil
		},
		Encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.UESecurityCapabilities == nil {
				return nil, false
			}

			return m.UESecurityCapabilities, true
		},
	},
	{
		ID: IDSourceMMEGUMMEI, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequest, r *per.Reader, enc per.Encoding) error {
			var v GUMMEI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.SourceMMEGUMMEI = &v

			return nil
		},
		Encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.SourceMMEGUMMEI == nil {
				return nil, false
			}

			return m.SourceMMEGUMMEI, true
		},
	},
	{
		ID: IDTunnelInformationForBBF, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.TunnelInformationForBBF)
		},
		Encode: func(m *PathSwitchRequest) (per.Marshaler, bool) { return optionalIE(m.TunnelInformationForBBF) },
	},
}

func (m *PathSwitchRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPathSwitchRequest, pathSwitchRequestIEs, m)
}

func (m *PathSwitchRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPathSwitchRequest,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePathSwitchRequest(value []byte) (*PathSwitchRequest, error) {
	return parseMessageBody[PathSwitchRequest](ProcPathSwitchRequest, TriggeringInitiatingMessage, pathSwitchRequestIEs, value)
}

func DecodePathSwitchRequest(value []byte) (*PathSwitchRequest, error) {
	return decodeMessageBody[PathSwitchRequest](ProcPathSwitchRequest, pathSwitchRequestIEs, value)
}

// TS 36.413 §9.1.5.9.
type PathSwitchRequestAcknowledge struct {
	MMEUES1APID               *MMEUES1APID
	ENBUES1APID               *ENBUES1APID
	UEAggregateMaximumBitRate *UEAggregateMaximumBitRate
	ERABToBeSwitchedUL        ERABToBeSwitchedULList
	SecurityContext           SecurityContext
	MMEUES1APID2              *MMEUES1APID
	UESecurityCapabilities    *UESecurityCapabilities
	// E-RABs the MME failed to switch the UP path for, whose data radio bearers
	// the eNB releases (§8.4.4.2).
	ERABToBeReleased    []ERABItem
	TimeRefDistribution *TimeRefDistribution

	messageMeta
}

var pathSwitchRequestAcknowledgeIEs = []ieSpec[PathSwitchRequestAcknowledge]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v ENBUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		ID: IDUEAggregateMaximumBitrate, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
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
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
	{
		ID: IDERABToBeSwitchedULList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ERABToBeSwitchedUL)
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if len(m.ERABToBeSwitchedUL) == 0 {
				return nil, false
			}

			return m.ERABToBeSwitchedUL, true
		},
	},
	{
		ID: IDERABToBeReleasedList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABToBeReleased, err = decodeItemList[ERABItem](r, enc, maxnoofERABs, IDERABItem, CriticalityIgnore)

			return err
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if len(m.ERABToBeReleased) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABItem, CriticalityIgnore, m.ERABToBeReleased), true
		},
	},
	{
		ID: IDSecurityContext, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SecurityContext)
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) { return &m.SecurityContext, true },
	},
	{
		ID: IDMMEUES1APID2, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID2 = &v

			return nil
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.MMEUES1APID2 == nil {
				return nil, false
			}

			return m.MMEUES1APID2, true
		},
	},
	{
		ID: IDUESecurityCapabilities, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var (
				err  error
				caps UESecurityCapabilities
			)

			err = decodeIE(r, &caps)
			if err != nil {
				return err
			}

			m.UESecurityCapabilities = &caps

			return nil
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.UESecurityCapabilities == nil {
				return nil, false
			}

			return m.UESecurityCapabilities, true
		},
	},
	{
		ID: IDTimeRefDistribution, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.TimeRefDistribution)
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) { return optionalIE(m.TimeRefDistribution) },
	},
}

func (m *PathSwitchRequestAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPathSwitchRequest, pathSwitchRequestAcknowledgeIEs, m)
}

func (m *PathSwitchRequestAcknowledge) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcPathSwitchRequest,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePathSwitchRequestAcknowledge(value []byte) (*PathSwitchRequestAcknowledge, error) {
	return parseMessageBody[PathSwitchRequestAcknowledge](ProcPathSwitchRequest, TriggeringSuccessfulOutcome, pathSwitchRequestAcknowledgeIEs, value)
}

func DecodePathSwitchRequestAcknowledge(value []byte) (*PathSwitchRequestAcknowledge, error) {
	return decodeMessageBody[PathSwitchRequestAcknowledge](ProcPathSwitchRequest, pathSwitchRequestAcknowledgeIEs, value)
}

// TS 36.413 §9.1.5.10.
type PathSwitchRequestFailure struct {
	MMEUES1APID            *MMEUES1APID
	ENBUES1APID            *ENBUES1APID
	Cause                  *Cause
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var pathSwitchRequestFailureIEs = []ieSpec[PathSwitchRequestFailure]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestFailure, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		Encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestFailure, r *per.Reader, enc per.Encoding) error {
			var v ENBUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		Encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestFailure, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestFailure, r *per.Reader, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		Encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *PathSwitchRequestFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPathSwitchRequest, pathSwitchRequestFailureIEs, m)
}

func (m *PathSwitchRequestFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcPathSwitchRequest,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePathSwitchRequestFailure(value []byte) (*PathSwitchRequestFailure, error) {
	return parseMessageBody[PathSwitchRequestFailure](ProcPathSwitchRequest, TriggeringUnsuccessfulOutcome, pathSwitchRequestFailureIEs, value)
}

func DecodePathSwitchRequestFailure(value []byte) (*PathSwitchRequestFailure, error) {
	return decodeMessageBody[PathSwitchRequestFailure](ProcPathSwitchRequest, pathSwitchRequestFailureIEs, value)
}
