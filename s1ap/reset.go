// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// TS 36.413, S1AP-Constants.
const maxnoofIndividualS1ConnectionsToReset = 256

// ResetType CHOICE root alternatives, defined inline in the RESET of TS 36.413
// §9.1.8.1. The CHOICE is extensible, so an extension bit precedes the index.
const (
	resetTypeS1Interface = iota
	resetTypePartOfS1Interface

	resetTypeChoiceRootCount = 2
)

// ResetAll ::= ENUMERATED { reset-all, ... }.
const resetAllRootCount = 1

// TS 36.413. Both identities are optional; an item may carry either or both.
type UEAssociatedLogicalS1ConnectionItem struct {
	_           [0]struct{}  `per:"extseq"`
	MMEUES1APID *MMEUES1APID `per:",optional"`
	ENBUES1APID *ENBUES1APID `per:",optional"`
	_           ieExtensions `per:",skip"`
}

// All selects s1-Interface, resetting the whole interface; otherwise Part
// selects partOfS1-Interface, the connections to reset.
type ResetType struct {
	All  bool
	Part []UEAssociatedLogicalS1ConnectionItem
}

func (t ResetType) MarshalPER(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	if t.All {
		if err := per.EncodeConstrainedWholeNumber(w, enc, 0, resetTypeChoiceRootCount-1, resetTypeS1Interface); err != nil {
			return err
		}

		return encodeRootEnumerated(w, enc, resetAllRootCount, 0, "ResetAll")
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, resetTypeChoiceRootCount-1, resetTypePartOfS1Interface); err != nil {
		return err
	}

	return encodeSingleContainerList(w, enc, maxnoofIndividualS1ConnectionsToReset, IDUEAssociatedLogicalS1ConnectionItem, CriticalityReject, t.Part)
}

func (t *ResetType) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	isExt, err := r.ReadBit()
	if err != nil {
		return fmt.Errorf("s1ap: ResetType choice: %w", err)
	}

	if isExt {
		// §10.3.1 case 6: handled on criticality, not by abandoning the RESET.
		return fmt.Errorf("%w: ResetType extension alternative", errNotComprehended)
	}

	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, resetTypeChoiceRootCount-1)
	if err != nil {
		return fmt.Errorf("s1ap: ResetType choice: %w", err)
	}

	switch idx {
	case resetTypeS1Interface:
		if _, err := decodeRootEnumerated(r, enc, resetAllRootCount, "ResetAll"); err != nil {
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

// TS 36.413 §9.1.8.1.
type Reset struct {
	Cause     *Cause
	ResetType ResetType

	messageMeta
}

var resetIEs = []ieSpec[Reset]{
	{
		id: IDCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *Reset, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *Reset) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: IDResetType, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *Reset, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ResetType)
		},
		encode: func(m *Reset) (per.Marshaler, bool) { return &m.ResetType, true },
	},
}

func (m *Reset) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcReset, resetIEs, m)
}

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

func ParseReset(value []byte) (*Reset, error) {
	return parseMessageBody[Reset](ProcReset, TriggeringInitiatingMessage, resetIEs, value)
}

// TS 36.413 §9.1.8.2.
type ResetAcknowledge struct {
	ConnectionList         []UEAssociatedLogicalS1ConnectionItem
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

// The message has no mandatory IE.
var resetAcknowledgeIEs = []ieSpec[ResetAcknowledge]{
	{
		id: IDUEAssociatedLogicalS1ConnectionListResAck, presence: presenceOptional, crit: CriticalityIgnore,
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
				return encodeSingleContainerList(w, enc, maxnoofIndividualS1ConnectionsToReset, IDUEAssociatedLogicalS1ConnectionItem, CriticalityIgnore, m.ConnectionList)
			}), true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
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
	return encodeMessageBody(w, enc, ProcReset, resetAcknowledgeIEs, m)
}

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

func ParseResetAcknowledge(value []byte) (*ResetAcknowledge, error) {
	return parseMessageBody[ResetAcknowledge](ProcReset, TriggeringSuccessfulOutcome, resetAcknowledgeIEs, value)
}
