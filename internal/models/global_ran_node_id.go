// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var nidPattern = regexp.MustCompile(`^[A-Fa-f0-9]{11}$`)

const (
	RanNodeTypeUnknown = "Unknown"
	RanNodeTypeGNB     = "gNB"
	RanNodeTypeNgENB   = "ng-eNB"
	RanNodeTypeENB     = "eNB"
	RanNodeTypeN3IWF   = "N3IWF"
	RanNodeTypeWAGF    = "W-AGF"
	RanNodeTypeTNGF    = "TNGF"
)

type GlobalRanNodeID struct {
	PlmnID  *PlmnID
	Nid     string
	N3IwfID string
	GNbID   *GNbID
	NgeNbID string
	WAgfID  string
	TngfID  string
	ENbID   string
}

type ranNodeAlternative struct {
	nodeType string
	id       string
}

func (g GlobalRanNodeID) alternative() (ranNodeAlternative, bool) {
	switch {
	case g.GNbID != nil:
		return ranNodeAlternative{RanNodeTypeGNB, g.GNbID.GNBValue}, true
	case g.NgeNbID != "":
		return ranNodeAlternative{RanNodeTypeNgENB, g.NgeNbID}, true
	case g.ENbID != "":
		return ranNodeAlternative{RanNodeTypeENB, g.ENbID}, true
	case g.N3IwfID != "":
		return ranNodeAlternative{RanNodeTypeN3IWF, g.N3IwfID}, true
	case g.WAgfID != "":
		return ranNodeAlternative{RanNodeTypeWAGF, g.WAgfID}, true
	case g.TngfID != "":
		return ranNodeAlternative{RanNodeTypeTNGF, g.TngfID}, true
	}

	return ranNodeAlternative{}, false
}

func (g GlobalRanNodeID) RanNodeType() string {
	alt, ok := g.alternative()
	if !ok {
		return RanNodeTypeUnknown
	}

	return alt.nodeType
}

func (g GlobalRanNodeID) NodeID() string {
	alt, ok := g.alternative()
	if !ok {
		return ""
	}

	return alt.id
}

func RanNodeIDKey(id *GlobalRanNodeID) (string, bool) {
	if id == nil {
		return "", false
	}

	return id.Ref()
}

func (g GlobalRanNodeID) Ref() (string, bool) {
	alt, ok := g.alternative()
	if !ok {
		return "", false
	}

	plmn := "-"
	if g.PlmnID != nil {
		plmn = g.PlmnID.Mcc + "-" + g.PlmnID.Mnc
	}

	id := alt.id
	if g.GNbID != nil {
		id = fmt.Sprintf("%s@%d", alt.id, g.GNbID.BitLength)
	}

	if g.Nid != "" {
		return alt.nodeType + ":" + plmn + ":" + g.Nid + ":" + id, true
	}

	return alt.nodeType + ":" + plmn + ":" + id, true
}

func (g GlobalRanNodeID) String() string {
	ref, ok := g.Ref()
	if !ok {
		return RanNodeTypeUnknown
	}

	return ref
}

func parsePlmnID(s string) (PlmnID, error) {
	mcc, mnc, ok := strings.Cut(s, "-")
	if !ok {
		return PlmnID{}, fmt.Errorf("PLMN %q is not <mcc>-<mnc>", s)
	}

	if len(mcc) != 3 || !allDigits(mcc) {
		return PlmnID{}, fmt.Errorf("MCC %q is not three digits", mcc)
	}

	if len(mnc) < 2 || len(mnc) > 3 || !allDigits(mnc) {
		return PlmnID{}, fmt.Errorf("MNC %q is not two or three digits", mnc)
	}

	return PlmnID{Mcc: mcc, Mnc: mnc}, nil
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return s != ""
}

func ParseRanNodeRef(ref string) (GlobalRanNodeID, error) {
	var nodeType, plmn, nid, id string

	parts := strings.Split(ref, ":")

	switch len(parts) {
	case 3:
		nodeType, plmn, id = parts[0], parts[1], parts[2]
	case 4:
		nodeType, plmn, nid, id = parts[0], parts[1], parts[2], parts[3]
	default:
		return GlobalRanNodeID{}, fmt.Errorf("radio %q is not <type>:<plmn>:<id>", ref)
	}

	if nid != "" && !nidPattern.MatchString(nid) {
		return GlobalRanNodeID{}, fmt.Errorf("NID %q is not 11 hexadecimal digits", nid)
	}

	out := GlobalRanNodeID{Nid: nid}

	if plmn != "-" {
		p, err := parsePlmnID(plmn)
		if err != nil {
			return GlobalRanNodeID{}, err
		}

		out.PlmnID = &p
	}

	value, bits, hasBits := strings.Cut(id, "@")
	if value == "" {
		return GlobalRanNodeID{}, fmt.Errorf("node ID is required")
	}

	if nodeType == RanNodeTypeGNB {
		if !hasBits {
			return GlobalRanNodeID{}, fmt.Errorf("a gNB is identified by its gNB ID and bit length, as <id>@<bits>")
		}

		bitLength, err := strconv.ParseInt(bits, 10, 32)
		if err != nil {
			return GlobalRanNodeID{}, fmt.Errorf("gNB ID bit length %q is not a number", bits)
		}

		if bitLength < 22 || bitLength > 32 {
			return GlobalRanNodeID{}, fmt.Errorf("no gNB ID is %d bits wide", bitLength)
		}

		out.GNbID = &GNbID{BitLength: int32(bitLength), GNBValue: value}

		return out, nil
	}

	if hasBits {
		return GlobalRanNodeID{}, fmt.Errorf("only a gNB ID carries a bit length")
	}

	switch nodeType {
	case RanNodeTypeNgENB:
		out.NgeNbID = value
	case RanNodeTypeENB:
		out.ENbID = value
	case RanNodeTypeN3IWF:
		out.N3IwfID = value
	case RanNodeTypeWAGF:
		out.WAgfID = value
	case RanNodeTypeTNGF:
		out.TngfID = value
	default:
		return GlobalRanNodeID{}, fmt.Errorf("unknown RAN node type %q", nodeType)
	}

	return out, nil
}
