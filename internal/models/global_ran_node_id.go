// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import "fmt"

const RanNodeTypeUnknown = "Unknown"

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
	tag      string
	nodeType string
	id       string
}

func (g GlobalRanNodeID) alternative() (ranNodeAlternative, bool) {
	switch {
	case g.GNbID != nil:
		return ranNodeAlternative{"gnb", "gNB", g.GNbID.GNBValue}, true
	case g.NgeNbID != "":
		return ranNodeAlternative{"ngenb", "ng-eNB", g.NgeNbID}, true
	case g.ENbID != "":
		return ranNodeAlternative{"enb", "eNB", g.ENbID}, true
	case g.N3IwfID != "":
		return ranNodeAlternative{"n3iwf", "N3IWF", g.N3IwfID}, true
	case g.WAgfID != "":
		return ranNodeAlternative{"wagf", "W-AGF", g.WAgfID}, true
	case g.TngfID != "":
		return ranNodeAlternative{"tngf", "TNGF", g.TngfID}, true
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

	return id.Key()
}

func (g GlobalRanNodeID) String() string {
	alt, ok := g.alternative()
	if !ok {
		return RanNodeTypeUnknown
	}

	plmn := "-"
	if g.PlmnID != nil {
		plmn = g.PlmnID.Mcc + "-" + g.PlmnID.Mnc
	}

	if g.Nid != "" {
		plmn += "/" + g.Nid
	}

	id := alt.id
	if g.GNbID != nil {
		id = fmt.Sprintf("%s@%d", alt.id, g.GNbID.BitLength)
	}

	return alt.nodeType + "/" + plmn + "/" + id
}

func (g GlobalRanNodeID) Key() (string, bool) {
	alt, ok := g.alternative()
	if !ok {
		return "", false
	}

	node := alt.tag + ":" + alt.id
	if g.GNbID != nil {
		node = fmt.Sprintf("%s:%d:%s", alt.tag, g.GNbID.BitLength, alt.id)
	}

	plmn := ""
	if g.PlmnID != nil {
		plmn = g.PlmnID.Mcc + "-" + g.PlmnID.Mnc
	}

	return plmn + "/" + g.Nid + "/" + node, true
}
