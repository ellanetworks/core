// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"fmt"
)

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

// The library owns every conversion between a wire field and the sub-fields
// 3GPP defines it as, so a caller never shifts or masks a value the library
// encodes. Both widths live here, and only their combination is correct.

// CellIdentity composes this eNB's E-UTRAN Cell Identity: the eNB ID in the
// leftmost bits of its kind, cellID in the rest (TS 36.413 §9.2.1.37). A Home
// eNB ID is itself 28 bits — "Equal to the Cell Identity IE contained in the
// E-UTRAN CGI IE of the cell served by the eNB" — so it leaves a zero-width
// cell part and only cell 0 exists.
func (e ENBID) CellIdentity(cellID uint32) (uint32, error) {
	nodeBits, ok := enbIDBits[e.Kind]
	if !ok {
		return 0, fmt.Errorf("s1ap: invalid ENB-ID kind %d", e.Kind)
	}

	if nodeBits > cellIDBits {
		return 0, fmt.Errorf("s1ap: eNB id of %d bits does not fit a %d-bit cell identity", nodeBits, cellIDBits)
	}

	cellBits := cellIDBits - nodeBits
	if uint64(cellID) >= 1<<uint(cellBits) {
		return 0, fmt.Errorf("s1ap: cell id %d exceeds the %d bits left by a %d-bit eNB id", cellID, cellBits, nodeBits)
	}

	if uint64(e.Value) >= 1<<uint(nodeBits) {
		return 0, fmt.Errorf("s1ap: eNB id %d exceeds its own %d bits", e.Value, nodeBits)
	}

	return e.Value<<uint(cellBits) | cellID, nil
}

// SplitCellIdentity is CellIdentity's inverse for a cell of this eNB.
func (e ENBID) SplitCellIdentity(eci uint32) (cellID uint32, ok bool) {
	nodeBits, known := enbIDBits[e.Kind]
	if !known || nodeBits > cellIDBits {
		return 0, false
	}

	cellBits := cellIDBits - nodeBits
	if eci>>uint(cellBits) != e.Value {
		return 0, false
	}

	return eci & (1<<uint(cellBits) - 1), true
}

// The NG-RAN node vocabulary TS 36.413 carries so an EPS to 5GS handover can
// name its target. It is the only reason these types appear in an S1AP
// specification at all.

// GNB-Identity ::= CHOICE { gNB-ID BIT STRING (SIZE(22..32)), ... }
// (TS 36.413). Unlike ENB-ID this is one alternative over a range of widths, so
// the width travels with the value rather than being implied by the alternative.
type GNBID struct {
	Value uint32
	Bits  int
}

// gnbIDBits bounds GNB-ID ::= BIT STRING (SIZE(22..32)).
const (
	gnbIDMinBits = 22
	gnbIDMaxBits = 32
)

// Global-GNB-ID ::= SEQUENCE { pLMN-Identity, gNB-ID GNB-Identity,
// iE-Extensions OPTIONAL } (extensible) (TS 36.413).
type GlobalGNBID struct {
	_            [0]struct{} `per:"extseq"`
	PLMNIdentity PLMNIdentity
	GNBID        GNBID
	_            ieExtensions `per:",skip"`
}

// GNB ::= SEQUENCE { global-gNB-ID, iE-Extensions OPTIONAL } (extensible)
// (TS 36.413).
type GNB struct {
	_           [0]struct{} `per:"extseq"`
	GlobalGNBID GlobalGNBID
	_           ieExtensions `per:",skip"`
}

// NG-eNB ::= SEQUENCE { global-ng-eNB-ID Global-ENB-ID, iE-Extensions OPTIONAL }
// (extensible) (TS 36.413). An ng-eNB is identified by the same Global-ENB-ID an
// eNB is, so this alternative reuses S1AP's own type rather than a 5GS one.
type NgENB struct {
	_             [0]struct{} `per:"extseq"`
	GlobalNgENBID GlobalENBID
	_             ieExtensions `per:",skip"`
}

// Global-RAN-NODE-ID ::= CHOICE { gNB, ng-eNB, ... } (TS 36.413). Exactly one
// alternative is set; which one is derived from the pointers rather than stored,
// so the two can never disagree.
type GlobalRANNodeID struct {
	GNB   *GNB
	NgENB *NgENB
}

// FiveGSTAC ::= OCTET STRING (SIZE(3)) (TS 36.413), held as the 24-bit number
// those octets carry. It is the 5GS tracking area code, three octets wide where
// the EPS TAC is two.
type FiveGSTAC uint32

// fiveGSTACMax is the widest value three octets hold.
const fiveGSTACMax = 1<<24 - 1

// FiveGSTAI ::= SEQUENCE { pLMNidentity, fiveGSTAC, iE-Extensions OPTIONAL }
// (extensible) (TS 36.413). The 5GS counterpart of TAI, which S1AP needs only to
// name the tracking area an EPS to 5GS handover targets.
type FiveGSTAI struct {
	_            [0]struct{} `per:"extseq"`
	PLMNIdentity PLMNIdentity
	TAC          FiveGSTAC
	_            ieExtensions `per:",skip"`
}

// Hex renders the eNB identifier as the hex digits its bit length covers,
// left-aligned in the bit string as the wire carries it.
func (e ENBID) Hex() string {
	bits := enbIDBits[e.Kind]
	b := make([]byte, (bits+7)/8)

	for i := range bits {
		if e.Value&(1<<uint(bits-1-i)) != 0 {
			b[i/8] |= 1 << uint(7-i%8)
		}
	}

	return hex.EncodeToString(b)[:(bits+3)/4]
}
