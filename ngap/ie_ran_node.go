// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

type GlobalRANNodeID struct {
	GlobalGNBID       *GlobalGNBID       `per:",choice:0"`
	GlobalNgENBID     *GlobalNgENBID     `per:",choice:1"`
	GlobalN3IWFID     *GlobalN3IWFID     `per:",choice:2"`
	GlobalTNGFID      *GlobalTNGFID      `per:",choice:3,extie:IDGlobalTNGFID,crit:reject"`
	GlobalTWIFID      *GlobalTWIFID      `per:",choice:3,extie:IDGlobalTWIFID,crit:reject"`
	GlobalWAGFID      *GlobalWAGFID      `per:",choice:3,extie:IDGlobalWAGFID,crit:reject"`
	ChoiceExtension   *SingleContainer   `per:",choice:3,extie"`
	choiceCriticality ap.WireCriticality `per:",choicecrit"`
}

type GlobalGNBID struct {
	PLMNIdentity PLMNIdentity
	GNBID        GNBID
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type GNBID struct {
	GNBID           *GNBIDValue      `per:",choice:0"`
	ChoiceExtension *SingleContainer `per:",choice:1,extie"`
}

type GNBIDValue struct {
	Value uint32
	Bits  int
}

type GlobalN3IWFID struct {
	PLMNIdentity PLMNIdentity
	N3IWFID      N3IWFID
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type N3IWFID struct {
	N3IWFID         *N3IWFIDValue    `per:",choice:0"`
	ChoiceExtension *SingleContainer `per:",choice:1,extie"`
}

type N3IWFIDValue uint16

type GlobalTNGFID struct {
	PLMNIdentity PLMNIdentity
	TNGFID       TNGFID
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type TNGFID struct {
	TNGFID          *TNGFIDValue     `per:",choice:0"`
	ChoiceExtension *SingleContainer `per:",choice:1,extie"`
}

type TNGFIDValue struct{ BitString }

type GlobalTWIFID struct {
	PLMNIdentity PLMNIdentity
	TWIFID       TWIFID
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type TWIFID struct {
	TWIFID          *TWIFIDValue     `per:",choice:0"`
	ChoiceExtension *SingleContainer `per:",choice:1,extie"`
}

type TWIFIDValue struct{ BitString }

type GlobalWAGFID struct {
	PLMNIdentity PLMNIdentity
	WAGFID       WAGFID
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type WAGFID struct {
	WAGFID          *WAGFIDValue     `per:",choice:0"`
	ChoiceExtension *SingleContainer `per:",choice:1,extie"`
}

type WAGFIDValue struct{ BitString }

// NgENB-ID ::= CHOICE { macroNgENB-ID, shortMacroNgENB-ID, longMacroNgENB-ID,
// choice-Extensions } — TS 38.413 §9.3.1.6.
type NgENBID struct {
	MacroNgENBID      *MacroNgENBID      `per:",choice:0"`
	ShortMacroNgENBID *ShortMacroNgENBID `per:",choice:1"`
	LongMacroNgENBID  *LongMacroNgENBID  `per:",choice:2"`
	ChoiceExtension   *SingleContainer   `per:",choice:3,extie"`
}

type MacroNgENBID uint32

type ShortMacroNgENBID uint32

type LongMacroNgENBID uint32

const (
	gnbIDMinBits          = 22
	gnbIDMaxBits          = 32
	macroNgENBIDBits      = 20
	shortMacroNgENBIDBits = 18
	longMacroNgENBIDBits  = 21
	n3IWFIDBits           = 16
	tngfIDBits            = 32
	twifIDBits            = 32
	wagfIDBits            = 16
)

func NewGlobalGNBID(plmn PLMNIdentity, id uint32, bits int) GlobalRANNodeID {
	return GlobalRANNodeID{GlobalGNBID: &GlobalGNBID{
		PLMNIdentity: plmn,
		GNBID:        GNBID{GNBID: &GNBIDValue{Value: id, Bits: bits}},
	}}
}

func NewGlobalNgENBID(plmn PLMNIdentity, id NgENBID) GlobalRANNodeID {
	return GlobalRANNodeID{GlobalNgENBID: &GlobalNgENBID{PLMNIdentity: plmn, NgENBID: id}}
}

func NewGlobalN3IWFID(plmn PLMNIdentity, id uint16) GlobalRANNodeID {
	v := N3IWFIDValue(id)

	return GlobalRANNodeID{GlobalN3IWFID: &GlobalN3IWFID{PLMNIdentity: plmn, N3IWFID: N3IWFID{N3IWFID: &v}}}
}

func NgENBIDOfBits(id uint32, bits int) (NgENBID, bool) {
	switch bits {
	case macroNgENBIDBits:
		v := MacroNgENBID(id)

		return NgENBID{MacroNgENBID: &v}, true
	case shortMacroNgENBIDBits:
		v := ShortMacroNgENBID(id)

		return NgENBID{ShortMacroNgENBID: &v}, true
	case longMacroNgENBIDBits:
		v := LongMacroNgENBID(id)

		return NgENBID{LongMacroNgENBID: &v}, true
	default:
		return NgENBID{}, false
	}
}

func (n NgENBID) ID() (value uint32, bits int, ok bool) {
	switch {
	case n.MacroNgENBID != nil:
		return uint32(*n.MacroNgENBID), macroNgENBIDBits, true
	case n.ShortMacroNgENBID != nil:
		return uint32(*n.ShortMacroNgENBID), shortMacroNgENBIDBits, true
	case n.LongMacroNgENBID != nil:
		return uint32(*n.LongMacroNgENBID), longMacroNgENBIDBits, true
	default:
		return 0, 0, false
	}
}

func (g GlobalRANNodeID) PLMNIdentity() (PLMNIdentity, bool) {
	switch {
	case g.GlobalGNBID != nil:
		return g.GlobalGNBID.PLMNIdentity, true
	case g.GlobalNgENBID != nil:
		return g.GlobalNgENBID.PLMNIdentity, true
	case g.GlobalN3IWFID != nil:
		return g.GlobalN3IWFID.PLMNIdentity, true
	case g.GlobalTNGFID != nil:
		return g.GlobalTNGFID.PLMNIdentity, true
	case g.GlobalTWIFID != nil:
		return g.GlobalTWIFID.PLMNIdentity, true
	case g.GlobalWAGFID != nil:
		return g.GlobalWAGFID.PLMNIdentity, true
	default:
		return PLMNIdentity{}, false
	}
}

func (g GlobalRANNodeID) NodeID() (value uint64, bits int, ok bool) {
	switch {
	case g.GlobalGNBID != nil:
		if v := g.GlobalGNBID.GNBID.GNBID; v != nil {
			return uint64(v.Value), v.Bits, true
		}
	case g.GlobalNgENBID != nil:
		v, n, ok := g.GlobalNgENBID.NgENBID.ID()

		return uint64(v), n, ok
	case g.GlobalN3IWFID != nil:
		if v := g.GlobalN3IWFID.N3IWFID.N3IWFID; v != nil {
			return uint64(*v), n3IWFIDBits, true
		}
	case g.GlobalTNGFID != nil:
		if v := g.GlobalTNGFID.TNGFID.TNGFID; v != nil && v.Bits() <= 64 {
			return v.Uint64(), v.Bits(), true
		}
	case g.GlobalTWIFID != nil:
		if v := g.GlobalTWIFID.TWIFID.TWIFID; v != nil && v.Bits() <= 64 {
			return v.Uint64(), v.Bits(), true
		}
	case g.GlobalWAGFID != nil:
		if v := g.GlobalWAGFID.WAGFID.WAGFID; v != nil && v.Bits() <= 64 {
			return v.Uint64(), v.Bits(), true
		}
	}

	return 0, 0, false
}

// GlobalNgENB-ID ::= SEQUENCE { pLMNIdentity, ngENB-ID, iE-Extensions OPTIONAL }
// (extensible) — TS 38.413 §9.3.1.8.
type GlobalNgENBID struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity PLMNIdentity
	NgENBID      NgENBID
	iEExtensions extensionMeta `per:",exttable"`
}

// EPS-TAC ::= OCTET STRING (SIZE(2)) — TS 38.413 §9.3.3.32, held as the 16-bit
// number those octets carry. It is the EPS tracking area code, two octets where
// the 5GS TAC is three, which is the whole reason EPS-TAI exists beside TAI.
type EPSTAC uint16

// EPS-TAI ::= SEQUENCE { pLMNIdentity, ePS-TAC, iE-Extensions OPTIONAL }
// (extensible) — TS 38.413 §9.3.3.31.
type EPSTAI struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity PLMNIdentity
	TAC          EPSTAC
	iEExtensions extensionMeta `per:",exttable"`
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
	value, bits, _ := g.NodeID()
	if bits <= 0 || bits >= width {
		return 0, fmt.Errorf("ngap: node id of %d bits leaves no room in a %d-bit %s cell identity", bits, width, kind)
	}

	cellBits := width - bits
	if cellID >= 1<<uint(cellBits) {
		return 0, fmt.Errorf("ngap: cell id %d exceeds the %d bits left by a %d-bit node id", cellID, cellBits, bits)
	}

	if bits < 64 && value >= 1<<uint(bits) {
		return 0, fmt.Errorf("ngap: node id %d exceeds its own %d bits", value, bits)
	}

	return value<<uint(cellBits) | cellID, nil
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
	value, bits, _ := g.NodeID()
	if bits <= 0 || bits >= width {
		return 0, false
	}

	cellBits := width - bits
	if id>>uint(cellBits) != value {
		return 0, false
	}

	return id & (1<<uint(cellBits) - 1), true
}

func (g GlobalRANNodeID) Hex() string {
	value, bits, _ := g.NodeID()
	if bits <= 0 {
		return ""
	}

	return fmt.Sprintf("%0*x", (bits+3)/4, value)
}
