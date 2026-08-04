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
// leftmost bits of its kind, cellID in the rest (TS 23.003 §19.4.2.3).
func (e ENBID) CellIdentity(cellID uint32) (uint32, error) {
	nodeBits, ok := enbIDBits[e.Kind]
	if !ok {
		return 0, fmt.Errorf("s1ap: invalid ENB-ID kind %d", e.Kind)
	}

	if nodeBits >= cellIDBits {
		return 0, fmt.Errorf("s1ap: eNB id of %d bits leaves no room in a %d-bit cell identity", nodeBits, cellIDBits)
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
	if !known || nodeBits >= cellIDBits {
		return 0, false
	}

	cellBits := cellIDBits - nodeBits
	if eci>>uint(cellBits) != e.Value {
		return 0, false
	}

	return eci & (1<<uint(cellBits) - 1), true
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
