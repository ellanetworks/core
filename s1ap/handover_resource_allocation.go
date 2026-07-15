// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/aper"
)

// HandoverRequest is the HANDOVER REQUEST message (TS 36.413), sent by
// the MME to the target eNB to reserve resources for an incoming handover. It
// carries no eNB UE S1AP ID: the target eNB allocates its own and returns it in
// the HANDOVER REQUEST ACKNOWLEDGE. SecurityContext carries the {NCC, NH} the
// target uses to derive KeNB (TS 33.401); SourceToTarget is the opaque
// source-to-target transparent container.
type HandoverRequest struct {
	MMEUES1APID            MMEUES1APID
	HandoverType           HandoverType
	Cause                  Cause
	UEAMBR                 UEAggregateMaximumBitRate
	ERABToBeSetup          []ERABToBeSetupItemHOReq
	SourceToTarget         TransparentContainer
	UESecurityCapabilities UESecurityCapabilities
	SecurityContext        SecurityContext

	unmodeledIEs
}

func (m *HandoverRequest) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idHandoverType, crit: CriticalityReject, enc: m.HandoverType.encode},
		{id: idCause, crit: CriticalityIgnore, enc: m.Cause.encode},
		{id: idUEAggregateMaximumBitrate, crit: CriticalityReject, enc: m.UEAMBR.encode},
		{id: idERABToBeSetupListHOReq, crit: CriticalityReject, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABToBeSetupItemHOReq, CriticalityReject, encoderList(m.ERABToBeSetup))
		}},
		{id: idSourceToTargetTransparentContainer, crit: CriticalityReject, enc: m.SourceToTarget.encode},
		{id: idUESecurityCapabilities, crit: CriticalityReject, enc: m.UESecurityCapabilities.encode},
		{id: idSecurityContext, crit: CriticalityReject, enc: m.SecurityContext.encode},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *HandoverRequest) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcHandoverResourceAllocation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseHandoverRequest decodes the message from an initiatingMessage open-type
// payload.
func ParseHandoverRequest(value []byte) (*HandoverRequest, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverRequest preamble: %w", err)
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

	m := &HandoverRequest{}

	var seenMME, seenType, seenCause, seenAMBR, seenERAB, seenContainer, seenSec, seenCtx bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idHandoverType:
			m.HandoverType, err = decodeHandoverType(sub)
			seenType = true
		case idCause:
			m.Cause, err = decodeCause(sub)
			seenCause = true
		case idUEAggregateMaximumBitrate:
			m.UEAMBR, err = decodeUEAggregateMaximumBitRate(sub)
			seenAMBR = true
		case idERABToBeSetupListHOReq:
			m.ERABToBeSetup, err = decodeERABToBeSetupListHOReq(sub)
			seenERAB = true
		case idSourceToTargetTransparentContainer:
			m.SourceToTarget, err = decodeTransparentContainer(sub)
			seenContainer = true
		case idUESecurityCapabilities:
			m.UESecurityCapabilities, err = decodeUESecurityCapabilities(sub)
			seenSec = true
		case idSecurityContext:
			m.SecurityContext, err = decodeSecurityContext(sub)
			seenCtx = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: HandoverRequest IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenType || !seenCause || !seenAMBR || !seenERAB || !seenContainer || !seenSec || !seenCtx {
		return nil, fmt.Errorf("s1ap: HandoverRequest missing mandatory IE")
	}

	return m, nil
}

// HandoverRequestAcknowledge is the HANDOVER REQUEST ACKNOWLEDGE message
// (TS 36.413), the successful outcome the target eNB returns. ERABAdmitted
// carries the target eNB's S1-U downlink endpoint per E-RAB; ERABFailedToSetup
// lists the bearers the target rejected; TargetToSource is the opaque target-to-
// source transparent container.
type HandoverRequestAcknowledge struct {
	MMEUES1APID       MMEUES1APID
	ENBUES1APID       ENBUES1APID
	ERABAdmitted      []ERABAdmittedItem
	ERABFailedToSetup []ERABItem
	TargetToSource    TransparentContainer

	unmodeledIEs
}

func (m *HandoverRequestAcknowledge) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityIgnore, enc: m.ENBUES1APID.encode},
		{id: idERABAdmittedList, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABAdmittedItem, CriticalityIgnore, encoderList(m.ERABAdmitted))
		}},
	}

	if len(m.ERABFailedToSetup) > 0 {
		fields = append(fields, ieField{id: idERABFailedToSetupListHOReqAck, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABItem, CriticalityIgnore, encoderList(m.ERABFailedToSetup))
		}})
	}

	fields = append(fields, ieField{id: idTargetToSourceTransparentContainer, crit: CriticalityReject, enc: m.TargetToSource.encode})

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *HandoverRequestAcknowledge) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcHandoverResourceAllocation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseHandoverRequestAcknowledge decodes the message from a successfulOutcome
// open-type payload.
func ParseHandoverRequestAcknowledge(value []byte) (*HandoverRequestAcknowledge, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverRequestAcknowledge preamble: %w", err)
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

	m := &HandoverRequestAcknowledge{}

	var seenMME, seenENB, seenAdmitted, seenContainer bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idERABAdmittedList:
			m.ERABAdmitted, err = decodeERABAdmittedList(sub)
			seenAdmitted = true
		case idERABFailedToSetupListHOReqAck:
			m.ERABFailedToSetup, err = decodeERABItemList(sub)
		case idTargetToSourceTransparentContainer:
			m.TargetToSource, err = decodeTransparentContainer(sub)
			seenContainer = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: HandoverRequestAcknowledge IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenAdmitted || !seenContainer {
		return nil, fmt.Errorf("s1ap: HandoverRequestAcknowledge missing mandatory IE")
	}

	return m, nil
}

// HandoverFailure is the HANDOVER FAILURE message (TS 36.413 in the
// Handover Resource Allocation procedure), the unsuccessful outcome the target eNB
// returns when it cannot admit the handover. It carries no eNB UE S1AP ID: the
// target allocated no UE context.
type HandoverFailure struct {
	MMEUES1APID MMEUES1APID
	Cause       Cause

	unmodeledIEs
}

func (m *HandoverFailure) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, enc: m.MMEUES1APID.encode},
		{id: idCause, crit: CriticalityIgnore, enc: m.Cause.encode},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *HandoverFailure) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcHandoverResourceAllocation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseHandoverFailure decodes the message from an unsuccessfulOutcome open-type
// payload.
func ParseHandoverFailure(value []byte) (*HandoverFailure, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverFailure preamble: %w", err)
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

	m := &HandoverFailure{}

	var seenMME, seenCause bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idCause:
			m.Cause, err = decodeCause(sub)
			seenCause = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: HandoverFailure IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenCause {
		return nil, fmt.Errorf("s1ap: HandoverFailure missing mandatory IE")
	}

	return m, nil
}
