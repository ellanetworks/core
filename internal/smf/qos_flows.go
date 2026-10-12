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
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	smfNgap "github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

const (
	defaultMaxPacketFilters      uint16 = 16
	minSignalledMaxPacketFilters uint16 = 17
	firstFlowQFI                 uint8  = 2
	lastFlowQFI                  uint8  = 63
	firstFlowQRI                 uint8  = 2
	lastFlowQRI                  uint8  = 255
	firstRulePrecedence          uint8  = 1
	lastRulePrecedence           uint8  = 69
)

type flowParts struct {
	n1 bool
	n2 bool
}

type legOutcome uint8

const (
	legPending legOutcome = iota
	legApplied
	legRefused
)

type flowProcedure struct {
	parts map[*dedicatedBearer]flowParts

	ue, ran     legOutcome
	ranFailed   []uint8
	epsFallback bool

	n1Msg, n2Msg []byte
}

func newFlowProcedure(parts map[*dedicatedBearer]flowParts, n1, n2 []byte) *flowProcedure {
	p := &flowProcedure{parts: parts, n1Msg: n1, n2Msg: n2}

	if n1 == nil {
		p.ue = legApplied
	}

	if n2 == nil {
		p.ran = legApplied
	}

	return p
}

func (p *flowProcedure) answered() bool {
	return p.ue != legPending && p.ran != legPending
}

type flowChange struct {
	rules     fgs.QoSRules
	flows     fgs.QoSFlowDescriptions
	mapped    fgs.MappedEPSBearerContexts
	n2Add     []smfNgap.GBRQoSFlow
	n2Release []uint8
	parts     map[*dedicatedBearer]flowParts
}

func (c *flowChange) touch(b *dedicatedBearer, n1, n2 bool) {
	if c.parts == nil {
		c.parts = make(map[*dedicatedBearer]flowParts)
	}

	p := c.parts[b]
	p.n1 = p.n1 || n1
	p.n2 = p.n2 || n2
	c.parts[b] = p
}

func (sc *SMContext) flowsReady() bool {
	return sc.Access == Access5G && sc.Tunnel != nil && sc.PolicyData != nil &&
		sc.PFCPContext != nil && sc.PFCPContext.Established &&
		!sc.releasing && sc.pending == nil && !sc.activating &&
		sc.flowProcedure == nil && !sc.procedureTimer.Active() &&
		time.Now().After(sc.fallbackUntil)
}

func (sc *SMContext) freeQFI() (uint8, bool) {
	for qfi := firstFlowQFI; qfi <= lastFlowQFI; qfi++ {
		if qfi == sc.Tunnel.QFI || slices.ContainsFunc(sc.dedicated, func(b *dedicatedBearer) bool { return b.qfi == qfi }) {
			continue
		}

		return qfi, true
	}

	return 0, false
}

type ruleIdentity struct {
	qri, precedence uint8
}

func (b *dedicatedBearer) ueQRIs() []uint8 {
	if b.ue == nil {
		return nil
	}

	qris := make([]uint8, 0, len(b.ue.rules))
	for _, r := range b.ue.rules {
		qris = append(qris, b.ruleIdentity[r.ID].qri)
	}

	return qris
}

func (sc *SMContext) allocateRuleIdentity(b *dedicatedBearer, id string) bool {
	if _, ok := b.ruleIdentity[id]; ok {
		return true
	}

	var qris, precedences []uint8

	for _, o := range sc.dedicated {
		for _, r := range o.ruleIdentity {
			qris = append(qris, r.qri)
			precedences = append(precedences, r.precedence)
		}
	}

	qri, ok := firstFree(firstFlowQRI, lastFlowQRI, qris)
	if !ok {
		return false
	}

	precedence, ok := firstFree(firstRulePrecedence, lastRulePrecedence, precedences)
	if !ok {
		return false
	}

	if b.ruleIdentity == nil {
		b.ruleIdentity = make(map[string]ruleIdentity)
	}

	b.ruleIdentity[id] = ruleIdentity{qri: qri, precedence: precedence}

	return true
}

func firstFree(first, last uint8, taken []uint8) (uint8, bool) {
	for v := first; ; v++ {
		if !slices.Contains(taken, v) {
			return v, true
		}

		if v == last {
			return 0, false
		}
	}
}

func (b *dedicatedBearer) pruneRuleIdentities() {
	held := func(id string) bool {
		for _, c := range []*bearerConfig{&b.installed, b.pending, b.ue} {
			if c != nil && slices.ContainsFunc(c.rules, func(r PCCRule) bool { return r.ID == id }) {
				return true
			}
		}

		return false
	}

	maps.DeleteFunc(b.ruleIdentity, func(id string, _ ruleIdentity) bool { return !held(id) })
}

func (b *dedicatedBearer) flowTFT(current bearerTFT, rules []PCCRule) (bearerTFT, error) {
	tft, err := assignFilters(b.slot, current, rules)
	if err != nil {
		return nil, err
	}

	for k, f := range tft {
		if k != blockingUplinkKey {
			f.Precedence = b.ruleIdentity[k.rule].precedence
			tft[k] = f
		}
	}

	return tft, nil
}

func (sc *SMContext) packetFiltersWith(b *dedicatedBearer, tft bearerTFT) int {
	n := 1 + tft.signalledFilters()

	for _, o := range sc.dedicated {
		if o == b {
			continue
		}

		switch {
		case o.pending != nil:
			n += o.pending.tft.signalledFilters()
		case o.ue != nil:
			n += o.ue.tft.signalledFilters()
		default:
			n += o.installed.tft.signalledFilters()
		}
	}

	return n
}

func (sc *SMContext) filterLimit() int {
	if sc.maxPacketFilters == 0 {
		return int(defaultMaxPacketFilters)
	}

	return int(sc.maxPacketFilters)
}

func ruleFilters(tft bearerTFT, id string) []fgs.PacketFilter {
	var out []fgs.PacketFilter

	for k, f := range tft {
		if k.rule != id {
			continue
		}

		var comps []fgs.PacketFilterComponent

		if c, err := fgs.RemoteAddressComponent(f.Remote); err == nil {
			comps = append(comps, c)
		}

		if f.Protocol != 0 {
			comps = append(comps, fgs.ProtocolComponent(f.Protocol))
		}

		if f.LocalPort != 0 {
			comps = append(comps, fgs.SingleLocalPortComponent(f.LocalPort))
		}

		if f.RemotePort != 0 {
			comps = append(comps, fgs.SingleRemotePortComponent(f.RemotePort))
		}

		out = append(out, fgs.PacketFilter{Identifier: f.ID, Direction: fgs.PacketFilterDirection(f.Direction), Components: comps})
	}

	slices.SortFunc(out, func(a, b fgs.PacketFilter) int { return cmp.Compare(a.Identifier, b.Identifier) })

	return out
}

func flowDescription(qfi uint8, fiveQI uint8, op fgs.QoSFlowOperation, mbr, gbr models.Ambr) (fgs.QoSFlowDescription, error) {
	d := fgs.FiveQIQoSFlow(qfi, fiveQI, op)

	for _, p := range []struct {
		id  fgs.QoSFlowParameterID
		bps uint64
	}{
		{fgs.QoSFlowParamGFBRUplink, gbr.Uplink.Bps()},
		{fgs.QoSFlowParamGFBRDownlink, gbr.Downlink.Bps()},
		{fgs.QoSFlowParamMFBRUplink, mbr.Uplink.Bps()},
		{fgs.QoSFlowParamMFBRDownlink, mbr.Downlink.Bps()},
	} {
		param, err := fgs.BitRateQoSFlowParameter(p.id, p.bps)
		if err != nil {
			return fgs.QoSFlowDescription{}, err
		}

		d.Parameters = append(d.Parameters, param)
	}

	return d, nil
}

func (b *dedicatedBearer) qosFlowDescription(fiveQI uint8, op fgs.QoSFlowOperation, mbr, gbr models.Ambr) (fgs.QoSFlowDescription, error) {
	d, err := flowDescription(b.qfi, fiveQI, op, mbr, gbr)
	if err != nil || b.ebi == 0 {
		return d, err
	}

	param, err := fgs.EPSBearerIDQoSFlowParameter(b.ebi)
	if err != nil {
		return fgs.QoSFlowDescription{}, err
	}

	d.Parameters = append(d.Parameters, param)

	return d, nil
}

func gbrFlow(b *dedicatedBearer, binding bearerBinding, mbr, gbr models.Ambr) smfNgap.GBRQoSFlow {
	return smfNgap.GBRQoSFlow{QFI: b.qfi, FiveQI: int32(binding.QCI), ARP: binding.ARP, MFBR: mbr, GFBR: gbr, ERABID: b.ebi}
}

func (s *SMF) planFlowsLocked(ctx context.Context, sc *SMContext) []RuleReport {
	if !sc.flowsReady() || sc.policyDecision == nil {
		return nil
	}

	unmoved := sc.unmovedFallbackRulesLocked()
	sc.recordFailedRulesLocked(unmoved)

	enforceable, failed := sc.enforceableRulesLocked()
	failed = append(failed, unmoved...)
	want := bindRules(enforceable)

	var (
		change   flowChange
		removed  []*dedicatedBearer
		unsynced []*dedicatedBearer
	)

	for _, b := range slices.Clone(sc.dedicated) {
		if b.state == dedicatedRemovalDeferred {
			if b.ue != nil || b.ran != nil {
				unsynced = append(unsynced, b)
			} else {
				removed = append(removed, b)
			}

			continue
		}

		key := b.target().binding
		if moved, ok := movedBinding(b, want, sc.dedicated); ok {
			key = moved
		}

		rules, ok := want[key]
		delete(want, key)

		switch {
		case b.state == dedicatedAwaitingUE && !ok:
			removed = append(removed, b)
		case b.state == dedicatedAwaitingUE && time.Since(b.deferredSince) > s.dedicatedAwaitLimit:
			removed = append(removed, b)
			failed = append(failed, rules...)
		case b.state == dedicatedAwaitingUE:
			failed = append(failed, s.createFlowLocked(ctx, sc, &change, b, key, rules)...)
		case (!b.enforces() || !ok) && b.ue == nil && b.ran == nil:
			removed = append(removed, b)
		case !b.enforces() || !ok:
			if !sc.upConnectionActive() {
				if b.enforces() {
					s.loseFlowLocked(ctx, sc, b)
				}

				continue
			}

			s.removeFlowLocked(&change, b)
		case b.ueBehind():
			if !sc.upConnectionActive() {
				continue
			}

			s.realignUELocked(&change, b)
		case b.ranBehind():
			change.n2Add = append(change.n2Add, gbrFlow(b, b.installed.binding, b.installed.mbr, b.installed.gbr))
			change.touch(b, false, true)
		default:
			failed = append(failed, s.modifyFlowLocked(ctx, sc, &change, b, key, rules)...)
		}
	}

	for binding, rules := range want {
		b, err := s.newFlowLocked(ctx, sc, binding)
		if err != nil {
			logger.From(ctx, logger.SmfLog).Warn("QoS flow not set up", logger.SUPI(sc.Supi.String()), zap.Uint8("5qi", binding.QCI), zap.Error(err))

			failed = append(failed, rules...)

			continue
		}

		failed = append(failed, s.createFlowLocked(ctx, sc, &change, b, binding, rules)...)
	}

	if err := s.removeDedicatedLocked(ctx, sc, removed...); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not removed", logger.SUPI(sc.Supi.String()), zap.Error(err))
	}

	if len(change.parts) > 0 {
		for _, b := range unsynced {
			s.removeFlowLocked(&change, b)
		}

		failed = append(failed, s.sendFlowChangeLocked(ctx, sc, &change)...)
	}

	sc.recordFailedRulesLocked(failed)

	return sc.ruleReportsLocked(failed)
}

func (s *SMF) newFlowLocked(ctx context.Context, sc *SMContext, binding bearerBinding) (*dedicatedBearer, error) {
	slot, ok := sc.freeDedicatedSlot()
	if !ok {
		return nil, fmt.Errorf("all %d dedicated QoS flows are in use", maxDedicatedBearers)
	}

	qfi, ok := sc.freeQFI()
	if !ok {
		return nil, fmt.Errorf("no free QFI")
	}

	b := &dedicatedBearer{slot: slot, qfi: qfi, pending: &bearerConfig{binding: binding}}

	next := sc.Tunnel.dataPlane
	next.Bearers = append(slices.Clone(next.Bearers), bearerLeg{Slot: slot, QFI: qfi})

	if err := s.applyDataPlane(ctx, sc, next, sc.policyID()); err != nil {
		return nil, fmt.Errorf("add the QoS flow's user plane: %w", err)
	}

	if sc.EBI != 0 && models.GBRQCI(binding.QCI) {
		ebi, err := s.amf.AssignEPSBearerIdentity(sc.Supi, sc.PDUSessionID, sc.Ref)
		if err != nil {
			logger.From(ctx, logger.SmfLog).Info("QoS flow set up without an EPS bearer identity; it stays in 5GS",
				logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", qfi), zap.Error(err))
		}

		b.ebi = ebi
	}

	sc.dedicated = append(sc.dedicated, b)

	return b, nil
}

func (s *SMF) createFlowLocked(ctx context.Context, sc *SMContext, change *flowChange, b *dedicatedBearer, binding bearerBinding, rules []PCCRule) []PCCRule {
	for _, r := range rules {
		if !sc.allocateRuleIdentity(b, r.ID) {
			return rules
		}
	}

	tft, err := b.flowTFT(nil, rules)
	if err == nil && sc.packetFiltersWith(b, tft) > sc.filterLimit() {
		err = fmt.Errorf("the UE supports %d packet filters (TS 24.501 §6.4.1.3)", sc.filterLimit())
	}

	mbr, gbr := bearerRates(rules)

	var desc fgs.QoSFlowDescription
	if err == nil {
		desc, err = b.qosFlowDescription(binding.QCI, fgs.QoSFlowOpCreate, mbr, gbr)
	}

	var mapped fgs.MappedEPSBearerContexts
	if err == nil {
		mapped, err = b.mappedCreate(binding.QCI, mbr, gbr, tft)
	}

	if err == nil {
		err = s.setLegRulesLocked(ctx, sc, b, sc.legRulesLocked(b, ruleSet{tft, rules}))
	}

	if err != nil {
		logger.From(ctx, logger.SmfLog).Warn("QoS flow cannot carry the rules", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Error(err))
		b.pruneRuleIdentities()

		if b.state == dedicatedPlanned {
			if rmErr := s.removeDedicatedLocked(ctx, sc, b); rmErr != nil {
				logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not removed", logger.SUPI(sc.Supi.String()), zap.Error(rmErr))
			}
		}

		return rules
	}

	for _, r := range rules {
		change.rules = append(change.rules, fgs.QoSRule{
			Identifier:    b.ruleIdentity[r.ID].qri,
			OperationCode: fgs.QoSRuleOpCreate,
			Parameters:    &fgs.QoSRuleParameters{Precedence: b.ruleIdentity[r.ID].precedence, QFI: b.qfi},
			Filters:       ruleFilters(tft, r.ID),
		})
	}

	change.flows = append(change.flows, desc)
	change.mapped = append(change.mapped, mapped...)
	change.n2Add = append(change.n2Add, gbrFlow(b, binding, mbr, gbr))
	change.touch(b, true, true)

	b.pending = &bearerConfig{binding: binding, rules: rules, tft: tft, mbr: mbr, gbr: gbr}
	b.state = dedicatedActivating

	return nil
}

func (s *SMF) modifyFlowLocked(ctx context.Context, sc *SMContext, change *flowChange, b *dedicatedBearer, binding bearerBinding, rules []PCCRule) []PCCRule {
	for _, r := range rules {
		if !sc.allocateRuleIdentity(b, r.ID) {
			b.pruneRuleIdentities()
			return addedOrChangedRules(rules, b.installed.rules)
		}
	}

	tft, err := b.flowTFT(b.installed.tft, rules)
	if err == nil && sc.packetFiltersWith(b, tft) > sc.filterLimit() {
		err = fmt.Errorf("the UE supports %d packet filters (TS 24.501 §6.4.1.3)", sc.filterLimit())
	}

	if err != nil {
		logger.From(ctx, logger.SmfLog).Warn("QoS flow cannot carry the rules", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Error(err))
		b.pruneRuleIdentities()

		return addedOrChangedRules(rules, b.installed.rules)
	}

	mbr, gbr := bearerRates(rules)

	if b.refused.refuses(binding, tft, mbr, gbr) {
		b.pruneRuleIdentities()

		return nil
	}

	ops := b.qosRuleOps(b.installed.rules, b.installed.tft, rules, tft)
	ratesChanged := !mbr.Equal(b.installed.mbr) || !gbr.Equal(b.installed.gbr)
	arpChanged := binding != b.installed.binding

	if len(ops) == 0 && !ratesChanged && !arpChanged {
		if err := s.setLegRulesLocked(ctx, sc, b, sc.legRulesLocked(b, ruleSet{b.installed.tft, rules})); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not updated", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Error(err))
			return addedOrChangedRules(rules, b.installed.rules)
		}

		b.installed.rules = rules

		return nil
	}

	if len(ops) == 0 && !ratesChanged && !sc.upConnectionActive() {
		if err := s.setLegRulesLocked(ctx, sc, b, sc.legRulesLocked(b, ruleSet{b.installed.tft, rules})); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not updated", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Error(err))
			return addedOrChangedRules(rules, b.installed.rules)
		}

		b.installed.binding, b.installed.rules = binding, rules

		return nil
	}

	if err := s.setLegRulesLocked(ctx, sc, b, sc.legRulesLocked(b, ruleSet{b.installed.tft, b.installed.rules}, ruleSet{tft, rules})); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not updated", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Error(err))
		b.pruneRuleIdentities()

		return addedOrChangedRules(rules, b.installed.rules)
	}

	mapped, err := b.mappedModify(binding.QCI, mbr, gbr, ratesChanged, b.installed.tft, tft)
	if err != nil {
		s.restoreLegLocked(ctx, sc, b)
		return addedOrChangedRules(rules, b.installed.rules)
	}

	change.rules = append(change.rules, ops...)
	change.mapped = append(change.mapped, mapped...)

	n1 := len(ops) > 0 || len(mapped) > 0

	if ratesChanged {
		desc, err := b.qosFlowDescription(binding.QCI, fgs.QoSFlowOpModify, mbr, gbr)
		if err != nil {
			s.restoreLegLocked(ctx, sc, b)
			return addedOrChangedRules(rules, b.installed.rules)
		}

		change.flows = append(change.flows, desc)
		n1 = true
	}

	n2 := ratesChanged || arpChanged
	if n2 {
		change.n2Add = append(change.n2Add, gbrFlow(b, binding, mbr, gbr))
	}

	change.touch(b, n1, n2)
	b.pending = &bearerConfig{binding: binding, rules: rules, tft: tft, mbr: mbr, gbr: gbr}

	return nil
}

func (b *dedicatedBearer) qosRuleOps(prevRules []PCCRule, prevTFT bearerTFT, rules []PCCRule, tft bearerTFT) fgs.QoSRules {
	var ops fgs.QoSRules

	for _, r := range rules {
		filters := ruleFilters(tft, r.ID)

		switch i := slices.IndexFunc(prevRules, func(o PCCRule) bool { return o.ID == r.ID }); {
		case i < 0:
			ops = append(ops, fgs.QoSRule{
				Identifier:    b.ruleIdentity[r.ID].qri,
				OperationCode: fgs.QoSRuleOpCreate,
				Parameters:    &fgs.QoSRuleParameters{Precedence: b.ruleIdentity[r.ID].precedence, QFI: b.qfi},
				Filters:       filters,
			})
		case !slices.EqualFunc(filters, ruleFilters(prevTFT, r.ID), samePacketFilter):
			ops = append(ops, fgs.QoSRule{Identifier: b.ruleIdentity[r.ID].qri, OperationCode: fgs.QoSRuleOpModifyReplaceFilters, Filters: filters})
		}
	}

	for _, r := range prevRules {
		if !slices.ContainsFunc(rules, func(o PCCRule) bool { return o.ID == r.ID }) {
			ops = append(ops, fgs.QoSRule{Identifier: b.ruleIdentity[r.ID].qri, OperationCode: fgs.QoSRuleOpDelete})
		}
	}

	return ops
}

func samePacketFilter(a, b fgs.PacketFilter) bool {
	return a.Identifier == b.Identifier && a.Direction == b.Direction &&
		slices.EqualFunc(a.Components, b.Components, func(x, y fgs.PacketFilterComponent) bool {
			return x.Type == y.Type && slices.Equal(x.Value, y.Value)
		})
}

func (s *SMF) removeFlowLocked(change *flowChange, b *dedicatedBearer) {
	n1, n2 := b.ue != nil, b.ran != nil

	if n1 {
		for _, qri := range slices.Sorted(slices.Values(b.ueQRIs())) {
			change.rules = append(change.rules, fgs.QoSRule{Identifier: qri, OperationCode: fgs.QoSRuleOpDelete})
		}

		change.flows = append(change.flows, fgs.QoSFlowDescription{QFI: b.qfi, OperationCode: fgs.QoSFlowOpDelete})
		change.mapped = append(change.mapped, b.mappedDelete()...)
	}

	if n2 {
		change.n2Release = append(change.n2Release, b.qfi)
	}

	b.state = dedicatedReleasing
	change.touch(b, n1, n2)
}

func (s *SMF) sendFlowChangeLocked(ctx context.Context, sc *SMContext, change *flowChange) []PCCRule {
	var (
		n1, n2 []byte
		err    error
	)

	if len(change.rules) > 0 || len(change.flows) > 0 || len(change.mapped) > 0 {
		n1, err = smfNas.BuildQoSFlowsModificationCommand(sc.PDUSessionID, networkRequestedPTI, change.rules, change.flows, change.mapped)
	}

	if err == nil && (len(change.n2Add) > 0 || len(change.n2Release) > 0) {
		if !sc.upConnectionActive() {
			err = ErrUENotReachable

			s.requestUserPlaneLocked(sc, change.n2Add)
		} else {
			n2, err = smfNgap.BuildQoSFlowsModifyRequestTransfer(change.n2Add, change.n2Release)
		}
	}

	supi, pduSessionID := sc.Supi, sc.PDUSessionID

	if err == nil {
		err = s.amf.ModifyN1N2(ctx, supi, pduSessionID, n1, n2)
	}

	if err != nil {
		switch {
		case errors.Is(err, ErrHandoverInProgress):
			s.reconcileAfter(sc.Ref, time.Second)
		case !errors.Is(err, ErrUENotReachable):
			logger.From(ctx, logger.SmfLog).Warn("QoS flow modification not sent", logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID), zap.Error(err))
		}

		return s.unsentFlowChangeLocked(ctx, sc, change, errors.Is(err, ErrUENotReachable) || errors.Is(err, ErrHandoverInProgress))
	}

	sc.flowProcedure = newFlowProcedure(change.parts, n1, n2)

	if n1 != nil {
		sc.MarkPTIInUse(networkRequestedPTI)
	}

	ref := sc.Ref

	s.armRetransmit(ctx, sc, s.timerT3591(),
		func(ctx context.Context) error {
			n1, n2, ok := s.unansweredFlowLegs(ref)
			if !ok {
				return nil
			}

			return s.amf.ModifyN1N2(ctx, supi, pduSessionID, n1, n2)
		},
		func(ctx context.Context, sc *SMContext) {
			p := sc.flowProcedure
			if p == nil {
				return
			}

			logger.From(ctx, logger.SmfLog).Warn("T3591 expired; QoS flow modification aborted",
				logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID))

			if p.ue == legPending {
				p.ue = legRefused
			}

			if p.ran == legPending {
				p.ran = legRefused
			}

			s.concludeFlowsLocked(ctx, sc)
		})

	logger.From(ctx, logger.SmfLog).Info("QoS flow modification sent", logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID),
		zap.Int("qos_rules", len(change.rules)), zap.Int("qos_flows", len(change.flows)), zap.Bool("n2", n2 != nil))

	return nil
}

func (s *SMF) unsentFlowChangeLocked(ctx context.Context, sc *SMContext, change *flowChange, unreachable bool) []PCCRule {
	var failed []PCCRule

	for b := range change.parts {
		switch {
		case b.state == dedicatedActivating && unreachable:
			b.state = dedicatedAwaitingUE

			if b.deferredSince.IsZero() {
				b.deferredSince = time.Now()

				s.reconcileAfter(sc.Ref, s.dedicatedAwaitLimit)
			}
		case b.state == dedicatedActivating:
			failed = append(failed, b.pending.rules...)
			b.pending = nil

			if err := s.removeDedicatedLocked(ctx, sc, b); err != nil {
				logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not removed", logger.SUPI(sc.Supi.String()), zap.Error(err))
			}
		case b.state == dedicatedReleasing:
			b.state = dedicatedActive
		case b.pending != nil:
			if !unreachable {
				failed = append(failed, addedOrChangedRules(b.pending.rules, b.installed.rules)...)
			}

			b.pending = nil
			b.pruneRuleIdentities()
			s.restoreLegLocked(ctx, sc, b)
		}
	}

	return failed
}

func (s *SMF) flowAnswered(ctx context.Context, ref string, update func(*flowProcedure) bool) {
	sc := s.GetSession(ref)
	if sc == nil {
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	p := sc.flowProcedure
	if p == nil || !update(p) {
		return
	}

	if p.answered() {
		s.concludeFlowsLocked(ctx, sc)
	}
}

func (s *SMF) qosFlowsModifiedByUE(ctx context.Context, sc *SMContext, accepted bool) bool {
	p := sc.flowProcedure
	if p == nil {
		return sc.pendingPolicy == nil
	}

	if p.ue != legPending {
		return true
	}

	p.ue = legRefused
	if accepted {
		p.ue = legApplied
	}

	if p.answered() {
		s.concludeFlowsLocked(ctx, sc)
	}

	return true
}

func (s *SMF) UpdateSmContextN2InfoPduResModifyRsp(ctx context.Context, ref string, n2 []byte) error {
	outcome, err := smfNgap.ParseQoSFlowsModifyResponse(n2)
	if err != nil {
		return err
	}

	s.flowAnswered(ctx, ref, func(p *flowProcedure) bool {
		if p.ran != legPending {
			return false
		}

		p.ran, p.epsFallback = legApplied, outcome.EPSFallback
		p.ranFailed = append(p.ranFailed, outcome.Failed...)

		return true
	})

	return nil
}

func (s *SMF) UpdateSmContextN2InfoPduResModifyFail(ctx context.Context, ref string, n2 []byte) error {
	outcome, err := smfNgap.ParseModifyUnsuccessful(n2)
	if err != nil {
		return err
	}

	if outcome.Handover {
		s.flowInterruptedByHandover(ctx, ref)
		return nil
	}

	s.flowAnswered(ctx, ref, func(p *flowProcedure) bool {
		if p.ran != legPending {
			return false
		}

		p.ran, p.epsFallback = legRefused, outcome.EPSFallback

		if p.ue == legPending {
			p.ue = legRefused
		}

		return true
	})

	return nil
}

func (s *SMF) concludeFlowsLocked(ctx context.Context, sc *SMContext) {
	p := sc.flowProcedure
	sc.flowProcedure = nil

	sc.stopProcedureTimer()

	if p.n1Msg != nil {
		sc.ClearPTIInUse(networkRequestedPTI)
	}

	var notAllocated, released []PCCRule

	for b, parts := range p.parts {
		ueApplied := !parts.n1 || p.ue == legApplied
		ranApplied := !parts.n2 || p.ran == legApplied && !slices.Contains(p.ranFailed, b.qfi)

		switch {
		case b.state == dedicatedActivating:
			notAllocated = append(notAllocated, s.concludeCreatedFlowLocked(ctx, sc, b, p, ueApplied, ranApplied)...)
		case b.state == dedicatedReleasing:
			s.concludeRemovedFlowLocked(ctx, sc, b, ueApplied, ranApplied)
		case b.pending != nil:
			notAllocated = append(notAllocated, s.concludeModifiedFlowLocked(ctx, sc, b, parts, ueApplied, ranApplied)...)
		case parts.n1 && ueApplied:
			b.ue = b.installed.clone()
			b.pruneRuleIdentities()
		case parts.n2 && ranApplied:
			b.ran = b.installed.clone()
		default:
			released = append(released, s.loseFlowLocked(ctx, sc, b)...)
		}
	}

	if p.epsFallback {
		sc.fallbackUntil = time.Now().Add(s.dedicatedAwaitLimit)
		s.reconcileAfter(sc.Ref, s.dedicatedAwaitLimit)
	}

	notReports := sc.ruleReportsLocked(notAllocated)
	releasedReports := sc.ruleReportsLocked(released)
	sc.recordFailedRulesLocked(append(slices.Clone(notAllocated), released...))

	logger.From(ctx, logger.SmfLog).Info("QoS flow modification concluded", logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID),
		zap.Bool("ue_accepted", p.ue == legApplied), zap.Bool("ran_rejected", p.ran == legRefused), zap.Int("failed_qos_flows", len(p.ranFailed)), zap.Bool("eps_fallback", p.epsFallback))

	s.reportFailedRules(sc, notReports, ResourcesNotAllocated)
	s.reportFailedRules(sc, releasedReports, BearerReleased)

	s.reconcileAfter(sc.Ref, 0)
}

func (s *SMF) concludeCreatedFlowLocked(ctx context.Context, sc *SMContext, b *dedicatedBearer, p *flowProcedure, ueApplied, ranApplied bool) []PCCRule {
	m := b.pending
	b.pending = nil

	b.deferredSince = time.Time{}

	switch {
	case ueApplied && ranApplied:
		b.state = dedicatedActive
		b.installed, b.ue, b.ran = *m, m.clone(), m.clone()
		s.commitFlowLegLocked(ctx, sc, b)

		logger.From(ctx, logger.SmfLog).Info("QoS flow active", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Uint8("5qi", b.installed.binding.QCI))

		return nil
	case ueApplied || ranApplied:
		b.state = dedicatedActive

		if ueApplied {
			b.ue = m.clone()
		}

		if ranApplied {
			b.ran = m.clone()
		}

		s.loseFlowLocked(ctx, sc, b)
	default:
		if err := s.removeDedicatedLocked(ctx, sc, b); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not removed", logger.SUPI(sc.Supi.String()), zap.Error(err))
		}
	}

	if p.epsFallback {
		sc.fallbackRules = append(sc.fallbackRules, m.rules...)
		return nil
	}

	return m.rules
}

func (sc *SMContext) unmovedFallbackRulesLocked() []PCCRule {
	var out []PCCRule

	for _, r := range sc.fallbackRules {
		if slices.ContainsFunc(sc.policyDecision.Rules, r.Equal) {
			out = append(out, r)
		}
	}

	sc.fallbackRules = nil

	return out
}

func (s *SMF) concludeModifiedFlowLocked(ctx context.Context, sc *SMContext, b *dedicatedBearer, parts flowParts, ueApplied, ranApplied bool) []PCCRule {
	m := b.pending
	b.pending = nil

	switch {
	case ueApplied && ranApplied:
		b.installed = *m

		if b.ue != nil {
			b.ue = m.clone()
		}

		if b.ran != nil {
			b.ran = m.clone()
		}

		b.pruneRuleIdentities()
		b.refused = nil
		s.commitFlowLegLocked(ctx, sc, b)

		return nil
	case parts.n1 && ueApplied:
		b.ue = m.clone()
	case parts.n2 && ranApplied:
		b.ran = m.clone()
	}

	b.pruneRuleIdentities()
	s.restoreLegLocked(ctx, sc, b)

	return b.refuse(m)
}

func (b *dedicatedBearer) refuse(m *bearerConfig) []PCCRule {
	failed := addedOrChangedRules(m.rules, b.installed.rules)

	if len(failed) == 0 {
		b.refused = m
	}

	return failed
}

func (s *SMF) concludeRemovedFlowLocked(ctx context.Context, sc *SMContext, b *dedicatedBearer, ueApplied, ranApplied bool) {
	if ueApplied {
		b.ue = nil
	}

	if ranApplied {
		b.ran = nil
	}

	if b.ue == nil && b.ran == nil {
		if err := s.removeDedicatedLocked(ctx, sc, b); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not removed", logger.SUPI(sc.Supi.String()), zap.Error(err))
		}

		logger.From(ctx, logger.SmfLog).Info("QoS flow released", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi))

		return
	}

	b.state = dedicatedRemovalDeferred
	s.loseFlowLocked(ctx, sc, b)
}

func (s *SMF) commitFlowLegLocked(ctx context.Context, sc *SMContext, b *dedicatedBearer) {
	if err := s.updateLegLocked(ctx, sc, b.slot, func(l *bearerLeg) {
		l.Admitted = b.ran != nil
		l.Rules = sc.legRulesLocked(b, ruleSet{b.installed.tft, b.installed.rules})
	}); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not updated", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Error(err))
	}
}

func (s *SMF) interruptFlowProcedureLocked(ctx context.Context, sc *SMContext) {
	p := sc.flowProcedure
	if p == nil {
		return
	}

	sc.flowProcedure = nil
	sc.stopProcedureTimer()

	if p.n1Msg != nil {
		sc.ClearPTIInUse(networkRequestedPTI)
	}

	var unanswered flowChange

	for b, parts := range p.parts {
		if parts.n1 && p.ue != legApplied {
			unanswered.touch(b, parts.n1, parts.n2)
			continue
		}

		switch {
		case b.state == dedicatedReleasing:
			b.ue = nil
			b.state = dedicatedActive
		case b.pending != nil:
			m := b.pending
			b.state, b.pending = dedicatedActive, nil
			b.installed = *m

			if parts.n1 || b.ue != nil {
				b.ue = m.clone()
			}

			b.deferredSince = time.Time{}
			b.pruneRuleIdentities()
			s.commitFlowLegLocked(ctx, sc, b)
		case parts.n1:
			b.ue = b.installed.clone()
			b.pruneRuleIdentities()
		}
	}

	s.unsentFlowChangeLocked(ctx, sc, &unanswered, true)
	s.reconcileAfter(sc.Ref, 0)
}

func (sc *SMContext) carriesPCCRules() bool {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	return len(sc.dedicated) > 0 || sc.policyDecision != nil && len(sc.policyDecision.Rules) > 0
}

func (s *SMF) unansweredFlowLegs(ref string) ([]byte, []byte, bool) {
	sc := s.GetSession(ref)
	if sc == nil {
		return nil, nil, false
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	p := sc.flowProcedure
	if p == nil {
		return nil, nil, false
	}

	var n1, n2 []byte

	if p.ue == legPending {
		n1 = p.n1Msg
	}

	if p.ran == legPending {
		n2 = p.n2Msg
	}

	return n1, n2, n1 != nil || n2 != nil
}

func (s *SMF) realignUELocked(change *flowChange, b *dedicatedBearer) {
	ue := b.ue

	var (
		rules  fgs.QoSRules
		flows  fgs.QoSFlowDescriptions
		mapped fgs.MappedEPSBearerContexts
	)

	rules = b.qosRuleOps(ue.rules, ue.tft, b.installed.rules, b.installed.tft)

	ratesChanged := !ue.mbr.Equal(b.installed.mbr) || !ue.gbr.Equal(b.installed.gbr)
	if ratesChanged {
		if desc, err := b.qosFlowDescription(b.installed.binding.QCI, fgs.QoSFlowOpModify, b.installed.mbr, b.installed.gbr); err == nil {
			flows = append(flows, desc)
		}
	}

	if m, err := b.mappedModify(b.installed.binding.QCI, b.installed.mbr, b.installed.gbr, ratesChanged, ue.tft, b.installed.tft); err == nil {
		mapped = m
	}

	if len(rules) == 0 && len(flows) == 0 && len(mapped) == 0 {
		b.ue = b.installed.clone()
		b.pruneRuleIdentities()

		return
	}

	change.rules = append(change.rules, rules...)
	change.flows = append(change.flows, flows...)
	change.mapped = append(change.mapped, mapped...)
	change.touch(b, true, false)
}

func (s *SMF) flowInterruptedByHandover(ctx context.Context, ref string) {
	sc := s.GetSession(ref)
	if sc == nil {
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	p := sc.flowProcedure
	if p == nil || p.ran != legPending {
		return
	}

	logger.From(ctx, logger.SmfLog).Info("QoS flow modification interrupted by a handover; retrying after it (TS 24.501 §6.3.2.5 f)",
		logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID))

	sc.flowProcedure = nil
	sc.stopProcedureTimer()

	if p.n1Msg != nil {
		sc.ClearPTIInUse(networkRequestedPTI)
	}

	s.unsentFlowChangeLocked(ctx, sc, &flowChange{parts: p.parts}, true)
	s.reconcileAfter(sc.Ref, time.Second)
}

func (sc *SMContext) heldFlowsLocked() []smfNgap.GBRQoSFlow {
	var flows []smfNgap.GBRQoSFlow

	for _, b := range sc.dedicated {
		if b.qfi == 0 || b.state != dedicatedActive || b.ue == nil || !b.enforces() {
			continue
		}

		flows = append(flows, gbrFlow(b, b.installed.binding, b.installed.mbr, b.installed.gbr))
	}

	return flows
}

func (s *SMF) requestUserPlaneLocked(sc *SMContext, flows []smfNgap.GBRQoSFlow) {
	if sc.userPlaneRequested {
		return
	}

	sc.userPlaneRequested = true

	var arp *models.Arp

	for _, f := range flows {
		if arp == nil || f.ARP.PriorityLevel < arp.PriorityLevel {
			arp = &f.ARP
		}
	}

	go func() {
		if err := s.notifyDownlinkWaitingOn(context.Background(), sc, 0, arp, models.DownlinkDataArrived); err != nil {
			logger.SmfLog.Warn("could not activate the user plane for a QoS flow", logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID), zap.Error(err))

			sc.Mutex.Lock()
			sc.userPlaneRequested = false
			sc.Mutex.Unlock()
		}
	}()
}

func (s *SMF) radioReleasedLocked(ctx context.Context, sc *SMContext, preserveGBR bool) {
	s.interruptFlowProcedureLocked(ctx, sc)

	sc.userPlaneRequested = false

	var lost []PCCRule

	for _, b := range sc.dedicated {
		if b.qfi == 0 {
			continue
		}

		wasAdmitted := b.ran != nil
		b.ran = nil

		if preserveGBR || b.state != dedicatedActive || !wasAdmitted {
			continue
		}

		lost = append(lost, s.loseFlowLocked(ctx, sc, b)...)
	}

	s.reportLostRulesLocked(sc, lost)
}

func (s *SMF) loseFlowLocked(ctx context.Context, sc *SMContext, b *dedicatedBearer) []PCCRule {
	lost := b.installed.rules
	b.installed = bearerConfig{}
	b.pruneRuleIdentities()

	if err := s.updateLegLocked(ctx, sc, b.slot, func(l *bearerLeg) { l.Rules, l.Admitted = nil, false }); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("lost QoS flow's user plane not removed", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Error(err))
	}

	return lost
}

func (s *SMF) reportLostRulesLocked(sc *SMContext, lost []PCCRule) {
	if len(lost) == 0 {
		return
	}

	s.reportLostLocked(sc, lost)
	s.reconcileAfter(sc.Ref, 0)
}

func (s *SMF) reportLostLocked(sc *SMContext, lost []PCCRule) {
	if sc.policyDecision != nil {
		lost = slices.DeleteFunc(slices.Clone(lost), func(r PCCRule) bool {
			return !slices.ContainsFunc(sc.policyDecision.Rules, func(p PCCRule) bool { return p.ID == r.ID })
		})
	}

	if len(lost) == 0 {
		return
	}

	reports := sc.ruleReportsLocked(lost)
	sc.recordFailedRulesLocked(lost)

	s.reportFailedRules(sc, reports, BearerReleased)
}

func (s *SMF) admitFlowsLocked(ctx context.Context, sc *SMContext, accepted []uint8) {
	sc.userPlaneRequested = false

	var lost []PCCRule

	for _, b := range sc.dedicated {
		if b.state == dedicatedRemovalDeferred {
			b.state = dedicatedActive

			s.reconcileAfter(sc.Ref, 0)
		}

		if b.refused != nil {
			b.refused = nil

			s.reconcileAfter(sc.Ref, 0)
		}
	}

	for _, b := range sc.dedicated {
		if b.qfi == 0 || b.state != dedicatedActive {
			continue
		}

		if !b.enforces() {
			if !slices.Contains(accepted, b.qfi) {
				b.ran = nil
			}

			continue
		}

		if b.ue == nil {
			continue
		}

		if slices.Contains(accepted, b.qfi) {
			b.ran = b.installed.clone()
			s.commitFlowLegLocked(ctx, sc, b)

			continue
		}

		lost = append(lost, s.loseFlowLocked(ctx, sc, b)...)

		logger.From(ctx, logger.SmfLog).Info("the NG-RAN node did not admit the QoS flow", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi))
	}

	s.reportLostRulesLocked(sc, lost)
}

func (s *SMF) releaseFlowsLocked(ctx context.Context, sc *SMContext, qfis []uint8) {
	var lost []PCCRule

	for _, b := range sc.dedicated {
		if b.qfi == 0 || !slices.Contains(qfis, b.qfi) {
			continue
		}

		if p := sc.flowProcedure; p != nil && b.state == dedicatedActivating && p.parts[b].n2 {
			p.ranFailed = append(p.ranFailed, b.qfi)
			continue
		}

		if b.state != dedicatedActive && b.state != dedicatedRemovalDeferred {
			continue
		}

		b.ran = nil
		lost = append(lost, s.loseFlowLocked(ctx, sc, b)...)
	}

	s.reportLostRulesLocked(sc, lost)
}

func (s *SMF) UpdateSmContextN2InfoNotify(ctx context.Context, ref string, n2 []byte) error {
	released, err := smfNgap.NotifyReleasedQoSFlows(n2)
	if err != nil {
		return err
	}

	sc := s.GetSession(ref)
	if sc == nil {
		return ErrSMContextNotFound
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if len(released) > 0 {
		logger.From(ctx, logger.SmfLog).Info("the NG-RAN node released QoS flows (TS 38.413 §8.2.4.2)",
			logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID), zap.Uint8s("qfis", released))
	}

	s.releaseFlowsLocked(ctx, sc, released)

	return nil
}

type flowDeletion struct {
	flow  *dedicatedBearer
	rules []string
	whole bool
}

func (sc *SMContext) requestedFlowDeletions(req *fgs.PDUSessionModificationRequest) ([]*flowDeletion, bool) {
	if req.Cause == nil || len(req.MappedEPSBearerContexts) > 0 || len(req.RequestedQoSRules)+len(req.RequestedQoSFlows) == 0 {
		return nil, false
	}

	var out []*flowDeletion

	deletion := func(b *dedicatedBearer) *flowDeletion {
		if i := slices.IndexFunc(out, func(d *flowDeletion) bool { return d.flow == b }); i >= 0 {
			return out[i]
		}

		d := &flowDeletion{flow: b}
		out = append(out, d)

		return d
	}

	for _, r := range req.RequestedQoSRules {
		if r.OperationCode != fgs.QoSRuleOpDelete {
			return nil, false
		}

		i := slices.IndexFunc(sc.dedicated, func(b *dedicatedBearer) bool {
			return slices.Contains(b.ueQRIs(), r.Identifier)
		})
		if i < 0 {
			return nil, false
		}

		b := sc.dedicated[i]
		for _, held := range b.ue.rules {
			if b.ruleIdentity[held.ID].qri == r.Identifier {
				deletion(b).rules = append(deletion(b).rules, held.ID)
			}
		}
	}

	for _, f := range req.RequestedQoSFlows {
		i := slices.IndexFunc(sc.dedicated, func(b *dedicatedBearer) bool { return b.ue != nil && b.qfi == f.QFI })
		if f.OperationCode != fgs.QoSFlowOpDelete || i < 0 {
			return nil, false
		}

		deletion(sc.dedicated[i]).whole = true
	}

	return out, true
}

func (s *SMF) acceptFlowDeletionLocked(ctx context.Context, sc *SMContext, deletions []*flowDeletion, pti uint8) (*UpdateResult, error) {
	var (
		rules  fgs.QoSRules
		flows  fgs.QoSFlowDescriptions
		mapped fgs.MappedEPSBearerContexts
		lost   []PCCRule
	)

	for _, d := range deletions {
		b := d.flow
		before := maps.Clone(b.ue.tft)

		gone := d.rules
		if d.whole {
			gone = nil
			for _, r := range b.ue.rules {
				gone = append(gone, r.ID)
			}
		}

		isGone := func(id string) bool { return slices.Contains(gone, id) }

		for _, id := range gone {
			rules = append(rules, fgs.QoSRule{Identifier: b.ruleIdentity[id].qri, OperationCode: fgs.QoSRuleOpDelete})

			if i := slices.IndexFunc(b.installed.rules, func(r PCCRule) bool { return r.ID == id }); i >= 0 {
				lost = append(lost, b.installed.rules[i])
			}
		}

		for _, c := range []*bearerConfig{&b.installed, b.ue} {
			c.rules = slices.DeleteFunc(c.rules, func(r PCCRule) bool { return isGone(r.ID) })
			maps.DeleteFunc(c.tft, func(k filterKey, _ models.SDFFilter) bool { return isGone(k.rule) })
		}

		if d.whole || len(b.ue.rules) == 0 {
			flows = append(flows, fgs.QoSFlowDescription{QFI: b.qfi, OperationCode: fgs.QoSFlowOpDelete})
			mapped = append(mapped, b.mappedDelete()...)
			b.ue = nil
			lost = append(lost, s.loseFlowLocked(ctx, sc, b)...)

			continue
		}

		if m, err := b.mappedModify(b.ue.binding.QCI, b.ue.mbr, b.ue.gbr, false, before, b.ue.tft); err == nil {
			mapped = append(mapped, m...)
		}

		if err := s.setLegRulesLocked(ctx, sc, b, sc.legRulesLocked(b, ruleSet{b.installed.tft, b.installed.rules})); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not updated", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Error(err))
		}
	}

	n1, err := smfNas.BuildQoSFlowsModificationCommand(sc.PDUSessionID, pti, rules, flows, mapped)
	if err != nil {
		return nil, fmt.Errorf("build PDU Session Modification Command (N1): %w", err)
	}

	logger.From(ctx, logger.SmfLog).Info("accepted the UE's deletion of QoS rules (TS 24.501 §6.4.2.2)",
		logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID), zap.Int("qos_rules", len(rules)), zap.Int("qos_flows", len(flows)))

	s.sendRequestedModificationLocked(ctx, sc, pti, n1)

	for _, d := range deletions {
		d.flow.pruneRuleIdentities()
	}

	s.reportLostRulesLocked(sc, lost)

	return &UpdateResult{N1Msg: n1}, nil
}

func (s *SMF) sendRequestedModificationLocked(ctx context.Context, sc *SMContext, pti uint8, n1 []byte) {
	sc.MarkPTIInUse(pti)

	supi, pduSessionID := sc.Supi, sc.PDUSessionID

	s.armRetransmit(ctx, sc, s.timerT3591(),
		func(ctx context.Context) error { return s.amf.ModifyN1N2(ctx, supi, pduSessionID, n1, nil) },
		func(ctx context.Context, sc *SMContext) {
			sc.ClearPTIInUse(pti)
			s.reconcileAfter(sc.Ref, 0)
		})
}
