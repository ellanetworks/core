// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// ERABToBeSwitchedDLItem ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// gTP-TEID, iE-Extensions OPTIONAL } (extensible). For one E-RAB it names the
// target eNB's S1-U downlink endpoint the GTP tunnel is switched to.
type ERABToBeSwitchedDLItem struct {
	_                     [0]struct{} `per:"extseq"`
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	_                     ieExtensions `per:",skip"`
}

// SecurityContext ::= SEQUENCE { nextHopChainingCount INTEGER (0..7),
// nextHopParameter SecurityKey, iE-Extensions OPTIONAL } (extensible). It carries
// the {NCC, NH} the target eNB uses to derive the next KeNB (TS 33.401).
type SecurityContext struct {
	_                    [0]struct{} `per:"extseq"`
	NextHopChainingCount uint8       `per:",range:0..7"`
	NextHopParameter     SecurityKey
	_                    ieExtensions `per:",skip"`
}

// PathSwitchRequest is the PATH SWITCH REQUEST message (TS 36.413), sent
// by the target eNB after an X2 handover to switch the downlink GTP tunnel to
// itself. SourceMMEUES1APID is the MME UE S1AP ID the source eNB held, used to
// find the UE context.
type PathSwitchRequest struct {
	ENBUES1APID            ENBUES1APID
	ERABToBeSwitchedDL     []ERABToBeSwitchedDLItem
	SourceMMEUES1APID      MMEUES1APID
	EUTRANCGI              EUTRANCGI
	TAI                    TAI
	UESecurityCapabilities UESecurityCapabilities

	unmodeledIEs
}

// pathSwitchRequestIEs is the PathSwitchRequest IE table (TS 36.413).
var pathSwitchRequestIEs = []ieSpec[PathSwitchRequest]{
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idERABToBeSwitchedDLList, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABToBeSwitchedDL, err = decodeItemList[ERABToBeSwitchedDLItem](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABToBeSwitchedDLItem, CriticalityReject, m.ERABToBeSwitchedDL)
			}), true
		},
	},
	{
		id: idSourceMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SourceMMEUES1APID)
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) { return &m.SourceMMEUES1APID, true },
	},
	{
		id: idEUTRANCGI, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.EUTRANCGI)
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) { return &m.EUTRANCGI, true },
	},
	{
		id: idTAI, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TAI)
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) { return &m.TAI, true },
	},
	{
		id: idUESecurityCapabilities, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UESecurityCapabilities)
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) { return &m.UESecurityCapabilities, true },
	},
}

func (m *PathSwitchRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, pathSwitchRequestIEs, m)
}

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParsePathSwitchRequest decodes the message from an initiatingMessage open-type
// payload.
func ParsePathSwitchRequest(value []byte) (*PathSwitchRequest, error) {
	return parseMessageBody[PathSwitchRequest](ProcPathSwitchRequest, pathSwitchRequestIEs, value)
}

// PathSwitchRequestAcknowledge is the PATH SWITCH REQUEST ACKNOWLEDGE message
// (TS 36.413), sent by the MME once the downlink path has been switched.
// SecurityContext carries the {NCC, NH}; UESecurityCapabilities is included only
// when the MME's stored capabilities differ from those the eNB reported.
type PathSwitchRequestAcknowledge struct {
	MMEUES1APID               MMEUES1APID
	ENBUES1APID               ENBUES1APID
	UEAggregateMaximumBitRate *UEAggregateMaximumBitRate
	SecurityContext           SecurityContext
	UESecurityCapabilities    *UESecurityCapabilities
	// ERABToBeReleased lists the E-RABs the MME failed to switch the UP path for, so
	// the eNB releases their data radio bearers (TS 36.413 §8.4.4.2). Empty on a full
	// switch.
	ERABToBeReleased []ERABItem

	unmodeledIEs
}

// pathSwitchRequestAcknowledgeIEs is the PathSwitchRequestAcknowledge IE table (TS 36.413).
var pathSwitchRequestAcknowledgeIEs = []ieSpec[PathSwitchRequestAcknowledge]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idUEAggregateMaximumBitrate, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var (
				err  error
				ambr UEAggregateMaximumBitRate
			)

			err = perIEDecode(raw, &ambr)
			m.UEAggregateMaximumBitRate = &ambr

			return err
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
	{
		id: idSecurityContext, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SecurityContext)
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) { return &m.SecurityContext, true },
	},
	{
		id: idUESecurityCapabilities, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var (
				err  error
				caps UESecurityCapabilities
			)

			err = perIEDecode(raw, &caps)
			m.UESecurityCapabilities = &caps

			return err
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.UESecurityCapabilities == nil {
				return nil, false
			}

			return m.UESecurityCapabilities, true
		},
	},
	{
		id: idERABToBeReleasedList, presence: PresenceOptional, crit: CriticalityIgnore,
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
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABToBeReleased)
			}), true
		},
	},
}

func (m *PathSwitchRequestAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, pathSwitchRequestAcknowledgeIEs, m)
}

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParsePathSwitchRequestAcknowledge decodes the message from a successfulOutcome
// open-type payload.
func ParsePathSwitchRequestAcknowledge(value []byte) (*PathSwitchRequestAcknowledge, error) {
	return parseMessageBody[PathSwitchRequestAcknowledge](ProcPathSwitchRequest, pathSwitchRequestAcknowledgeIEs, value)
}

// PathSwitchRequestFailure is the PATH SWITCH REQUEST FAILURE message (TS 36.413),
// sent by the MME when the downlink path could not be switched for
// any E-RAB.
type PathSwitchRequestFailure struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Cause       Cause

	unmodeledIEs
}

// pathSwitchRequestFailureIEs is the PathSwitchRequestFailure IE table (TS 36.413).
var pathSwitchRequestFailureIEs = []ieSpec[PathSwitchRequestFailure]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idCause, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.Cause)
		},
		encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) { return &m.Cause, true },
	},
}

func (m *PathSwitchRequestFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, pathSwitchRequestFailureIEs, m)
}

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParsePathSwitchRequestFailure decodes the message from an unsuccessfulOutcome
// open-type payload.
func ParsePathSwitchRequestFailure(value []byte) (*PathSwitchRequestFailure, error) {
	return parseMessageBody[PathSwitchRequestFailure](ProcPathSwitchRequest, pathSwitchRequestFailureIEs, value)
}
