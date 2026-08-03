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
