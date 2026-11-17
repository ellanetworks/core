// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// ENB-ID ::= CHOICE { macroENB-ID, homeENB-ID, ..., short-macroENB-ID,
// long-macroENB-ID }: two root alternatives, two extension alternatives.
type ENBID struct {
	MacroENBID      *MacroENBID               `per:",choice:0"`
	HomeENBID       *HomeENBID                `per:",choice:1"`
	ShortMacroENBID *ShortMacroENBID          `per:",choice:0,ext"`
	LongMacroENBID  *LongMacroENBID           `per:",choice:1,ext"`
	Ext             *per.ExtensionAlternative `per:",extalt"`
}

type MacroENBID uint32

type HomeENBID uint32

type ShortMacroENBID uint32

type LongMacroENBID uint32

const (
	macroENBIDBits      = 20
	homeENBIDBits       = 28
	shortMacroENBIDBits = 18
	longMacroENBIDBits  = 21
)

func (e ENBID) idBits() (value uint32, nbits int, ok bool) {
	switch {
	case e.MacroENBID != nil:
		return uint32(*e.MacroENBID), macroENBIDBits, true
	case e.HomeENBID != nil:
		return uint32(*e.HomeENBID), homeENBIDBits, true
	case e.ShortMacroENBID != nil:
		return uint32(*e.ShortMacroENBID), shortMacroENBIDBits, true
	case e.LongMacroENBID != nil:
		return uint32(*e.LongMacroENBID), longMacroENBIDBits, true
	default:
		return 0, 0, false
	}
}

func (e ENBID) Value() (uint32, bool) {
	v, _, ok := e.idBits()

	return v, ok
}

func (e ENBID) Bits() int {
	_, n, _ := e.idBits()

	return n
}

func (e ENBID) AlternativeName() string {
	switch {
	case e.MacroENBID != nil:
		return "macroENB-ID"
	case e.HomeENBID != nil:
		return "homeENB-ID"
	case e.ShortMacroENBID != nil:
		return "short-macroENB-ID"
	case e.LongMacroENBID != nil:
		return "long-macroENB-ID"
	default:
		return ""
	}
}

// GlobalENBID ::= SEQUENCE { pLMNidentity, eNB-ID, iE-Extensions OPTIONAL }
// (extensible).
type GlobalENBID struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity PLMNIdentity
	ENBID        ENBID
	iEExtensions extensionMeta `per:",exttable"`
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
	value, nodeBits, ok := e.idBits()
	if !ok {
		return 0, fmt.Errorf("s1ap: ENB-ID carries no known alternative")
	}

	if nodeBits > cellIDBits {
		return 0, fmt.Errorf("s1ap: eNB id of %d bits does not fit a %d-bit cell identity", nodeBits, cellIDBits)
	}

	cellBits := cellIDBits - nodeBits
	if uint64(cellID) >= 1<<uint(cellBits) {
		return 0, fmt.Errorf("s1ap: cell id %d exceeds the %d bits left by a %d-bit eNB id", cellID, cellBits, nodeBits)
	}

	if uint64(value) >= 1<<uint(nodeBits) {
		return 0, fmt.Errorf("s1ap: eNB id %d exceeds its own %d bits", value, nodeBits)
	}

	return value<<uint(cellBits) | cellID, nil
}

// SplitCellIdentity is CellIdentity's inverse for a cell of this eNB.
func (e ENBID) SplitCellIdentity(eci uint32) (cellID uint32, ok bool) {
	value, nodeBits, known := e.idBits()
	if !known || nodeBits > cellIDBits {
		return 0, false
	}

	cellBits := cellIDBits - nodeBits
	if eci>>uint(cellBits) != value {
		return 0, false
	}

	return eci & (1<<uint(cellBits) - 1), true
}

// The NG-RAN node vocabulary TS 36.413 carries so an EPS to 5GS handover can
// name its target. It is the only reason these types appear in an S1AP
// specification at all.

// GNB-Identity ::= CHOICE { gNB-ID GNB-ID, ... } (TS 36.413).
type GNBIdentity struct {
	GNBID *GNBID                    `per:",choice:0"`
	Ext   *per.ExtensionAlternative `per:",extalt"`
}

// GNB-ID ::= BIT STRING (SIZE(22..32)). Unlike ENB-ID this is one alternative
// over a range of widths, so the width travels with the value rather than being
// implied by the alternative.
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
	extensions   per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity PLMNIdentity
	GNBID        GNBIdentity
	iEExtensions extensionMeta `per:",exttable"`
}

// GNB ::= SEQUENCE { global-gNB-ID, iE-Extensions OPTIONAL } (extensible)
// (TS 36.413).
type GNB struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	GlobalGNBID  GlobalGNBID
	iEExtensions extensionMeta `per:",exttable"`
}

// NG-eNB ::= SEQUENCE { global-ng-eNB-ID Global-ENB-ID, iE-Extensions OPTIONAL }
// (extensible) (TS 36.413). An ng-eNB is identified by the same Global-ENB-ID an
// eNB is, so this alternative reuses S1AP's own type rather than a 5GS one.
type NgENB struct {
	extensions    per.ExtensionAdditions `per:",extraw"`
	GlobalNgENBID GlobalENBID
	iEExtensions  extensionMeta `per:",exttable"`
}

// Global-RAN-NODE-ID ::= CHOICE { gNB, ng-eNB, ... } (TS 36.413). Exactly one
// alternative is set; which one is derived from the pointers rather than stored,
// so the two can never disagree.
type GlobalRANNodeID struct {
	GNB   *GNB                      `per:",choice:0"`
	NgENB *NgENB                    `per:",choice:1"`
	Ext   *per.ExtensionAlternative `per:",extalt"`
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
	extensions   per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity PLMNIdentity
	TAC          FiveGSTAC
	iEExtensions extensionMeta `per:",exttable"`
}

func (e ENBID) Hex() string {
	value, bits, ok := e.idBits()
	if !ok {
		return ""
	}

	return fmt.Sprintf("%0*x", (bits+3)/4, value)
}

func (e ENBID) Prefix() string {
	switch {
	case e.MacroENBID != nil:
		return "MacroeNB-"
	case e.HomeENBID != nil:
		return "HomeeNB-"
	case e.ShortMacroENBID != nil:
		return "SMacroeNB-"
	case e.LongMacroENBID != nil:
		return "LMacroeNB-"
	}

	return ""
}

func (e ENBID) String() string {
	hex := e.Hex()
	if hex == "" {
		return ""
	}

	return e.Prefix() + hex
}
