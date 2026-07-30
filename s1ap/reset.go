// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// maxnoofIndividualS1ConnectionsToReset bounds the UE-associated logical
// S1-connection lists in Reset/Reset Acknowledge (TS 36.413).
const maxnoofIndividualS1ConnectionsToReset = 256

// resetTypeChoiceRootCount is the number of root alternatives of the ResetType
// CHOICE: s1-Interface and partOfS1-Interface (TS 36.413).
const resetTypeChoiceRootCount = 2

// resetAllRootCount is the number of root values of ResetAll ENUMERATED
// { reset-all, ... } (TS 36.413).
const resetAllRootCount = 1

// UEAssociatedLogicalS1ConnectionItem identifies one UE-associated logical
// S1-connection by its MME-UE-S1AP-ID and/or eNB-UE-S1AP-ID (TS 36.413).
// Both identities are optional; an item may carry either or both.
type UEAssociatedLogicalS1ConnectionItem struct {
	_           [0]struct{}  `per:"extseq"`
	MMEUES1APID *MMEUES1APID `per:",optional"`
	ENBUES1APID *ENBUES1APID `per:",optional"`
	_           ieExtensions `per:",skip"`
}

// ResetType is the ResetType CHOICE (TS 36.413): All selects
// s1-Interface (ResetAll, value reset-all), reset of the whole S1 interface;
// otherwise Part selects partOfS1-Interface, the UE-associated logical
// S1-connections to reset.
type ResetType struct {
	All  bool
	Part []UEAssociatedLogicalS1ConnectionItem
}

func (t ResetType) MarshalPER(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	if t.All {
		if err := per.EncodeConstrainedWholeNumber(w, enc, 0, resetTypeChoiceRootCount-1, 0); err != nil {
			return err
		}

		return per.EncodeEnumerated(w, enc, resetAllRootCount, true, 0)
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, resetTypeChoiceRootCount-1, 1); err != nil {
		return err
	}

	return encodeSingleContainerList(w, enc, maxnoofIndividualS1ConnectionsToReset, idUEAssociatedLogicalS1ConnectionItem, CriticalityReject, t.Part)
}

func (t *ResetType) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	isExt, err := r.ReadBit()
	if err != nil {
		return fmt.Errorf("s1ap: ResetType choice: %w", err)
	}

	if isExt {
		return fmt.Errorf("s1ap: ResetType extension alternative unsupported")
	}

	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, resetTypeChoiceRootCount-1)
	if err != nil {
		return fmt.Errorf("s1ap: ResetType choice: %w", err)
	}

	switch idx {
	case 0:
		if _, err := per.DecodeEnumerated(r, enc, resetAllRootCount, true); err != nil {
			return fmt.Errorf("s1ap: ResetAll: %w", err)
		}

		*t = ResetType{All: true}

		return nil
	default:
		items, err := decodeItemList[UEAssociatedLogicalS1ConnectionItem](r, enc, maxnoofIndividualS1ConnectionsToReset)
		if err != nil {
			return err
		}

		*t = ResetType{Part: items}

		return nil
	}
}

// Reset is the RESET message (TS 36.413), sent by the eNB or MME to
// reset the whole S1 interface or a subset of its UE-associated logical
// connections.
type Reset struct {
	Cause     Cause
	ResetType ResetType

	unmodeledIEs
}

func (m *Reset) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idCause, crit: CriticalityIgnore, val: &m.Cause},
		{id: idResetType, crit: CriticalityReject, val: &m.ResetType},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *Reset) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcReset,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseReset decodes the message from an initiatingMessage open-type payload.
func ParseReset(value []byte) (*Reset, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: Reset preamble: %w", err)
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

	m := &Reset{}

	var seenCause, seenResetType bool

	for _, f := range fields {
		switch f.id {
		case idCause:
			err = perIEDecode(f.value, &m.Cause)
			seenCause = true
		case idResetType:
			err = perIEDecode(f.value, &m.ResetType)
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

// ResetAcknowledge is the RESET ACKNOWLEDGE message (TS 36.413). The
// ConnectionList is present only in answer to a partOfS1-Interface reset, where
// it echoes the UE-associated logical S1-connections that were reset.
type ResetAcknowledge struct {
	ConnectionList         []UEAssociatedLogicalS1ConnectionItem
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *ResetAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	var fields []ieField

	if len(m.ConnectionList) > 0 {
		fields = append(fields, ieField{id: idUEAssociatedLogicalS1ConnectionListResAck, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofIndividualS1ConnectionsToReset, idUEAssociatedLogicalS1ConnectionItem, CriticalityIgnore, m.ConnectionList)
		})})
	}

	if m.CriticalityDiagnostics != nil {
		d := *m.CriticalityDiagnostics
		fields = append(fields, ieField{id: idCriticalityDiagnostics, crit: CriticalityIgnore, val: &d})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ResetAcknowledge) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcReset,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseResetAcknowledge decodes the message from a successfulOutcome open-type
// payload.
func ParseResetAcknowledge(value []byte) (*ResetAcknowledge, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: ResetAcknowledge preamble: %w", err)
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

	m := &ResetAcknowledge{}

	for _, f := range fields {
		switch f.id {
		case idUEAssociatedLogicalS1ConnectionListResAck:
			items, err := decodeItemList[UEAssociatedLogicalS1ConnectionItem](per.NewReader(f.value), enc, maxnoofIndividualS1ConnectionsToReset)
			if err != nil {
				return nil, fmt.Errorf("s1ap: ResetAcknowledge connection list: %w", err)
			}

			m.ConnectionList = append(m.ConnectionList, items...)
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			if err := perIEDecode(f.value, &cd); err != nil {
				return nil, fmt.Errorf("s1ap: ResetAcknowledge CriticalityDiagnostics: %w", err)
			}

			m.CriticalityDiagnostics = &cd
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}
	}

	return m, nil
}
