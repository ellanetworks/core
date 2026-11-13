// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type BroadcastPLMN struct {
	Mcc    string
	Mnc    string
	Slices []SliceOpt
}

func sliceSupportList(slices []SliceOpt) (ngap.SliceSupportList, error) {
	support := make(ngap.SliceSupportList, 0, len(slices))

	for _, s := range slices {
		snssai, err := sliceToNGAP(s)
		if err != nil {
			return nil, err
		}

		support = append(support, ngap.SliceSupportItem{SNSSAI: snssai})
	}

	return support, nil
}

func BuildBroadcastPLMNList(primary ngap.PLMNIdentity, support ngap.SliceSupportList, additional []BroadcastPLMN) (ngap.BroadcastPLMNList, error) {
	list := ngap.BroadcastPLMNList{{PLMNIdentity: primary, TAISliceSupportList: support}}
	seen := map[ngap.PLMNIdentity]bool{primary: true}

	for _, p := range additional {
		plmn, err := PLMNIdentity(p.Mcc, p.Mnc)
		if err != nil {
			return nil, err
		}

		if seen[plmn] {
			return nil, fmt.Errorf("PLMN %s/%s is broadcast twice", p.Mcc, p.Mnc)
		}

		seen[plmn] = true

		item := ngap.BroadcastPLMNItem{PLMNIdentity: plmn, TAISliceSupportList: support}

		if len(p.Slices) > 0 {
			item.TAISliceSupportList, err = sliceSupportList(p.Slices)
			if err != nil {
				return nil, err
			}
		}

		list = append(list, item)
	}

	return list, nil
}

func selectedOrServing(mcc, mnc, selMcc, selMnc string) (string, string) {
	if selMcc == "" || selMnc == "" {
		return mcc, mnc
	}

	return selMcc, selMnc
}

func (g *GnodeB) SelectPLMN(ranUENGAPID int64, mcc, mnc string) error {
	plmn, err := PLMNIdentity(mcc, mnc)
	if err != nil {
		return err
	}

	broadcast := mcc == g.MCC && mnc == g.MNC

	for _, p := range g.AdditionalPLMNs {
		if other, err := PLMNIdentity(p.Mcc, p.Mnc); err == nil && other == plmn {
			broadcast = true
		}
	}

	if !broadcast {
		return fmt.Errorf("PLMN %s/%s is not broadcast by this node", mcc, mnc)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.selectedPLMNs == nil {
		g.selectedPLMNs = make(map[int64]BroadcastPLMN)
	}

	g.selectedPLMNs[ranUENGAPID] = BroadcastPLMN{Mcc: mcc, Mnc: mnc}

	return nil
}

func (g *GnodeB) SelectedPLMN(ranUENGAPID int64) (mcc, mnc string) {
	if mcc, mnc = g.explicitPLMN(ranUENGAPID); mcc != "" {
		return mcc, mnc
	}

	return g.MCC, g.MNC
}

func (g *GnodeB) explicitPLMN(ranUENGAPID int64) (mcc, mnc string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	p := g.selectedPLMNs[ranUENGAPID]

	return p.Mcc, p.Mnc
}
