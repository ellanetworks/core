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
	_                     [0]struct{} `per:"extseq"`
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	_                     ieExtensions `per:",skip"`
}

// TS 36.413 §9.1.5.8.
type PathSwitchRequest struct {
	ENBUES1APID            ENBUES1APID
	ERABToBeSwitchedDL     []ERABToBeSwitchedDLItem
	SourceMMEUES1APID      MMEUES1APID
	EUTRANCGI              *EUTRANCGI
	TAI                    *TAI
	UESecurityCapabilities *UESecurityCapabilities

	messageMeta
}

var pathSwitchRequestIEs = []ieSpec[PathSwitchRequest]{
	{
		id: IDENBUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: IDERABToBeSwitchedDLList, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABToBeSwitchedDL, err = decodeItemList[ERABToBeSwitchedDLItem](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABToBeSwitchedDLItem, CriticalityReject, m.ERABToBeSwitchedDL)
			}), true
		},
	},
	{
		id: IDSourceMMEUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SourceMMEUES1APID)
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) { return &m.SourceMMEUES1APID, true },
	},
	{
		id: IDEUTRANCGI, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			var v EUTRANCGI

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.EUTRANCGI = &v

			return nil
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.EUTRANCGI == nil {
				return nil, false
			}

			return m.EUTRANCGI, true
		},
	},
	{
		id: IDTAI, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			var v TAI

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.TAI = &v

			return nil
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.TAI == nil {
				return nil, false
			}

			return m.TAI, true
		},
	},
	{
		id: IDUESecurityCapabilities, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			var v UESecurityCapabilities

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UESecurityCapabilities = &v

			return nil
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.UESecurityCapabilities == nil {
				return nil, false
			}

			return m.UESecurityCapabilities, true
		},
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

// TS 36.413 §9.1.5.9.
type PathSwitchRequestAcknowledge struct {
	MMEUES1APID               *MMEUES1APID
	ENBUES1APID               *ENBUES1APID
	UEAggregateMaximumBitRate *UEAggregateMaximumBitRate
	SecurityContext           SecurityContext
	UESecurityCapabilities    *UESecurityCapabilities
	// E-RABs the MME failed to switch the UP path for, whose data radio bearers
	// the eNB releases (§8.4.4.2).
	ERABToBeReleased []ERABItem

	messageMeta
}

var pathSwitchRequestAcknowledgeIEs = []ieSpec[PathSwitchRequestAcknowledge]{
	{
		id: IDMMEUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var v MMEUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		id: IDENBUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var v ENBUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		id: IDUEAggregateMaximumBitrate, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
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
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
	{
		id: IDERABToBeReleasedList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABToBeReleased, err = decodeItemList[ERABItem](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if len(m.ERABToBeReleased) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABItem, CriticalityIgnore, m.ERABToBeReleased)
			}), true
		},
	},
	{
		id: IDSecurityContext, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SecurityContext)
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) { return &m.SecurityContext, true },
	},
	{
		id: IDUESecurityCapabilities, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var (
				err  error
				caps UESecurityCapabilities
			)

			err = perIEDecode(raw, &caps)
			if err != nil {
				return err
			}

			m.UESecurityCapabilities = &caps

			return nil
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.UESecurityCapabilities == nil {
				return nil, false
			}

			return m.UESecurityCapabilities, true
		},
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
		id: IDMMEUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestFailure, raw []byte, enc per.Encoding) error {
			var v MMEUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		id: IDENBUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestFailure, raw []byte, enc per.Encoding) error {
			var v ENBUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		id: IDCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestFailure, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestFailure, raw []byte, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
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
