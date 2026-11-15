// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// TS 36.413, S1AP-Constants.
const maxnoofIndividualS1ConnectionsToReset = 256

// ResetAll ::= ENUMERATED { reset-all, ... }.
type ResetAll uint8

const ResetAllResetAll ResetAll = 0

// TS 36.413. Both identities are optional; an item may carry either or both.
type UEAssociatedLogicalS1ConnectionItem struct {
	MMEUES1APID   *MMEUES1APID           `per:",optional"`
	ENBUES1APID   *ENBUES1APID           `per:",optional"`
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
	containerCrit `per:"-"`
}

type UEAssociatedLogicalS1ConnectionListRes []UEAssociatedLogicalS1ConnectionItem

func (l UEAssociatedLogicalS1ConnectionListRes) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofIndividualS1ConnectionsToReset, IDUEAssociatedLogicalS1ConnectionItem, CriticalityReject, l)
}

func (l *UEAssociatedLogicalS1ConnectionListRes) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[UEAssociatedLogicalS1ConnectionItem](r, enc, maxnoofIndividualS1ConnectionsToReset, IDUEAssociatedLogicalS1ConnectionItem, CriticalityReject)
	*l = items

	return err
}

// ResetType ::= CHOICE { s1-Interface ResetAll, partOfS1-Interface
// UE-associatedLogicalS1-ConnectionListRes, ... } (TS 36.413 §9.1.8.1).
type ResetType struct {
	S1Interface       *ResetAll                               `per:",choice:0"`
	PartOfS1Interface *UEAssociatedLogicalS1ConnectionListRes `per:",choice:1"`
	Ext               *per.ExtensionAlternative               `per:",extalt"`
}

// TS 36.413 §9.1.8.1.
type Reset struct {
	Cause     *Cause
	ResetType ResetType

	messageMeta
}

var resetIEs = []ieSpec[Reset]{
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *Reset, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *Reset) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDResetType, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *Reset, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ResetType)
		},
		Encode: func(m *Reset) (per.Marshaler, bool) { return &m.ResetType, true },
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

func DecodeReset(value []byte) (*Reset, error) {
	return decodeMessageBody[Reset](ProcReset, resetIEs, value)
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
		ID: IDUEAssociatedLogicalS1ConnectionListResAck, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ResetAcknowledge, r *per.Reader, enc per.Encoding) error {
			items, err := decodeItemList[UEAssociatedLogicalS1ConnectionItem](r, enc, maxnoofIndividualS1ConnectionsToReset, IDUEAssociatedLogicalS1ConnectionItem, CriticalityIgnore)
			if err != nil {
				return err
			}

			m.ConnectionList = append(m.ConnectionList, items...)

			return nil
		},
		Encode: func(m *ResetAcknowledge) (per.Marshaler, bool) {
			if len(m.ConnectionList) == 0 {
				return nil, false
			}

			return containerList(maxnoofIndividualS1ConnectionsToReset, IDUEAssociatedLogicalS1ConnectionItem, CriticalityIgnore, m.ConnectionList), true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ResetAcknowledge, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics
			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *ResetAcknowledge) (per.Marshaler, bool) {
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

func DecodeResetAcknowledge(value []byte) (*ResetAcknowledge, error) {
	return decodeMessageBody[ResetAcknowledge](ProcReset, resetAcknowledgeIEs, value)
}
