// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// maxnoofErrors bounds CriticalityDiagnostics-IE-List (TS 38.413).
const maxnoofErrors = 256

// TypeOfError ::= ENUMERATED { not-understood, missing, ... } (extensible).
type TypeOfError uint8

const (
	TypeOfErrorNotUnderstood TypeOfError = iota
	TypeOfErrorMissing
)

// CriticalityDiagnosticsIEItem reports one offending IE (TS 38.413 §9.3.1.3).
type CriticalityDiagnosticsIEItem struct {
	extensions    per.ExtensionAdditions `per:",extraw"`
	IECriticality Criticality            `per:"ENUMERATED,range:0..2"`
	IEID          ProtocolIEID           `per:",range:0..65535"`
	TypeOfError   TypeOfError
	iEExtensions  extensionMeta `per:",exttable"`
}

type CriticalityDiagnosticsIEList []CriticalityDiagnosticsIEItem

func (l CriticalityDiagnosticsIEList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofErrors, []CriticalityDiagnosticsIEItem(l))
}

func (l *CriticalityDiagnosticsIEList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[CriticalityDiagnosticsIEItem](r, enc, 1, maxnoofErrors)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

// CriticalityDiagnostics ::= SEQUENCE (extensible) with five optional fields.
type CriticalityDiagnostics struct {
	ProcedureCode             *ProcedureCode               `per:",optional,range:0..255"`
	TriggeringMessage         *TriggeringMessage           `per:",optional"`
	ProcedureCriticality      *Criticality                 `per:"ENUMERATED,optional,range:0..2"`
	IEsCriticalityDiagnostics CriticalityDiagnosticsIEList `per:",optional"`
	iEExtensions              extensionMeta                `per:",exttable"`
	extensions                per.ExtensionAdditions       `per:",extraw"`
}
