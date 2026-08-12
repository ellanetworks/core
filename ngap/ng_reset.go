// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// TS 38.413, NGAP-Constants.
const maxnoofNGConnectionsToReset = 65536

// ResetType CHOICE alternatives, defined inline in the NG RESET of TS 38.413
// §9.2.6.11 rather than in a clause of its own. The CHOICE is not
// extensible, so choice-Extensions is a plain alternative index.
const (
	resetTypeNGInterface = iota
	resetTypePartOfNGInterface
	resetTypeChoiceExtensions

	resetTypeAlternatives = 3
)

// ResetAll ::= ENUMERATED { reset-all, ... }.
const resetAllRootCount = 1

// TS 38.413 §9.3.3.25. Both identities are optional; an item may carry either
// or both.
type UEAssociatedLogicalNGConnectionItem struct {
	_           [0]struct{}  `per:"extseq"`
	AMFUENGAPID *AMFUENGAPID `per:",optional"`
	RANUENGAPID *RANUENGAPID `per:",optional"`
	_           ieExtensions `per:",skip"`
}

// UEAssociatedLogicalNGConnectionList ::= SEQUENCE (SIZE(1..
// maxnoofNGConnectionsToReset)) OF UE-associatedLogicalNG-connectionItem.
// A plain SEQUENCE OF: the items are not in a ProtocolIE-SingleContainer.
type UEAssociatedLogicalNGConnectionList []UEAssociatedLogicalNGConnectionItem

// All selects nG-Interface, resetting the whole interface; otherwise Part
// selects partOfNG-Interface, the connections to reset.
type ResetType struct {
	All  bool
	Part UEAssociatedLogicalNGConnectionList
}

func (t ResetType) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if t.All {
		if err := per.EncodeConstrainedWholeNumber(w, enc, 0, resetTypeAlternatives-1, resetTypeNGInterface); err != nil {
			return err
		}

		return encodeRootEnumerated(w, enc, resetAllRootCount, 0, "ResetAll")
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, resetTypeAlternatives-1, resetTypePartOfNGInterface); err != nil {
		return err
	}

	return t.Part.MarshalPER(w, enc)
}

func (t *ResetType) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, resetTypeAlternatives-1)
	if err != nil {
		return fmt.Errorf("ngap: ResetType choice: %w", err)
	}

	switch idx {
	case resetTypeNGInterface:
		if _, err := decodeRootEnumerated(r, enc, resetAllRootCount, "ResetAll"); err != nil {
			return fmt.Errorf("ngap: ResetAll: %w", err)
		}

		*t = ResetType{All: true}

		return nil
	case resetTypeChoiceExtensions:
		return decodeChoiceExtension(r, enc, "ResetType")
	default:
		var items UEAssociatedLogicalNGConnectionList
		if err := items.UnmarshalPER(r, enc); err != nil {
			return err
		}

		*t = ResetType{Part: items}

		return nil
	}
}

// TS 38.413 §9.2.6.11.
type NGReset struct {
	Cause     *Cause
	ResetType ResetType

	messageMeta
}

var nGResetIEs = []ieSpec[NGReset]{
	{
		id: IDCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *NGReset, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *NGReset) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: IDResetType, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *NGReset, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ResetType)
		},
		encode: func(m *NGReset) (per.Marshaler, bool) { return &m.ResetType, true },
	},
}

func (m *NGReset) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcNGReset, nGResetIEs, m)
}

func (m *NGReset) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcNGReset,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseNGReset(value []byte) (*NGReset, error) {
	return parseMessageBody[NGReset](ProcNGReset, TriggeringInitiatingMessage, nGResetIEs, value)
}

// TS 38.413 §9.2.6.12.
type NGResetAcknowledge struct {
	ConnectionList         UEAssociatedLogicalNGConnectionList
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

// The message has no mandatory IE.
var nGResetAcknowledgeIEs = []ieSpec[NGResetAcknowledge]{
	{
		id: IDUEAssociatedLogicalNGConnectionList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *NGResetAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ConnectionList)
		},
		encode: func(m *NGResetAcknowledge) (per.Marshaler, bool) {
			if len(m.ConnectionList) == 0 {
				return nil, false
			}

			return m.ConnectionList, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *NGResetAcknowledge, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics
			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *NGResetAcknowledge) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *NGResetAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcNGReset, nGResetAcknowledgeIEs, m)
}

func (m *NGResetAcknowledge) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcNGReset,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseNGResetAcknowledge(value []byte) (*NGResetAcknowledge, error) {
	return parseMessageBody[NGResetAcknowledge](ProcNGReset, TriggeringSuccessfulOutcome, nGResetAcknowledgeIEs, value)
}
