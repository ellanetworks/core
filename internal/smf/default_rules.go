// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"slices"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

func fullARP(a models.Arp) models.Arp {
	if a.PreemptCap != models.PreemptionCapabilityMayPreempt {
		a.PreemptCap = models.PreemptionCapabilityNotPreempt
	}

	if a.PreemptVuln != models.PreemptionVulnerabilityPreemptable {
		a.PreemptVuln = models.PreemptionVulnerabilityNotPreemptable
	}

	return a
}

func (sc *SMContext) defaultBinding() (bearerBinding, bool) {
	if sc.PolicyData == nil || sc.PolicyData.QosData.Arp == nil {
		return bearerBinding{}, false
	}

	return bearerBinding{QCI: uint8(sc.PolicyData.QosData.Var5qi), ARP: fullARP(*sc.PolicyData.QosData.Arp)}, true
}

func (sc *SMContext) splitDefaultRules(rules []PCCRule) (onDefault, dedicated []PCCRule) {
	binding, ok := sc.defaultBinding()

	for _, r := range rules {
		if ok && r.QCI == binding.QCI && fullARP(r.ARP) == binding.ARP {
			onDefault = append(onDefault, r)
		} else {
			dedicated = append(dedicated, r)
		}
	}

	return onDefault, dedicated
}

func (s *SMF) bindDefaultLocked(ctx context.Context, sc *SMContext, rules []PCCRule) []PCCRule {
	if sc.onDefault == nil {
		if len(rules) == 0 {
			return nil
		}

		sc.onDefault = &dedicatedBearer{slot: defaultRuleSlot}
	}

	b := sc.onDefault
	binding, _ := sc.defaultBinding()

	var (
		tft  bearerTFT
		legs []ruleLeg
	)

	if len(rules) > 0 {
		var err error

		tft, err = assignFilters(defaultRuleSlot, b.installed.tft, rules)
		if err != nil {
			logger.From(ctx, logger.SmfLog).Warn("default bearer cannot carry the rules", logger.SUPI(sc.Supi.String()), zap.Error(err))
			return rules
		}

		delete(tft, blockingUplinkKey)

		legs = sc.legRulesLocked(b, ruleSet{tft, rules})
	} else {
		b.ruleIndex = nil
	}

	if !slices.EqualFunc(sc.Tunnel.DefaultRules, legs, sameRuleLeg) {
		next := sc.Tunnel.dataPlane
		next.DefaultRules = legs

		err := next.checkSDFCapacity()
		if err == nil {
			err = s.applyDataPlane(ctx, sc, next, sc.policyID())
		}

		if err != nil {
			logger.From(ctx, logger.SmfLog).Warn("rules not bound to the default bearer", logger.SUPI(sc.Supi.String()), zap.Error(err))
			return addedOrChangedRules(rules, b.installed.rules)
		}
	}

	b.installed = bearerConfig{binding: binding, rules: rules, tft: tft}

	return nil
}
