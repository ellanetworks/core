// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

// ENBIDKind selects an ENB-ID CHOICE alternative (TS 36.413).
type ENBIDKind uint8

const (
	ENBIDMacro      ENBIDKind = iota // root 0: BIT STRING (SIZE(20))
	ENBIDHome                        // root 1: BIT STRING (SIZE(28))
	ENBIDShortMacro                  // extension: BIT STRING (SIZE(18))
	ENBIDLongMacro                   // extension: BIT STRING (SIZE(21))
)

var enbIDBits = map[ENBIDKind]int{
	ENBIDMacro:      20,
	ENBIDHome:       28,
	ENBIDShortMacro: 18,
	ENBIDLongMacro:  21,
}

// ENB-ID ::= CHOICE { macroENB-ID, homeENB-ID, ..., short-macroENB-ID,
// long-macroENB-ID }: two root alternatives, two extension alternatives.
type ENBID struct {
	Kind  ENBIDKind
	Value uint32
}

// GlobalENBID ::= SEQUENCE { pLMNidentity, eNB-ID, iE-Extensions OPTIONAL }
// (extensible).
type GlobalENBID struct {
	_            [0]struct{} `per:"extseq"`
	PLMNIdentity PLMNIdentity
	ENBID        ENBID
	_            ieExtensions `per:",skip"`
}

// TAC ::= OCTET STRING (SIZE(2)).
type TAC uint16

// BPLMNs ::= SEQUENCE (SIZE(1..maxnoofBPLMNs)) OF PLMNidentity.
type BPLMNs []PLMNIdentity

// SupportedTAItem ::= SEQUENCE { tAC, broadcastPLMNs, iE-Extensions OPTIONAL }
// (extensible).
type SupportedTAItem struct {
	_              [0]struct{} `per:"extseq"`
	TAC            TAC
	BroadcastPLMNs BPLMNs
	_              ieExtensions `per:",skip"`
}

// SupportedTAs ::= SEQUENCE (SIZE(1..maxnoofTACs)) OF SupportedTAs-Item.
type SupportedTAs []SupportedTAItem

// PagingDRX ::= ENUMERATED { v32, v64, v128, v256, ... } (extensible).
type PagingDRX uint8

const (
	PagingDRXv32 PagingDRX = iota
	PagingDRXv64
	PagingDRXv128
	PagingDRXv256

	pagingDRXRootCount = 4
)

// maxnoof constants for S1 Setup IEs (TS 36.413, S1AP-Constants).
const (
	maxnoofTACs   = 256
	maxnoofBPLMNs = 6
)
