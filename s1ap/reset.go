// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// maxnoofIndividualS1ConnectionsToReset bounds the UE-associated logical
// S1-connection lists in Reset/Reset Acknowledge (TS 36.413 §9.3).
const maxnoofIndividualS1ConnectionsToReset = 256

// resetTypeChoiceRootCount is the number of root alternatives of the ResetType
// CHOICE: s1-Interface and partOfS1-Interface (TS 36.413 §9.2.1.3).
const resetTypeChoiceRootCount = 2

// resetAllRootCount is the number of root values of ResetAll ENUMERATED
// { reset-all, ... } (TS 36.413 §9.2.1.3).
const resetAllRootCount = 1

// UEAssociatedLogicalS1ConnectionItem identifies one UE-associated logical
// S1-connection by its MME-UE-S1AP-ID and/or eNB-UE-S1AP-ID (TS 36.413
// §9.2.3.x). Both identities are optional; an item may carry either or both.
type UEAssociatedLogicalS1ConnectionItem struct {
	MMEUES1APID *MMEUES1APID
	ENBUES1APID *ENBUES1APID
}

func (it UEAssociatedLogicalS1ConnectionItem) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{it.MMEUES1APID != nil, it.ENBUES1APID != nil, false})

	if it.MMEUES1APID != nil {
		if err := it.MMEUES1APID.encode(w); err != nil {
			return err
		}
	}

	if it.ENBUES1APID != nil {
		if err := it.ENBUES1APID.encode(w); err != nil {
			return err
		}
	}

	return nil
}

func decodeUEAssociatedLogicalS1ConnectionItem(r *aper.Reader) (UEAssociatedLogicalS1ConnectionItem, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 3)
	if err != nil {
		return UEAssociatedLogicalS1ConnectionItem{}, fmt.Errorf("s1ap: UE-associatedLogicalS1-ConnectionItem preamble: %w", err)
	}

	var it UEAssociatedLogicalS1ConnectionItem

	if opt[0] {
		v, err := decodeMMEUES1APID(r)
		if err != nil {
			return UEAssociatedLogicalS1ConnectionItem{}, err
		}

		it.MMEUES1APID = &v
	}

	if opt[1] {
		v, err := decodeENBUES1APID(r)
		if err != nil {
			return UEAssociatedLogicalS1ConnectionItem{}, err
		}

		it.ENBUES1APID = &v
	}

	if err := skipSequenceExtensions(r, opt[2], extPresent); err != nil {
		return UEAssociatedLogicalS1ConnectionItem{}, err
	}

	return it, nil
}

// ResetType is the ResetType CHOICE (TS 36.413 §9.2.1.3): All selects
// s1-Interface (ResetAll, value reset-all), reset of the whole S1 interface;
// otherwise Part selects partOfS1-Interface, the UE-associated logical
// S1-connections to reset.
type ResetType struct {
	All  bool
	Part []UEAssociatedLogicalS1ConnectionItem
}

func (t ResetType) encode(w *aper.Writer) error {
	if t.All {
		if err := w.WriteChoiceIndex(0, resetTypeChoiceRootCount, true, false); err != nil {
			return err
		}

		return w.WriteEnum(0, resetAllRootCount, true, false)
	}

	if err := w.WriteChoiceIndex(1, resetTypeChoiceRootCount, true, false); err != nil {
		return err
	}

	return encodeSingleContainerList(w, maxnoofIndividualS1ConnectionsToReset, idUEAssociatedLogicalS1ConnectionItem, CriticalityReject, encoderList(t.Part))
}

func decodeResetType(r *aper.Reader) (ResetType, error) {
	idx, isExt, err := r.ReadChoiceIndex(resetTypeChoiceRootCount, true)
	if err != nil {
		return ResetType{}, fmt.Errorf("s1ap: ResetType choice: %w", err)
	}

	if isExt {
		return ResetType{}, fmt.Errorf("s1ap: ResetType extension alternative unsupported")
	}

	switch idx {
	case 0:
		if _, _, err := r.ReadEnum(resetAllRootCount, true); err != nil {
			return ResetType{}, fmt.Errorf("s1ap: ResetAll: %w", err)
		}

		return ResetType{All: true}, nil
	case 1:
		items, err := decodeItemList(r, maxnoofIndividualS1ConnectionsToReset, decodeUEAssociatedLogicalS1ConnectionItem)
		if err != nil {
			return ResetType{}, err
		}

		return ResetType{Part: items}, nil
	default:
		return ResetType{}, fmt.Errorf("s1ap: unexpected ResetType choice index %d", idx)
	}
}

// Reset is the RESET message (TS 36.413 §9.1.2.6), sent by the eNB or MME to
// reset the whole S1 interface or a subset of its UE-associated logical
// connections.
type Reset struct {
	Cause     Cause
	ResetType ResetType

	unmodeledIEs
}

func (m *Reset) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idCause, crit: CriticalityIgnore, enc: m.Cause.encode},
		{id: idResetType, crit: CriticalityReject, enc: m.ResetType.encode},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *Reset) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcReset,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseReset decodes the message from an initiatingMessage open-type payload.
func ParseReset(value []byte) (*Reset, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: Reset preamble: %w", err)
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

	m := &Reset{}

	var seenCause, seenResetType bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idCause:
			m.Cause, err = decodeCause(sub)
			seenCause = true
		case idResetType:
			m.ResetType, err = decodeResetType(sub)
			seenResetType = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: Reset IE %d: %w", f.id, err)
		}
	}

	if !seenCause || !seenResetType {
		return nil, fmt.Errorf("s1ap: Reset missing mandatory IE")
	}

	return m, nil
}

// ResetAcknowledge is the RESET ACKNOWLEDGE message (TS 36.413 §9.1.2.7). The
// ConnectionList is present only in answer to a partOfS1-Interface reset, where
// it echoes the UE-associated logical S1-connections that were reset.
type ResetAcknowledge struct {
	ConnectionList         []UEAssociatedLogicalS1ConnectionItem
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *ResetAcknowledge) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	var fields []ieField

	if len(m.ConnectionList) > 0 {
		fields = append(fields, ieField{id: idUEAssociatedLogicalS1ConnectionListResAck, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofIndividualS1ConnectionsToReset, idUEAssociatedLogicalS1ConnectionItem, CriticalityIgnore, encoderList(m.ConnectionList))
		}})
	}

	if m.CriticalityDiagnostics != nil {
		d := *m.CriticalityDiagnostics
		fields = append(fields, ieField{id: idCriticalityDiagnostics, crit: CriticalityIgnore, enc: d.encode})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ResetAcknowledge) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcReset,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseResetAcknowledge decodes the message from a successfulOutcome open-type
// payload.
func ParseResetAcknowledge(value []byte) (*ResetAcknowledge, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: ResetAcknowledge preamble: %w", err)
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

	m := &ResetAcknowledge{}

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idUEAssociatedLogicalS1ConnectionListResAck:
			items, err := decodeItemList(sub, maxnoofIndividualS1ConnectionsToReset, decodeUEAssociatedLogicalS1ConnectionItem)
			if err != nil {
				return nil, fmt.Errorf("s1ap: ResetAcknowledge connection list: %w", err)
			}

			m.ConnectionList = append(m.ConnectionList, items...)
		case idCriticalityDiagnostics:
			cd, err := decodeCriticalityDiagnostics(sub)
			if err != nil {
				return nil, fmt.Errorf("s1ap: ResetAcknowledge CriticalityDiagnostics: %w", err)
			}

			m.CriticalityDiagnostics = &cd
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}
	}

	return m, nil
}
