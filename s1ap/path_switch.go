// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

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

func (m *PathSwitchRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		{id: idERABToBeSwitchedDLList, crit: CriticalityReject, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABToBeSwitchedDLItem, CriticalityReject, m.ERABToBeSwitchedDL)
		})},
		{id: idSourceMMEUES1APID, crit: CriticalityReject, val: &m.SourceMMEUES1APID},
		{id: idEUTRANCGI, crit: CriticalityIgnore, val: &m.EUTRANCGI},
		{id: idTAI, crit: CriticalityIgnore, val: &m.TAI},
		{id: idUESecurityCapabilities, crit: CriticalityIgnore, val: &m.UESecurityCapabilities},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
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
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: PathSwitchRequest preamble: %w", err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, err
		}
	}

	m := &PathSwitchRequest{}

	var seenENB, seenERAB, seenSrcMME, seenCGI, seenTAI, seenSec bool

	for _, f := range fields {
		switch f.id {
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idERABToBeSwitchedDLList:
			m.ERABToBeSwitchedDL, err = decodeItemList[ERABToBeSwitchedDLItem](per.NewReader(f.value), enc, maxnoofERABs)
			seenERAB = true
		case idSourceMMEUES1APID:
			err = perIEDecode(f.value, &m.SourceMMEUES1APID)
			seenSrcMME = true
		case idEUTRANCGI:
			err = perIEDecode(f.value, &m.EUTRANCGI)
			seenCGI = true
		case idTAI:
			err = perIEDecode(f.value, &m.TAI)
			seenTAI = true
		case idUESecurityCapabilities:
			err = perIEDecode(f.value, &m.UESecurityCapabilities)
			seenSec = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: PathSwitchRequest IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcPathSwitchRequest,
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idERABToBeSwitchedDLList, CriticalityReject, seenERAB},
		ieCheck{idSourceMMEUES1APID, CriticalityReject, seenSrcMME},
		ieCheck{idEUTRANCGI, CriticalityIgnore, seenCGI},
		ieCheck{idTAI, CriticalityIgnore, seenTAI},
		ieCheck{idUESecurityCapabilities, CriticalityIgnore, seenSec},
	); err != nil {
		return nil, err
	}

	return m, nil
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

func (m *PathSwitchRequestAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
	}

	if m.UEAggregateMaximumBitRate != nil {
		ambr := *m.UEAggregateMaximumBitRate
		fields = append(fields, ieField{id: idUEAggregateMaximumBitrate, crit: CriticalityIgnore, val: &ambr})
	}

	fields = append(fields, ieField{id: idSecurityContext, crit: CriticalityReject, val: &m.SecurityContext})

	if m.UESecurityCapabilities != nil {
		caps := *m.UESecurityCapabilities
		fields = append(fields, ieField{id: idUESecurityCapabilities, crit: CriticalityIgnore, val: &caps})
	}

	if len(m.ERABToBeReleased) > 0 {
		fields = append(fields, ieField{id: idERABToBeReleasedList, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABToBeReleased)
		})})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
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
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: PathSwitchRequestAcknowledge preamble: %w", err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, err
		}
	}

	m := &PathSwitchRequestAcknowledge{}

	var seenMME, seenENB, seenSec bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idUEAggregateMaximumBitrate:
			var ambr UEAggregateMaximumBitRate

			err = perIEDecode(f.value, &ambr)
			m.UEAggregateMaximumBitRate = &ambr
		case idSecurityContext:
			err = perIEDecode(f.value, &m.SecurityContext)
			seenSec = true
		case idUESecurityCapabilities:
			var caps UESecurityCapabilities

			err = perIEDecode(f.value, &caps)
			m.UESecurityCapabilities = &caps
		case idERABToBeReleasedList:
			m.ERABToBeReleased, err = decodeItemList[ERABItem](per.NewReader(f.value), enc, maxnoofERABs)
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: PathSwitchRequestAcknowledge IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcPathSwitchRequest,
		ieCheck{idSecurityContext, CriticalityReject, seenSec},
		ieCheck{idMMEUES1APID, CriticalityIgnore, seenMME},
		ieCheck{idENBUES1APID, CriticalityIgnore, seenENB},
	); err != nil {
		return nil, err
	}

	return m, nil
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

func (m *PathSwitchRequestFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		{id: idCause, crit: CriticalityIgnore, val: &m.Cause},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
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
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: PathSwitchRequestFailure preamble: %w", err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, err
		}
	}

	m := &PathSwitchRequestFailure{}

	var seenMME, seenENB, seenCause bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idCause:
			err = perIEDecode(f.value, &m.Cause)
			seenCause = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: PathSwitchRequestFailure IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcPathSwitchRequest,
		ieCheck{idMMEUES1APID, CriticalityIgnore, seenMME},
		ieCheck{idENBUES1APID, CriticalityIgnore, seenENB},
		ieCheck{idCause, CriticalityIgnore, seenCause},
	); err != nil {
		return nil, err
	}

	return m, nil
}
