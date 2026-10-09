// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"go.uber.org/zap"
)

type filterKey struct {
	rule  string
	index int
}

type bearerTFT map[filterKey]models.SDFFilter

type pendingModification struct {
	binding bearerBinding
	rules   []PCCRule
	tft     bearerTFT
	mbr     models.Ambr
	gbr     models.Ambr
}

type rejectedStep struct {
	step  bearerTFT
	until time.Time
}

func (t bearerTFT) carries(r PCCRule) bool {
	for i, f := range r.Filters {
		got, ok := t[filterKey{rule: r.ID, index: i}]
		if !ok {
			return false
		}

		f.ID, f.Precedence = got.ID, got.Precedence
		if got != f {
			return false
		}
	}

	return true
}

func installedAfter(installed, wanted []PCCRule, tft bearerTFT) []PCCRule {
	var out []PCCRule

	for _, r := range wanted {
		if tft.carries(r) {
			out = append(out, r)
		}
	}

	for _, r := range installed {
		if !slices.ContainsFunc(out, func(o PCCRule) bool { return o.ID == r.ID }) && tft.carries(r) {
			out = append(out, r)
		}
	}

	return out
}

func filterPrecedence(slot, id uint8) uint8 {
	return dedicatedFilterPrecedence + slot*maxDedicatedFilters + id - 1
}

func assignFilters(slot uint8, current bearerTFT, rules []PCCRule) (bearerTFT, error) {
	next := make(bearerTFT)
	taken := make(map[uint8]bool, len(current))

	for _, f := range current {
		taken[f.ID] = true
	}

	var fresh []filterKey

	for _, r := range rules {
		for i, f := range r.Filters {
			k := filterKey{rule: r.ID, index: i}

			if old, ok := current[k]; ok {
				f.ID = old.ID
				f.Precedence = old.Precedence
				next[k] = f

				continue
			}

			next[k] = f
			fresh = append(fresh, k)
		}
	}

	if len(next) == 0 || len(next) > maxDedicatedFilters {
		return nil, fmt.Errorf("a dedicated bearer carries 1 to %d packet filters, the rules have %d", maxDedicatedFilters, len(next))
	}

	for _, k := range fresh {
		id := uint8(1)
		for ; id <= maxDedicatedFilters && taken[id]; id++ {
		}

		if id > maxDedicatedFilters {
			return nil, fmt.Errorf("no free packet filter identifier on the bearer")
		}

		taken[id] = true

		f := next[k]
		f.ID, f.Precedence = id, filterPrecedence(slot, id)
		next[k] = f
	}

	return next, nil
}

func (t bearerTFT) list() []models.SDFFilter {
	out := slices.Collect(maps.Values(t))
	slices.SortFunc(out, func(a, b models.SDFFilter) int { return cmp.Compare(a.ID, b.ID) })

	return out
}

type ruleSet struct {
	tft   bearerTFT
	rules []PCCRule
}

func (sc *SMContext) legRulesLocked(b *dedicatedBearer, sets ...ruleSet) []ruleLeg {
	fallback := slices.Clone(b.rules)
	if sc.policyDecision != nil {
		fallback = append(fallback, sc.policyDecision.Rules...)
	}

	return b.legRules(fallback, sets...)
}

func (b *dedicatedBearer) legRules(fallback []PCCRule, sets ...ruleSet) []ruleLeg {
	legs := make(map[string]*ruleLeg)

	for _, set := range sets {
		filters := make(map[string][]models.SDFFilter)
		for k, f := range set.tft {
			filters[k.rule] = append(filters[k.rule], f)
		}

		for id, fs := range filters {
			rule := PCCRule{Gate: models.GateStatus{ULGate: models.GateClose, DLGate: models.GateClose}}
			if i := slices.IndexFunc(set.rules, func(r PCCRule) bool { return r.ID == id }); i >= 0 {
				rule = set.rules[i]
			} else if i := slices.IndexFunc(fallback, func(r PCCRule) bool { return r.ID == id }); i >= 0 {
				rule = fallback[i]
			}

			l, ok := legs[id]
			if !ok {
				legs[id] = &ruleLeg{Filters: fs, MBR: rule.MBR, Gate: rule.Gate}
				continue
			}

			for _, f := range fs {
				if !slices.Contains(l.Filters, f) {
					l.Filters = append(l.Filters, f)
				}
			}

			l.MBR = maxAmbr(l.MBR, rule.MBR)
			l.Gate = openEither(l.Gate, rule.Gate)
		}
	}

	for id := range b.ruleIndex {
		if _, ok := legs[id]; !ok {
			delete(b.ruleIndex, id)
		}
	}

	if b.ruleIndex == nil {
		b.ruleIndex = make(map[string]uint8)
	}

	out := make([]ruleLeg, 0, len(legs))

	for _, id := range slices.Sorted(maps.Keys(legs)) {
		index, ok := b.ruleIndex[id]
		if !ok {
			index = b.freeRuleIndex()
			b.ruleIndex[id] = index
		}

		l := legs[id]
		l.Index = index
		slices.SortFunc(l.Filters, func(x, y models.SDFFilter) int { return cmp.Compare(x.ID, y.ID) })
		out = append(out, *l)
	}

	slices.SortFunc(out, func(x, y ruleLeg) int { return cmp.Compare(x.Index, y.Index) })

	return out
}

func (b *dedicatedBearer) freeRuleIndex() uint8 {
	var index uint8
	for slices.Contains(slices.Collect(maps.Values(b.ruleIndex)), index) {
		index++
	}

	return index
}

func openEither(a, b models.GateStatus) models.GateStatus {
	return models.GateStatus{ULGate: min(a.ULGate, b.ULGate), DLGate: min(a.DLGate, b.DLGate)}
}

func sameRuleLeg(a, b ruleLeg) bool {
	return a.Index == b.Index && a.Gate == b.Gate && ambrEqual(a.MBR, b.MBR) && slices.Equal(a.Filters, b.Filters)
}

func maxAmbr(a, b models.Ambr) models.Ambr {
	return models.Ambr{
		Uplink:   models.BitRateFromBps(max(a.Uplink.Bps(), b.Uplink.Bps())),
		Downlink: models.BitRateFromBps(max(a.Downlink.Bps(), b.Downlink.Bps())),
	}
}

func ambrEqual(a, b models.Ambr) bool {
	return a.Uplink.Equal(b.Uplink) && a.Downlink.Equal(b.Downlink)
}

func nextTFTStep(current, next bearerTFT) (models.TFTOperation, bearerTFT, []models.SDFFilter, []uint8) {
	var replaced, added []models.SDFFilter

	var deleted []uint8

	for k, f := range next {
		old, ok := current[k]

		switch {
		case !ok:
			added = append(added, f)
		case old != f:
			replaced = append(replaced, f)
		}
	}

	for k, f := range current {
		if _, ok := next[k]; !ok {
			deleted = append(deleted, f.ID)
		}
	}

	step := maps.Clone(current)

	switch {
	case len(replaced) > 0:
		for k, f := range next {
			if old, ok := current[k]; ok && old != f {
				step[k] = f
			}
		}

		return models.TFTReplaceFilters, step, sortedFilters(replaced), nil
	case len(added) > 0:
		for k, f := range next {
			if _, ok := current[k]; !ok {
				step[k] = f
			}
		}

		return models.TFTAddFilters, step, sortedFilters(added), nil
	case len(deleted) > 0:
		for k := range current {
			if _, ok := next[k]; !ok {
				delete(step, k)
			}
		}

		slices.Sort(deleted)

		return models.TFTDeleteFilters, step, nil, deleted
	default:
		return models.TFTNoChange, step, nil, nil
	}
}

func sortedFilters(filters []models.SDFFilter) []models.SDFFilter {
	slices.SortFunc(filters, func(a, b models.SDFFilter) int { return cmp.Compare(a.ID, b.ID) })
	return filters
}

func addedOrChangedRules(next, previous []PCCRule) []PCCRule {
	var out []PCCRule

	for _, r := range next {
		if !slices.ContainsFunc(previous, func(p PCCRule) bool { return samePCCRule(p, r) }) {
			out = append(out, r)
		}
	}

	return out
}

func (s *SMF) planModificationLocked(ctx context.Context, sc *SMContext, b *dedicatedBearer, binding bearerBinding, rules []PCCRule) (*dedicatedAction, []PCCRule) {
	if !b.modifyWaitingSince.IsZero() && time.Since(b.modifyWaitingSince) > s.dedicatedAwaitLimit {
		b.modifyWaitingSince = time.Time{}
		return nil, addedOrChangedRules(rules, b.rules)
	}

	next, err := assignFilters(b.slot, b.tft, rules)
	if err != nil {
		logger.From(ctx, logger.SmfLog).Warn("dedicated bearer cannot carry the rules", logger.SUPI(sc.Supi.String()), zap.Uint8("ebi", b.ebi), zap.Error(err))
		return nil, addedOrChangedRules(rules, b.rules)
	}

	op, step, filters, deleted := nextTFTStep(b.tft, next)
	if time.Now().Before(b.rejected.until) && maps.Equal(step, b.rejected.step) {
		return nil, nil
	}

	after := installedAfter(b.rules, rules, step)
	mbr, gbr := bearerRates(after)
	qos := !ambrEqual(mbr, b.mbr) || !ambrEqual(gbr, b.gbr)
	arpChanged := binding != b.binding

	if op == models.TFTNoChange && !qos && !arpChanged {
		if err := s.setLegRulesLocked(ctx, sc, b, sc.legRulesLocked(b, ruleSet{b.tft, after})); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("dedicated bearer user plane not updated", logger.SUPI(sc.Supi.String()), zap.Uint8("ebi", b.ebi), zap.Error(err))
			return nil, addedOrChangedRules(after, b.rules)
		}

		b.rules = after
		b.modifyWaitingSince = time.Time{}

		return nil, nil
	}

	teid := sc.Tunnel.bearerTEID(b.slot)

	if err := s.setLegRulesLocked(ctx, sc, b, sc.legRulesLocked(b, ruleSet{b.tft, b.rules}, ruleSet{step, after})); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("dedicated bearer user plane not updated", logger.SUPI(sc.Supi.String()), zap.Uint8("ebi", b.ebi), zap.Error(err))
		return nil, addedOrChangedRules(after, b.rules)
	}

	b.modification = &pendingModification{binding: binding, rules: after, tft: step, mbr: mbr, gbr: gbr}

	var mapped []nas.PCOContainer

	if b.fiveGSQoS {
		mapped, err = sc.mappedFiveGSQoSLocked(b, b.rules, b.tft, after, step, qos)
		if err != nil {
			logger.From(ctx, logger.SmfLog).Warn("dedicated bearer modified without its 5GS QoS; it stays in EPS", logger.SUPI(sc.Supi.String()), zap.Uint8("ebi", b.ebi), zap.Error(err))

			b.fiveGSQoS = false
		}
	}

	modify := &models.DedicatedBearerModification{
		SessionRef: sc.Ref, SGWTEID: teid,
		QoSChanged: qos, MBR: mbr, GBR: gbr,
		Operation: op, Filters: filters, DeleteIDs: deleted,
		MappedFiveGSQoS: mapped,
	}

	if arpChanged {
		modify.ARP = new(binding.ARP)
	}

	return &dedicatedAction{
		modify:  modify,
		ebi:     b.ebi,
		sgwTEID: teid,
	}, nil
}

func (s *SMF) restoreLegLocked(ctx context.Context, sc *SMContext, b *dedicatedBearer) {
	if err := s.setLegRulesLocked(ctx, sc, b, sc.legRulesLocked(b, ruleSet{b.tft, b.rules})); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("dedicated bearer user plane not restored", logger.SUPI(sc.Supi.String()), zap.Uint8("ebi", b.ebi), zap.Error(err))
	}
}

func (s *SMF) failModificationLocked(ctx context.Context, sc *SMContext, b *dedicatedBearer) []RuleReport {
	step := b.modification.tft
	failed := addedOrChangedRules(b.modification.rules, b.rules)
	b.modification = nil
	b.pruneRuleIdentities(b.rules)
	s.restoreLegLocked(ctx, sc, b)

	if len(failed) == 0 {
		b.rejected = rejectedStep{step: step, until: time.Now().Add(s.dedicatedAwaitLimit)}
		s.reconcileAfter(sc.Ref, s.dedicatedAwaitLimit)

		return nil
	}

	sc.recordFailedRulesLocked(failed)

	return sc.ruleReportsLocked(failed)
}

func (s *SMF) modificationNotSent(ctx context.Context, sc *SMContext, teid uint32, err error) {
	sc.Mutex.Lock()

	b := sc.dedicatedBySGWTEID(teid)
	if b == nil || b.modification == nil {
		sc.Mutex.Unlock()
		return
	}

	if errors.Is(err, ErrUENotReachable) {
		b.modification = nil
		b.pruneRuleIdentities(b.rules)
		s.restoreLegLocked(ctx, sc, b)

		if b.modifyWaitingSince.IsZero() {
			b.modifyWaitingSince = time.Now()
		}
		sc.Mutex.Unlock()

		s.reconcileAfter(sc.Ref, s.dedicatedAwaitLimit)

		return
	}

	reports := s.failModificationLocked(ctx, sc, b)
	sc.Mutex.Unlock()

	logger.From(ctx, logger.SmfLog).Warn("dedicated bearer modification not sent", logger.SUPI(sc.Supi.String()), zap.Error(err))
	s.reportFailedRules(sc, reports, ResourcesNotAllocated)
}

func (s *SMF) DedicatedBearerModified(ctx context.Context, ref string, sgwTEID uint32, accepted bool) {
	sc := s.GetSession(ref)
	if sc == nil {
		return
	}

	sc.Mutex.Lock()

	b := sc.dedicatedBySGWTEID(sgwTEID)
	if b == nil || b.modification == nil {
		sc.Mutex.Unlock()
		return
	}

	b.modifyWaitingSince = time.Time{}

	var reports []RuleReport

	if accepted {
		m := b.modification
		b.modification = nil
		b.binding, b.tft, b.mbr, b.gbr, b.rules = m.binding, m.tft, m.mbr, m.gbr, m.rules
		b.pruneRuleIdentities(b.rules)
		b.rejected = rejectedStep{}
		s.restoreLegLocked(ctx, sc, b)
	} else {
		reports = s.failModificationLocked(ctx, sc, b)
	}
	sc.Mutex.Unlock()

	logger.From(ctx, logger.SmfLog).Info("dedicated bearer modification concluded", logger.SUPI(sc.Supi.String()), zap.Uint8("ebi", b.ebi), zap.Bool("accepted", accepted))

	s.reportFailedRules(sc, reports, ResourcesNotAllocated)
	s.reconcileAfter(ref, 0)
}
