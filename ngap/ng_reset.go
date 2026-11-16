// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// TS 38.413, NGAP-Constants.
const maxnoofNGConnectionsToReset = 65536

// ResetAll ::= ENUMERATED { reset-all, ... }.
type ResetAll uint8

const ResetAllResetAll ResetAll = 0

// TS 38.413 §9.3.3.25. Both identities are optional; an item may carry either
// or both.
type UEAssociatedLogicalNGConnectionItem struct {
	AMFUENGAPID  *AMFUENGAPID           `per:",optional"`
	RANUENGAPID  *RANUENGAPID           `per:",optional"`
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

// UEAssociatedLogicalNGConnectionList ::= SEQUENCE (SIZE(1..
// maxnoofNGConnectionsToReset)) OF UE-associatedLogicalNG-connectionItem.
// A plain SEQUENCE OF: the items are not in a ProtocolIE-SingleContainer.
type UEAssociatedLogicalNGConnectionList []UEAssociatedLogicalNGConnectionItem

type ResetType struct {
	NGInterface       *ResetAll                            `per:",choice:0"`
	PartOfNGInterface *UEAssociatedLogicalNGConnectionList `per:",choice:1"`
	ChoiceExtension   *SingleContainer                     `per:",choice:2,extie"`
}

// TS 38.413 §9.2.6.11.
type NGReset struct {
	Cause     *Cause
	ResetType ResetType

	messageMeta
}

var nGResetIEs = []ieSpec[NGReset]{
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *NGReset, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *NGReset) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDResetType, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *NGReset, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ResetType)
		},
		Encode: func(m *NGReset) (per.Marshaler, bool) { return &m.ResetType, true },
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

func DecodeNGReset(value []byte) (*NGReset, error) {
	return decodeMessageBody[NGReset](ProcNGReset, nGResetIEs, value)
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
		ID: IDUEAssociatedLogicalNGConnectionList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *NGResetAcknowledge, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ConnectionList)
		},
		Encode: func(m *NGResetAcknowledge) (per.Marshaler, bool) {
			if len(m.ConnectionList) == 0 {
				return nil, false
			}

			return m.ConnectionList, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *NGResetAcknowledge, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics
			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *NGResetAcknowledge) (per.Marshaler, bool) {
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

func DecodeNGResetAcknowledge(value []byte) (*NGResetAcknowledge, error) {
	return decodeMessageBody[NGResetAcknowledge](ProcNGReset, nGResetAcknowledgeIEs, value)
}
