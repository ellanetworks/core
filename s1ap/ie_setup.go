// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "github.com/ellanetworks/core/per"

// TAC ::= OCTET STRING (SIZE(2)).
type TAC uint16

// BPLMNs ::= SEQUENCE (SIZE(1..maxnoofBPLMNs)) OF PLMNidentity.
type BPLMNs []PLMNIdentity

// SupportedTAItem ::= SEQUENCE { tAC, broadcastPLMNs, iE-Extensions OPTIONAL }
// (extensible).
type SupportedTAItem struct {
	extensions     per.ExtensionAdditions `per:",extraw"`
	TAC            TAC
	BroadcastPLMNs BPLMNs
	RATType        *RATType      `per:",extie:IDRATType,crit:reject"`
	iEExtensions   extensionMeta `per:",exttable"`
}

// SupportedTAs ::= SEQUENCE (SIZE(1..maxnoofTACs)) OF SupportedTAs-Item.
type SupportedTAs []SupportedTAItem

// UERetentionInformation ::= ENUMERATED { ues-retained, ... } (extensible) —
// TS 36.413 §9.2.1.108. Modeled but never accepted: Ella Core retains no UE
// context across S1 Setup.
type UERetentionInformation uint8

const (
	UERetentionUesRetained UERetentionInformation = iota
)

// PagingDRX ::= ENUMERATED { v32, v64, v128, v256, ... } (extensible).
type PagingDRX uint8

const (
	PagingDRXv32 PagingDRX = iota
	PagingDRXv64
	PagingDRXv128
	PagingDRXv256
)

// maxnoof constants for S1 Setup IEs (TS 36.413, S1AP-Constants).
const (
	maxnoofTACs   = 256
	maxnoofBPLMNs = 6
)
