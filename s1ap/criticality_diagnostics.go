// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// maxnoofErrors bounds CriticalityDiagnostics-IE-List (TS 36.413).
const maxnoofErrors = 256

// TypeOfError ::= ENUMERATED { not-understood, missing, ... } (extensible).
type TypeOfError uint8

const (
	TypeOfErrorNotUnderstood TypeOfError = iota
	TypeOfErrorMissing
)

// CriticalityDiagnosticsIEItem reports one offending IE (TS 36.413).
type CriticalityDiagnosticsIEItem struct {
	extensions    per.ExtensionAdditions `per:",extraw"`
	IECriticality Criticality            `per:"ENUMERATED,range:0..2"`
	IEID          ProtocolIEID           `per:",range:0..65535"`
	TypeOfError   TypeOfError
	iEExtensions  extensionMeta `per:",exttable"`
}

// CriticalityDiagnostics ::= SEQUENCE (extensible) with five optional fields.
type CriticalityDiagnostics struct {
	ProcedureCode             *ProcedureCode                 `per:",optional,range:0..255"`
	TriggeringMessage         *TriggeringMessage             `per:",optional"`
	ProcedureCriticality      *Criticality                   `per:"ENUMERATED,range:0..2,optional"`
	IEsCriticalityDiagnostics []CriticalityDiagnosticsIEItem `per:"SEQUENCE-OF,size:1..256,optional"`
	iEExtensions              extensionMeta                  `per:",exttable"`
	extensions                per.ExtensionAdditions         `per:",extraw"`
}
