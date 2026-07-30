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

// resetIEs is the Reset IE table (TS 36.413).
var resetIEs = []ieSpec[Reset]{
	{
		id: idCause, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *Reset, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.Cause)
		},
		encode: func(m *Reset) (per.Marshaler, bool) { return &m.Cause, true },
	},
	{
		id: idResetType, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *Reset, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ResetType)
		},
		encode: func(m *Reset) (per.Marshaler, bool) { return &m.ResetType, true },
	},
}

func (m *Reset) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, resetIEs, m)
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
	return parseMessageBody[Reset](ProcReset, resetIEs, value)
}

// ResetAcknowledge is the RESET ACKNOWLEDGE message (TS 36.413). The
// ConnectionList is present only in answer to a partOfS1-Interface reset, where
// it echoes the UE-associated logical S1-connections that were reset.
type ResetAcknowledge struct {
	ConnectionList         []UEAssociatedLogicalS1ConnectionItem
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

// resetAcknowledgeIEs is the RESET ACKNOWLEDGE IE table (TS 36.413 §9.1.8.2).
// The message has no mandatory IE.
var resetAcknowledgeIEs = []ieSpec[ResetAcknowledge]{
	{
		id: idUEAssociatedLogicalS1ConnectionListResAck, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ResetAcknowledge, raw []byte, enc per.Encoding) error {
			items, err := decodeItemList[UEAssociatedLogicalS1ConnectionItem](per.NewReader(raw), enc, maxnoofIndividualS1ConnectionsToReset)
			if err != nil {
				return err
			}

			m.ConnectionList = append(m.ConnectionList, items...)

			return nil
		},
		encode: func(m *ResetAcknowledge) (per.Marshaler, bool) {
			if len(m.ConnectionList) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofIndividualS1ConnectionsToReset, idUEAssociatedLogicalS1ConnectionItem, CriticalityIgnore, m.ConnectionList)
			}), true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ResetAcknowledge, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics
			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *ResetAcknowledge) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *ResetAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, resetAcknowledgeIEs, m)
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
	return parseMessageBody[ResetAcknowledge](ProcReset, resetAcknowledgeIEs, value)
}
