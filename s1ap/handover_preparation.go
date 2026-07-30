// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
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

func (m *HandoverRequired) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		{id: idHandoverType, crit: CriticalityReject, val: &m.HandoverType},
		{id: idCause, crit: CriticalityIgnore, val: &m.Cause},
		{id: idTargetID, crit: CriticalityReject, val: &m.TargetID},
		{id: idSourceToTargetTransparentContainer, crit: CriticalityReject, val: &m.SourceToTarget},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *HandoverRequired) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcHandoverPreparation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseHandoverRequired decodes the message from an initiatingMessage open-type
// payload.
func ParseHandoverRequired(value []byte) (*HandoverRequired, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverRequired preamble: %w", err)
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

	m := &HandoverRequired{}

	var seenMME, seenENB, seenType, seenCause, seenTarget, seenContainer bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idHandoverType:
			err = perIEDecode(f.value, &m.HandoverType)
			seenType = true
		case idCause:
			err = perIEDecode(f.value, &m.Cause)
			seenCause = true
		case idTargetID:
			err = perIEDecode(f.value, &m.TargetID)
			seenTarget = true
		case idSourceToTargetTransparentContainer:
			err = perIEDecode(f.value, &m.SourceToTarget)
			seenContainer = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: HandoverRequired IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcHandoverPreparation,
		ieCheck{idMMEUES1APID, CriticalityReject, seenMME},
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idHandoverType, CriticalityReject, seenType},
		ieCheck{idTargetID, CriticalityReject, seenTarget},
		ieCheck{idSourceToTargetTransparentContainer, CriticalityReject, seenContainer},
		ieCheck{idCause, CriticalityIgnore, seenCause},
	); err != nil {
		return nil, err
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

func (m *HandoverCommand) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		{id: idHandoverType, crit: CriticalityReject, val: &m.HandoverType},
	}

	if len(m.ERABToRelease) > 0 {
		fields = append(fields, ieField{id: idERABtoReleaseListHOCmd, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABToRelease)
		})})
	}

	fields = append(fields, ieField{id: idTargetToSourceTransparentContainer, crit: CriticalityReject, val: &m.TargetToSource})

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *HandoverCommand) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcHandoverPreparation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseHandoverCommand decodes the message from a successfulOutcome open-type
// payload.
func ParseHandoverCommand(value []byte) (*HandoverCommand, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverCommand preamble: %w", err)
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

	m := &HandoverCommand{}

	var seenMME, seenENB, seenType, seenContainer bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idHandoverType:
			err = perIEDecode(f.value, &m.HandoverType)
			seenType = true
		case idERABtoReleaseListHOCmd:
			m.ERABToRelease, err = decodeItemList[ERABItem](per.NewReader(f.value), enc, maxnoofERABs)
		case idTargetToSourceTransparentContainer:
			err = perIEDecode(f.value, &m.TargetToSource)
			seenContainer = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: HandoverCommand IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcHandoverPreparation,
		ieCheck{idMMEUES1APID, CriticalityReject, seenMME},
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idHandoverType, CriticalityReject, seenType},
		ieCheck{idTargetToSourceTransparentContainer, CriticalityReject, seenContainer},
	); err != nil {
		return nil, err
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

func (m *HandoverPreparationFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
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
func (m *HandoverPreparationFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcHandoverPreparation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseHandoverPreparationFailure decodes the message from an unsuccessfulOutcome
// open-type payload.
func ParseHandoverPreparationFailure(value []byte) (*HandoverPreparationFailure, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverPreparationFailure preamble: %w", err)
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

	m := &HandoverPreparationFailure{}

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
			return nil, fmt.Errorf("s1ap: HandoverPreparationFailure IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcHandoverPreparation,
		ieCheck{idMMEUES1APID, CriticalityIgnore, seenMME},
		ieCheck{idENBUES1APID, CriticalityIgnore, seenENB},
		ieCheck{idCause, CriticalityIgnore, seenCause},
	); err != nil {
		return nil, err
	}

	return m, nil
}
