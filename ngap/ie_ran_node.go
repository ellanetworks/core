// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"fmt"
)

// GlobalRANNodeID ::= CHOICE { globalGNB-ID, globalNgENB-ID, globalN3IWF-ID,
// choice-Extensions } root alternatives (TS 38.413 §9.3.1.5). The CHOICE is
// not extensible, so choice-Extensions is a plain alternative index.
const (
	globalGNBID = iota
	globalNgENBID
	globalN3IWFID
	globalRANNodeIDChoiceExtensions

	globalRANNodeIDAlternatives = 4
)

// Selects a GlobalRANNodeID alternative together with the nested node-id
// CHOICE alternative. The two levels are flattened because every combination
// names one kind of RAN node.
type RANNodeIDKind uint8

const (
	// RANNodeIDGNB is globalGNB-ID / gNB-ID: BIT STRING (SIZE(22..32)).
	RANNodeIDGNB RANNodeIDKind = iota
	// RANNodeIDMacroNgENB is globalNgENB-ID / macroNgENB-ID: SIZE(20).
	RANNodeIDMacroNgENB
	// RANNodeIDShortMacroNgENB is globalNgENB-ID / shortMacroNgENB-ID: SIZE(18).
	RANNodeIDShortMacroNgENB
	// RANNodeIDLongMacroNgENB is globalNgENB-ID / longMacroNgENB-ID: SIZE(21).
	RANNodeIDLongMacroNgENB
	// RANNodeIDN3IWF is globalN3IWF-ID / n3IWF-ID: BIT STRING (SIZE(16)).
	RANNodeIDN3IWF
)

// How one kind sits on the wire.
type ranNodeIDShape struct {
	outer  int
	inner  int
	lb, ub int
}

var ranNodeIDShapes = map[RANNodeIDKind]ranNodeIDShape{
	RANNodeIDGNB:             {globalGNBID, 0, 22, 32},
	RANNodeIDMacroNgENB:      {globalNgENBID, 0, 20, 20},
	RANNodeIDShortMacroNgENB: {globalNgENBID, 1, 18, 18},
	RANNodeIDLongMacroNgENB:  {globalNgENBID, 2, 21, 21},
	RANNodeIDN3IWF:           {globalN3IWFID, 0, 16, 16},
}

// nodeIDAlternatives is the number of alternatives in each Global*-ID's nested
// node-id CHOICE, including its choice-Extensions. None is extensible.
var nodeIDAlternatives = map[int]int{
	globalGNBID:   2, // gNB-ID, choice-Extensions
	globalNgENBID: 4, // macroNgENB-ID, shortMacroNgENB-ID, longMacroNgENB-ID, choice-Extensions
	globalN3IWFID: 2, // n3IWF-ID, choice-Extensions
}

var nodeIDChoiceName = map[int]string{
	globalGNBID:   "GNB-ID",
	globalNgENBID: "NgENB-ID",
	globalN3IWFID: "N3IWF-ID",
}

var kindForShape = func() map[[2]int]RANNodeIDKind {
	m := make(map[[2]int]RANNodeIDKind, len(ranNodeIDShapes))
	for kind, s := range ranNodeIDShapes {
		m[[2]int{s.outer, s.inner}] = kind
	}

	return m
}()

// TS 38.413 §9.3.1.5. Each of globalGNB-ID, globalNgENB-ID and globalN3IWF-ID
// is an extensible SEQUENCE of a PLMN identity and a node-id CHOICE, so the
// three flatten into one struct without losing anything the wire carries.
type GlobalRANNodeID struct {
	Kind         RANNodeIDKind
	PLMNIdentity PLMNIdentity

	// Value is the node identifier, right-aligned in Bits bits.
	Value uint32

	// Only RANNodeIDGNB has a range to choose from (22..32); every other kind
	// has one legal length, and encoding rejects anything else.
	Bits int
}

// The library owns every conversion between a wire field and the sub-fields
// 3GPP defines it as, so a caller never shifts or masks a value the library
// encodes. The cell-identity widths and the node-id width both live here, and
// only their combination is correct.

// NRCellIdentity composes this node's NR Cell Identity: the gNB ID in the
// leftmost Bits, cellID in the remaining NRCellIdentityBits-Bits (TS 23.003
// §19.6). The split point is the gNB ID's own length, which only this type
// carries — which is why composing by hand goes wrong.
func (g GlobalRANNodeID) NRCellIdentity(cellID uint64) (uint64, error) {
	return g.cellIdentity(cellID, NRCellIdentityBits, "NR")
}

// EUTRACellIdentity is NRCellIdentity for an ng-eNB's E-UTRA cell.
func (g GlobalRANNodeID) EUTRACellIdentity(cellID uint64) (uint64, error) {
	return g.cellIdentity(cellID, EUTRACellIdentityBits, "E-UTRA")
}

func (g GlobalRANNodeID) cellIdentity(cellID uint64, width int, kind string) (uint64, error) {
	// Unlike S1AP, whose 28-bit Home eNB ID is itself the cell identity, no NGAP
	// node kind is as wide as either cell identity (gNB-ID tops out at 32 of 36,
	// ng-eNB at 21 of 28), so a node filling it is always an error.
	if g.Bits <= 0 || g.Bits >= width {
		return 0, fmt.Errorf("ngap: node id of %d bits leaves no room in a %d-bit %s cell identity", g.Bits, width, kind)
	}

	cellBits := width - g.Bits
	if cellID >= 1<<uint(cellBits) {
		return 0, fmt.Errorf("ngap: cell id %d exceeds the %d bits left by a %d-bit node id", cellID, cellBits, g.Bits)
	}

	if g.Bits < 64 && uint64(g.Value) >= 1<<uint(g.Bits) {
		return 0, fmt.Errorf("ngap: node id %d exceeds its own %d bits", g.Value, g.Bits)
	}

	return uint64(g.Value)<<uint(cellBits) | cellID, nil
}

// SplitNRCellIdentity is NRCellIdentity's inverse for a cell of this node.
func (g GlobalRANNodeID) SplitNRCellIdentity(nci uint64) (cellID uint64, ok bool) {
	return g.splitCellIdentity(nci, NRCellIdentityBits)
}

// SplitEUTRACellIdentity is EUTRACellIdentity's inverse.
func (g GlobalRANNodeID) SplitEUTRACellIdentity(eci uint64) (cellID uint64, ok bool) {
	return g.splitCellIdentity(eci, EUTRACellIdentityBits)
}

func (g GlobalRANNodeID) splitCellIdentity(id uint64, width int) (uint64, bool) {
	if g.Bits <= 0 || g.Bits >= width {
		return 0, false
	}

	cellBits := width - g.Bits
	if id>>uint(cellBits) != uint64(g.Value) {
		return 0, false
	}

	return id & (1<<uint(cellBits) - 1), true
}

// Hex renders the node identifier as the hex digits its bit length covers,
// left-aligned in the bit string as the wire carries it.
func (g GlobalRANNodeID) Hex() string {
	b := make([]byte, (g.Bits+7)/8)

	for i := range g.Bits {
		if g.Value&(1<<uint(g.Bits-1-i)) != 0 {
			b[i/8] |= 1 << uint(7-i%8)
		}
	}

	return hex.EncodeToString(b)[:(g.Bits+3)/4]
}
