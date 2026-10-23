// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"cmp"
	"slices"

	"github.com/ellanetworks/core/internal/models"
)

type BearerView struct {
	ID      uint8
	Default bool
	QCI     uint8
	ARP     models.Arp
	MBR     models.Ambr
	GBR     models.Ambr
	Rules   []string
}

func ruleIDs(rules []PCCRule) []string {
	ids := make([]string, 0, len(rules))
	for _, r := range rules {
		ids = append(ids, r.ID)
	}

	slices.Sort(ids)

	return ids
}

func (sc *SMContext) Bearers() []BearerView {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	binding, ok := sc.defaultBinding()
	if !ok {
		return nil
	}

	def := BearerView{ID: sc.EBI, Default: true, QCI: binding.QCI, ARP: binding.ARP, Rules: []string{}}
	if sc.Access == Access5G {
		def.ID = sc.PolicyData.QosData.QFI
	}

	if sc.onDefault != nil {
		def.Rules = ruleIDs(sc.onDefault.installed.rules)
	}

	var dedicated []BearerView

	for _, b := range sc.dedicated {
		if b.state != dedicatedActive || !b.enforces() {
			continue
		}

		v := BearerView{ID: b.ebi, QCI: b.installed.binding.QCI, ARP: fullARP(b.installed.binding.ARP), MBR: b.installed.mbr, GBR: b.installed.gbr, Rules: ruleIDs(b.installed.rules)}
		if sc.Access == Access5G {
			v.ID = b.qfi
		}

		dedicated = append(dedicated, v)
	}

	slices.SortFunc(dedicated, func(a, b BearerView) int { return cmp.Compare(a.ID, b.ID) })

	return append([]BearerView{def}, dedicated...)
}
