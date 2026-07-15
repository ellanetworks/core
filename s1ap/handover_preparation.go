// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/aper"
)

// HandoverRequired is the HANDOVER REQUIRED message (TS 36.413), sent by
// the source eNB to start the Handover Preparation procedure. SourceToTarget is
// the opaque source-eNB-to-target-eNB transparent container relayed to the target.
type HandoverRequired struct {
	MMEUES1APID    MMEUES1APID
	ENBUES1APID    ENBUES1APID
	HandoverType   HandoverType
	Cause          Cause
	TargetID       TargetID
	SourceToTarget TransparentContainer

	unmodeledIEs
}

func (m *HandoverRequired) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		{id: idHandoverType, crit: CriticalityReject, enc: m.HandoverType.encode},
		{id: idCause, crit: CriticalityIgnore, enc: m.Cause.encode},
		{id: idTargetID, crit: CriticalityReject, enc: m.TargetID.encode},
		{id: idSourceToTargetTransparentContainer, crit: CriticalityReject, enc: m.SourceToTarget.encode},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *HandoverRequired) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcHandoverPreparation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseHandoverRequired decodes the message from an initiatingMessage open-type
// payload.
func ParseHandoverRequired(value []byte) (*HandoverRequired, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverRequired preamble: %w", err)
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

	m := &HandoverRequired{}

	var seenMME, seenENB, seenType, seenCause, seenTarget, seenContainer bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idHandoverType:
			m.HandoverType, err = decodeHandoverType(sub)
			seenType = true
		case idCause:
			m.Cause, err = decodeCause(sub)
			seenCause = true
		case idTargetID:
			m.TargetID, err = decodeTargetID(sub)
			seenTarget = true
		case idSourceToTargetTransparentContainer:
			m.SourceToTarget, err = decodeTransparentContainer(sub)
			seenContainer = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: HandoverRequired IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenType || !seenCause || !seenTarget || !seenContainer {
		return nil, fmt.Errorf("s1ap: HandoverRequired missing mandatory IE")
	}

	return m, nil
}

// HandoverCommand is the HANDOVER COMMAND message (TS 36.413), the
// successful outcome the MME returns to the source eNB. ERABToRelease lists the
// bearers the target did not admit (TS 23.401); TargetToSource
// is the opaque target-to-source transparent container.
type HandoverCommand struct {
	MMEUES1APID    MMEUES1APID
	ENBUES1APID    ENBUES1APID
	HandoverType   HandoverType
	ERABToRelease  []ERABItem
	TargetToSource TransparentContainer

	unmodeledIEs
}

func (m *HandoverCommand) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		{id: idHandoverType, crit: CriticalityReject, enc: m.HandoverType.encode},
	}

	if len(m.ERABToRelease) > 0 {
		fields = append(fields, ieField{id: idERABtoReleaseListHOCmd, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABItem, CriticalityIgnore, encoderList(m.ERABToRelease))
		}})
	}

	fields = append(fields, ieField{id: idTargetToSourceTransparentContainer, crit: CriticalityReject, enc: m.TargetToSource.encode})

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *HandoverCommand) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcHandoverPreparation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseHandoverCommand decodes the message from a successfulOutcome open-type
// payload.
func ParseHandoverCommand(value []byte) (*HandoverCommand, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverCommand preamble: %w", err)
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

	m := &HandoverCommand{}

	var seenMME, seenENB, seenType, seenContainer bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idHandoverType:
			m.HandoverType, err = decodeHandoverType(sub)
			seenType = true
		case idERABtoReleaseListHOCmd:
			m.ERABToRelease, err = decodeERABItemList(sub)
		case idTargetToSourceTransparentContainer:
			m.TargetToSource, err = decodeTransparentContainer(sub)
			seenContainer = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: HandoverCommand IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenType || !seenContainer {
		return nil, fmt.Errorf("s1ap: HandoverCommand missing mandatory IE")
	}

	return m, nil
}

// HandoverPreparationFailure is the HANDOVER PREPARATION FAILURE message
// (TS 36.413), the unsuccessful outcome the MME returns to the source
// eNB when the handover cannot be prepared. The UE keeps its source-eNB context.
type HandoverPreparationFailure struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Cause       Cause

	unmodeledIEs
}

func (m *HandoverPreparationFailure) encodeBody(w *aper.Writer) error {
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
func (m *HandoverPreparationFailure) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcHandoverPreparation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseHandoverPreparationFailure decodes the message from an unsuccessfulOutcome
// open-type payload.
func ParseHandoverPreparationFailure(value []byte) (*HandoverPreparationFailure, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverPreparationFailure preamble: %w", err)
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

	m := &HandoverPreparationFailure{}

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
			return nil, fmt.Errorf("s1ap: HandoverPreparationFailure IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenCause {
		return nil, fmt.Errorf("s1ap: HandoverPreparationFailure missing mandatory IE")
	}

	return m, nil
}
