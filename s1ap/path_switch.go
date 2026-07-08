// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// ERABToBeSwitchedDLItem ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// gTP-TEID, iE-Extensions OPTIONAL } (extensible). For one E-RAB it names the
// target eNB's S1-U downlink endpoint the GTP tunnel is switched to.
type ERABToBeSwitchedDLItem struct {
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
}

func (it ERABToBeSwitchedDLItem) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := it.ERABID.encode(w); err != nil {
		return err
	}

	if err := it.TransportLayerAddress.encode(w); err != nil {
		return err
	}

	return it.GTPTEID.encode(w)
}

func decodeERABToBeSwitchedDLItem(r *aper.Reader) (ERABToBeSwitchedDLItem, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return ERABToBeSwitchedDLItem{}, err
	}

	var it ERABToBeSwitchedDLItem

	if it.ERABID, err = decodeERABID(r); err != nil {
		return it, err
	}

	if it.TransportLayerAddress, err = decodeTransportLayerAddress(r); err != nil {
		return it, err
	}

	if it.GTPTEID, err = decodeGTPTEID(r); err != nil {
		return it, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return it, err
	}

	return it, nil
}

func decodeERABToBeSwitchedDLList(r *aper.Reader) ([]ERABToBeSwitchedDLItem, error) {
	return decodeItemList(r, maxnoofERABs, decodeERABToBeSwitchedDLItem)
}

// SecurityContext ::= SEQUENCE { nextHopChainingCount INTEGER (0..7),
// nextHopParameter SecurityKey, iE-Extensions OPTIONAL } (extensible). It carries
// the {NCC, NH} the target eNB uses to derive the next KeNB (TS 33.401).
type SecurityContext struct {
	NextHopChainingCount uint8
	NextHopParameter     SecurityKey
}

func (s SecurityContext) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := w.WriteConstrainedInt(int64(s.NextHopChainingCount), 0, 7); err != nil {
		return err
	}

	return s.NextHopParameter.encode(w)
}

func decodeSecurityContext(r *aper.Reader) (SecurityContext, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return SecurityContext{}, err
	}

	ncc, err := r.ReadConstrainedInt(0, 7)
	if err != nil {
		return SecurityContext{}, err
	}

	nh, err := decodeSecurityKey(r)
	if err != nil {
		return SecurityContext{}, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return SecurityContext{}, err
	}

	return SecurityContext{NextHopChainingCount: uint8(ncc), NextHopParameter: nh}, nil
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

func (m *PathSwitchRequest) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		{id: idERABToBeSwitchedDLList, crit: CriticalityReject, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABToBeSwitchedDLItem, CriticalityReject, encoderList(m.ERABToBeSwitchedDL))
		}},
		{id: idSourceMMEUES1APID, crit: CriticalityReject, enc: m.SourceMMEUES1APID.encode},
		{id: idEUTRANCGI, crit: CriticalityIgnore, enc: m.EUTRANCGI.encode},
		{id: idTAI, crit: CriticalityIgnore, enc: m.TAI.encode},
		{id: idUESecurityCapabilities, crit: CriticalityIgnore, enc: m.UESecurityCapabilities.encode},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *PathSwitchRequest) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPathSwitchRequest,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParsePathSwitchRequest decodes the message from an initiatingMessage open-type
// payload.
func ParsePathSwitchRequest(value []byte) (*PathSwitchRequest, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: PathSwitchRequest preamble: %w", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, err
		}
	}

	m := &PathSwitchRequest{}

	var seenENB, seenERAB, seenSrcMME, seenCGI, seenTAI, seenSec bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idERABToBeSwitchedDLList:
			m.ERABToBeSwitchedDL, err = decodeERABToBeSwitchedDLList(sub)
			seenERAB = true
		case idSourceMMEUES1APID:
			m.SourceMMEUES1APID, err = decodeMMEUES1APID(sub)
			seenSrcMME = true
		case idEUTRANCGI:
			m.EUTRANCGI, err = decodeEUTRANCGI(sub)
			seenCGI = true
		case idTAI:
			m.TAI, err = decodeTAI(sub)
			seenTAI = true
		case idUESecurityCapabilities:
			m.UESecurityCapabilities, err = decodeUESecurityCapabilities(sub)
			seenSec = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: PathSwitchRequest IE %d: %w", f.id, err)
		}
	}

	if !seenENB || !seenERAB || !seenSrcMME || !seenCGI || !seenTAI || !seenSec {
		return nil, fmt.Errorf("s1ap: PathSwitchRequest missing mandatory IE")
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

func (m *PathSwitchRequestAcknowledge) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
	}

	if m.UEAggregateMaximumBitRate != nil {
		ambr := *m.UEAggregateMaximumBitRate
		fields = append(fields, ieField{id: idUEAggregateMaximumBitrate, crit: CriticalityIgnore, enc: ambr.encode})
	}

	fields = append(fields, ieField{id: idSecurityContext, crit: CriticalityReject, enc: m.SecurityContext.encode})

	if m.UESecurityCapabilities != nil {
		caps := *m.UESecurityCapabilities
		fields = append(fields, ieField{id: idUESecurityCapabilities, crit: CriticalityIgnore, enc: caps.encode})
	}

	if len(m.ERABToBeReleased) > 0 {
		fields = append(fields, ieField{id: idERABToBeReleasedList, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABItem, CriticalityIgnore, encoderList(m.ERABToBeReleased))
		}})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *PathSwitchRequestAcknowledge) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcPathSwitchRequest,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParsePathSwitchRequestAcknowledge decodes the message from a successfulOutcome
// open-type payload.
func ParsePathSwitchRequestAcknowledge(value []byte) (*PathSwitchRequestAcknowledge, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: PathSwitchRequestAcknowledge preamble: %w", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, err
		}
	}

	m := &PathSwitchRequestAcknowledge{}

	var seenMME, seenENB, seenSec bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idUEAggregateMaximumBitrate:
			var ambr UEAggregateMaximumBitRate

			ambr, err = decodeUEAggregateMaximumBitRate(sub)
			m.UEAggregateMaximumBitRate = &ambr
		case idSecurityContext:
			m.SecurityContext, err = decodeSecurityContext(sub)
			seenSec = true
		case idUESecurityCapabilities:
			var caps UESecurityCapabilities

			caps, err = decodeUESecurityCapabilities(sub)
			m.UESecurityCapabilities = &caps
		case idERABToBeReleasedList:
			m.ERABToBeReleased, err = decodeERABItemList(sub)
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: PathSwitchRequestAcknowledge IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenSec {
		return nil, fmt.Errorf("s1ap: PathSwitchRequestAcknowledge missing mandatory IE")
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

func (m *PathSwitchRequestFailure) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		{id: idCause, crit: CriticalityIgnore, enc: m.Cause.encode},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *PathSwitchRequestFailure) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcPathSwitchRequest,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParsePathSwitchRequestFailure decodes the message from an unsuccessfulOutcome
// open-type payload.
func ParsePathSwitchRequestFailure(value []byte) (*PathSwitchRequestFailure, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: PathSwitchRequestFailure preamble: %w", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, err
		}
	}

	m := &PathSwitchRequestFailure{}

	var seenMME, seenENB, seenCause bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idCause:
			m.Cause, err = decodeCause(sub)
			seenCause = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: PathSwitchRequestFailure IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenCause {
		return nil, fmt.Errorf("s1ap: PathSwitchRequestFailure missing mandatory IE")
	}

	return m, nil
}
