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
	maxEPSFallbacks                     = 1
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

type flowProcedure struct {
	parts map[*dedicatedBearer]flowParts

	n1         bool
	n1Answered bool
	ueAccepted bool

	n2          bool
	n2Answered  bool
	ranRejected bool
	ranAccepted []uint8
	ranFailed   []uint8
	epsFallback bool

	n1Msg, n2Msg []byte
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

func (sc *SMContext) allocateRuleIdentity(b *dedicatedBearer, id string) bool {
	if _, ok := b.qri[id]; ok {
		return true
	}

	var qris, precedences []uint8

	for _, o := range sc.dedicated {
		qris = append(qris, slices.Collect(maps.Values(o.qri))...)
		precedences = append(precedences, slices.Collect(maps.Values(o.precedence))...)
	}

	qri, ok := firstFree(firstFlowQRI, lastFlowQRI, qris)
	if !ok {
		return false
	}

	precedence, ok := firstFree(firstRulePrecedence, lastRulePrecedence, precedences)
	if !ok {
		return false
	}

	if b.qri == nil {
		b.qri, b.precedence = make(map[string]uint8), make(map[string]uint8)
	}

	b.qri[id], b.precedence[id] = qri, precedence

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

func (b *dedicatedBearer) pruneRuleIdentities(rules []PCCRule) {
	for id := range b.qri {
		if !slices.ContainsFunc(rules, func(r PCCRule) bool { return r.ID == id }) {
			delete(b.qri, id)
			delete(b.precedence, id)
		}
	}
}

func (b *dedicatedBearer) flowTFT(current bearerTFT, rules []PCCRule) (bearerTFT, error) {
	tft, err := assignFilters(b.slot, current, rules)
	if err != nil {
		return nil, err
	}

	for k, f := range tft {
		f.Precedence = b.precedence[k.rule]
		tft[k] = f
	}

	return tft, nil
}

func (sc *SMContext) packetFiltersWith(b *dedicatedBearer, tft bearerTFT) int {
	n := 1 + len(tft)

	for _, o := range sc.dedicated {
		if o == b {
			continue
		}

		if o.modification != nil {
			n += len(o.modification.tft)
		} else {
			n += len(o.tft)
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

	enforceable, failed := sc.enforceableRulesLocked()
	want := bindRules(enforceable)

	var (
		change  flowChange
		removed []*dedicatedBearer
	)

	for _, b := range slices.Clone(sc.dedicated) {
		key := b.binding
		if moved, ok := movedBinding(b, want, sc.dedicated); ok {
			key = moved
		}

		rules, ok := want[key]
		delete(want, key)

		switch {
		case b.state == dedicatedAwaitingUE && !ok:
			removed = append(removed, b)
		case b.state == dedicatedAwaitingUE && time.Since(b.awaitingSince) > s.dedicatedAwaitLimit:
			removed = append(removed, b)
			failed = append(failed, rules...)
		case b.state == dedicatedAwaitingUE:
			failed = append(failed, s.createFlowLocked(ctx, sc, &change, b, key, rules)...)
		case (b.withdraw || !ok) && !b.ueHolds && !b.admitted:
			removed = append(removed, b)
		case b.withdraw || !ok:
			if !sc.upConnectionActive() {
				if !b.withdraw {
					s.loseFlowLocked(ctx, sc, b)
				}

				continue
			}

			if time.Now().Before(b.rejected.until) {
				continue
			}

			s.removeFlowLocked(&change, b)
		case b.realignUE != nil:
			if !sc.upConnectionActive() {
				continue
			}

			s.realignUELocked(&change, b)
		case b.realignRAN:
			change.n2Add = append(change.n2Add, gbrFlow(b, b.binding, b.mbr, b.gbr))
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

	b := &dedicatedBearer{binding: binding, slot: slot, qfi: qfi, state: dedicatedAwaitingUE}

	next := sc.Tunnel.dataPlane
	next.Bearers = append(slices.Clone(next.Bearers), bearerLeg{Slot: slot, QFI: qfi})

	if err := s.applyDataPlane(ctx, sc, next, sc.policyID()); err != nil {
		return nil, fmt.Errorf("add the QoS flow's user plane: %w", err)
	}

	if sc.EBI != 0 {
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

	if err == nil && len(directionFilters(tft.list(), models.FilterUplink)) == 0 {
		err = fmt.Errorf("the rules have no uplink packet filter")
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
		b.pruneRuleIdentities(b.rules)

		if b.state == dedicatedAwaitingUE && len(b.rules) == 0 {
			if rmErr := s.removeDedicatedLocked(ctx, sc, b); rmErr != nil {
				logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not removed", logger.SUPI(sc.Supi.String()), zap.Error(rmErr))
			}
		}

		return rules
	}

	for _, r := range rules {
		change.rules = append(change.rules, fgs.QoSRule{
			Identifier:    b.qri[r.ID],
			OperationCode: fgs.QoSRuleOpCreate,
			Parameters:    &fgs.QoSRuleParameters{Precedence: b.precedence[r.ID], QFI: b.qfi},
			Filters:       ruleFilters(tft, r.ID),
		})
	}

	change.flows = append(change.flows, desc)
	change.mapped = append(change.mapped, mapped...)
	change.n2Add = append(change.n2Add, gbrFlow(b, binding, mbr, gbr))
	change.touch(b, true, true)

	b.modification = &pendingModification{binding: binding, rules: rules, tft: tft, mbr: mbr, gbr: gbr}
	b.state = dedicatedActivating

	return nil
}

func (s *SMF) modifyFlowLocked(ctx context.Context, sc *SMContext, change *flowChange, b *dedicatedBearer, binding bearerBinding, rules []PCCRule) []PCCRule {
	for _, r := range rules {
		if !sc.allocateRuleIdentity(b, r.ID) {
			b.pruneRuleIdentities(b.rules)
			return addedOrChangedRules(rules, b.rules)
		}
	}

	tft, err := b.flowTFT(b.tft, rules)
	if err == nil && sc.packetFiltersWith(b, tft) > sc.filterLimit() {
		err = fmt.Errorf("the UE supports %d packet filters (TS 24.501 §6.4.1.3)", sc.filterLimit())
	}

	if err == nil && len(directionFilters(tft.list(), models.FilterUplink)) == 0 {
		err = fmt.Errorf("the rules leave the QoS flow with no uplink packet filter")
	}

	if err != nil {
		logger.From(ctx, logger.SmfLog).Warn("QoS flow cannot carry the rules", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Error(err))
		b.pruneRuleIdentities(b.rules)

		return addedOrChangedRules(rules, b.rules)
	}

	ops := b.qosRuleOps(b.rules, b.tft, rules, tft)

	mbr, gbr := bearerRates(rules)
	ratesChanged := !ambrEqual(mbr, b.mbr) || !ambrEqual(gbr, b.gbr)
	arpChanged := binding != b.binding

	if len(ops) == 0 && !ratesChanged && !arpChanged {
		if err := s.setLegRulesLocked(ctx, sc, b, sc.legRulesLocked(b, ruleSet{b.tft, rules})); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not updated", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Error(err))
			return addedOrChangedRules(rules, b.rules)
		}

		b.rules = rules

		return nil
	}

	if len(ops) == 0 && !ratesChanged && !sc.upConnectionActive() {
		if err := s.setLegRulesLocked(ctx, sc, b, sc.legRulesLocked(b, ruleSet{b.tft, rules})); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not updated", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Error(err))
			return addedOrChangedRules(rules, b.rules)
		}

		b.binding, b.rules = binding, rules

		return nil
	}

	if err := s.setLegRulesLocked(ctx, sc, b, sc.legRulesLocked(b, ruleSet{b.tft, b.rules}, ruleSet{tft, rules})); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not updated", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Error(err))
		b.pruneRuleIdentities(b.rules)

		return addedOrChangedRules(rules, b.rules)
	}

	mapped, err := b.mappedModify(binding.QCI, mbr, gbr, ratesChanged, b.tft, tft)
	if err != nil {
		s.restoreLegLocked(ctx, sc, b)
		return addedOrChangedRules(rules, b.rules)
	}

	change.rules = append(change.rules, ops...)
	change.mapped = append(change.mapped, mapped...)

	n1 := len(ops) > 0 || len(mapped) > 0

	if ratesChanged {
		desc, err := b.qosFlowDescription(binding.QCI, fgs.QoSFlowOpModify, mbr, gbr)
		if err != nil {
			s.restoreLegLocked(ctx, sc, b)
			return addedOrChangedRules(rules, b.rules)
		}

		change.flows = append(change.flows, desc)
		n1 = true
	}

	n2 := ratesChanged || arpChanged
	if n2 {
		change.n2Add = append(change.n2Add, gbrFlow(b, binding, mbr, gbr))
	}

	change.touch(b, n1, n2)
	b.modification = &pendingModification{binding: binding, rules: rules, tft: tft, mbr: mbr, gbr: gbr}

	return nil
}

func (b *dedicatedBearer) qosRuleOps(prevRules []PCCRule, prevTFT bearerTFT, rules []PCCRule, tft bearerTFT) fgs.QoSRules {
	var ops fgs.QoSRules

	for _, r := range rules {
		filters := ruleFilters(tft, r.ID)

		switch i := slices.IndexFunc(prevRules, func(o PCCRule) bool { return o.ID == r.ID }); {
		case i < 0:
			ops = append(ops, fgs.QoSRule{
				Identifier:    b.qri[r.ID],
				OperationCode: fgs.QoSRuleOpCreate,
				Parameters:    &fgs.QoSRuleParameters{Precedence: b.precedence[r.ID], QFI: b.qfi},
				Filters:       filters,
			})
		case !slices.EqualFunc(filters, ruleFilters(prevTFT, r.ID), samePacketFilter):
			ops = append(ops, fgs.QoSRule{Identifier: b.qri[r.ID], OperationCode: fgs.QoSRuleOpModifyReplaceFilters, Filters: filters})
		}
	}

	for _, r := range prevRules {
		if !slices.ContainsFunc(rules, func(o PCCRule) bool { return o.ID == r.ID }) {
			ops = append(ops, fgs.QoSRule{Identifier: b.qri[r.ID], OperationCode: fgs.QoSRuleOpDelete})
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
	n1 := b.ueHolds
	if n1 {
		for _, id := range slices.Sorted(maps.Keys(b.qri)) {
			change.rules = append(change.rules, fgs.QoSRule{Identifier: b.qri[id], OperationCode: fgs.QoSRuleOpDelete})
		}

		change.flows = append(change.flows, fgs.QoSFlowDescription{QFI: b.qfi, OperationCode: fgs.QoSFlowOpDelete})
		change.mapped = append(change.mapped, b.mappedDelete()...)
	}

	if b.admitted {
		change.n2Release = append(change.n2Release, b.qfi)
	}

	b.state = dedicatedReleasing
	change.touch(b, n1, b.admitted)
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

	sc.flowProcedure = &flowProcedure{parts: change.parts, n1: n1 != nil, n2: n2 != nil, n1Msg: n1, n2Msg: n2}

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

			if !p.n1Answered {
				p.n1Answered, p.ueAccepted = true, false
			}

			if !p.n2Answered {
				p.n2Answered, p.ranRejected = true, true
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
			b.rules = b.modification.rules
			b.modification = nil

			if b.awaitingSince.IsZero() {
				b.awaitingSince = time.Now()

				s.reconcileAfter(sc.Ref, s.dedicatedAwaitLimit)
			}
		case b.state == dedicatedActivating:
			failed = append(failed, b.modification.rules...)
			b.modification = nil
			b.pruneRuleIdentities(nil)

			if err := s.removeDedicatedLocked(ctx, sc, b); err != nil {
				logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not removed", logger.SUPI(sc.Supi.String()), zap.Error(err))
			}
		case b.state == dedicatedReleasing:
			b.state = dedicatedActive
		case b.modification != nil:
			if !unreachable {
				failed = append(failed, addedOrChangedRules(b.modification.rules, b.rules)...)
			}

			b.modification = nil
			b.pruneRuleIdentities(b.rules)
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

	if (!p.n1 || p.n1Answered) && (!p.n2 || p.n2Answered) {
		s.concludeFlowsLocked(ctx, sc)
	}
}

func (s *SMF) qosFlowsModifiedByUE(ctx context.Context, sc *SMContext, accepted bool) bool {
	p := sc.flowProcedure
	if p == nil {
		return sc.pendingPolicy == nil
	}

	if !p.n1 || p.n1Answered {
		return true
	}

	p.n1Answered, p.ueAccepted = true, accepted

	if !p.n2 || p.n2Answered {
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
		if !p.n2 || p.n2Answered {
			return false
		}

		p.n2Answered, p.ranAccepted, p.ranFailed, p.epsFallback = true, outcome.Accepted, outcome.Failed, outcome.EPSFallback

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
		if !p.n2 || p.n2Answered {
			return false
		}

		p.n2Answered, p.ranRejected, p.epsFallback = true, true, outcome.EPSFallback

		if p.n1 && !p.n1Answered {
			p.n1Answered, p.ueAccepted = true, false
		}

		return true
	})

	return nil
}

func (s *SMF) concludeFlowsLocked(ctx context.Context, sc *SMContext) {
	p := sc.flowProcedure
	sc.flowProcedure = nil

	sc.stopProcedureTimer()

	if p.n1 {
		sc.ClearPTIInUse(networkRequestedPTI)
	}

	var notAllocated, released []PCCRule

	if p.epsFallback {
		sc.fallbacks++
	}

	for b, parts := range p.parts {
		ueApplied := !parts.n1 || p.ueAccepted
		ranApplied := !parts.n2 || !p.ranRejected && !slices.Contains(p.ranFailed, b.qfi) &&
			(b.state == dedicatedReleasing || slices.Contains(p.ranAccepted, b.qfi))

		switch {
		case b.state == dedicatedActivating:
			notAllocated = append(notAllocated, s.concludeCreatedFlowLocked(ctx, sc, b, p, ueApplied, ranApplied)...)
		case b.state == dedicatedReleasing:
			s.concludeRemovedFlowLocked(ctx, sc, b, ueApplied, ranApplied)
		case b.modification != nil:
			notAllocated = append(notAllocated, s.concludeModifiedFlowLocked(ctx, sc, b, parts, ueApplied, ranApplied)...)
		case b.realignUE != nil && ueApplied:
			b.realignUE = nil
			b.pruneRuleIdentities(b.rules)
		case b.realignUE != nil:
			b.realignUE = nil
			b.withdraw = true
			released = append(released, b.rules...)
		case b.realignRAN && ranApplied:
			b.realignRAN = false
		case b.realignRAN:
			b.realignRAN = false
			b.withdraw = true
			released = append(released, b.rules...)
		}
	}

	if p.epsFallback && sc.fallbacks <= maxEPSFallbacks {
		sc.fallbackUntil = time.Now().Add(s.dedicatedAwaitLimit)
		s.reconcileAfter(sc.Ref, s.dedicatedAwaitLimit)
	} else {
		sc.fallbacks = 0
	}

	notReports := sc.ruleReportsLocked(notAllocated)
	releasedReports := sc.ruleReportsLocked(released)
	sc.recordFailedRulesLocked(append(slices.Clone(notAllocated), released...))

	logger.From(ctx, logger.SmfLog).Info("QoS flow modification concluded", logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID),
		zap.Bool("ue_accepted", p.ueAccepted), zap.Bool("ran_rejected", p.ranRejected), zap.Int("failed_qos_flows", len(p.ranFailed)), zap.Bool("eps_fallback", p.epsFallback))

	go func() {
		s.reportFailedRules(sc, notReports, ResourcesNotAllocated)
		s.reportFailedRules(sc, releasedReports, BearerReleased)
	}()

	s.reconcileAfter(sc.Ref, 0)
}

func (s *SMF) concludeCreatedFlowLocked(ctx context.Context, sc *SMContext, b *dedicatedBearer, p *flowProcedure, ueApplied, ranApplied bool) []PCCRule {
	m := b.modification
	b.modification = nil

	switch {
	case ueApplied && ranApplied:
		b.state = dedicatedActive
		b.binding, b.tft, b.mbr, b.gbr, b.rules = m.binding, m.tft, m.mbr, m.gbr, m.rules
		b.ueHolds, b.admitted = true, true
		b.awaitingSince = time.Time{}
		s.commitFlowLegLocked(ctx, sc, b)

		logger.From(ctx, logger.SmfLog).Info("QoS flow active", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Uint8("5qi", b.binding.QCI))

		return nil
	case ueApplied || ranApplied:
		b.state = dedicatedActive
		b.binding, b.tft, b.mbr, b.gbr, b.rules = m.binding, m.tft, m.mbr, m.gbr, m.rules
		b.ueHolds, b.admitted = ueApplied, ranApplied
		b.withdraw = true
		s.commitFlowLegLocked(ctx, sc, b)
	default:
		b.pruneRuleIdentities(nil)

		if err := s.removeDedicatedLocked(ctx, sc, b); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not removed", logger.SUPI(sc.Supi.String()), zap.Error(err))
		}
	}

	if p.epsFallback && sc.fallbacks <= maxEPSFallbacks {
		return nil
	}

	return m.rules
}

func (s *SMF) concludeModifiedFlowLocked(ctx context.Context, sc *SMContext, b *dedicatedBearer, parts flowParts, ueApplied, ranApplied bool) []PCCRule {
	m := b.modification
	b.modification = nil

	switch {
	case ueApplied && ranApplied:
		b.binding, b.tft, b.mbr, b.gbr, b.rules = m.binding, m.tft, m.mbr, m.gbr, m.rules
		b.pruneRuleIdentities(b.rules)
		b.rejected = rejectedStep{}
		s.commitFlowLegLocked(ctx, sc, b)

		return nil
	case parts.n1 && ueApplied:
		b.realignUE = m
		b.pruneRuleIdentities(append(slices.Clone(b.rules), m.rules...))
		s.restoreLegLocked(ctx, sc, b)

		return addedOrChangedRules(m.rules, b.rules)
	default:
		failed := addedOrChangedRules(m.rules, b.rules)
		b.pruneRuleIdentities(b.rules)
		b.realignRAN = parts.n2 && ranApplied && (m.binding != b.binding || !ambrEqual(m.mbr, b.mbr) || !ambrEqual(m.gbr, b.gbr))
		s.restoreLegLocked(ctx, sc, b)

		if len(failed) == 0 {
			b.rejected = rejectedStep{until: time.Now().Add(s.dedicatedAwaitLimit)}
			s.reconcileAfter(sc.Ref, s.dedicatedAwaitLimit)
		}

		return failed
	}
}

func (s *SMF) concludeRemovedFlowLocked(ctx context.Context, sc *SMContext, b *dedicatedBearer, ueApplied, ranApplied bool) {
	if ueApplied {
		b.ueHolds = false
	}

	if ranApplied {
		b.admitted = false
	}

	if !b.ueHolds && !b.admitted {
		if err := s.removeDedicatedLocked(ctx, sc, b); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not removed", logger.SUPI(sc.Supi.String()), zap.Error(err))
		}

		logger.From(ctx, logger.SmfLog).Info("QoS flow released", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi))

		return
	}

	b.state = dedicatedActive
	b.rejected = rejectedStep{until: time.Now().Add(s.dedicatedAwaitLimit)}
	s.reconcileAfter(sc.Ref, s.dedicatedAwaitLimit)
}

func (s *SMF) commitFlowLegLocked(ctx context.Context, sc *SMContext, b *dedicatedBearer) {
	if err := s.updateLegLocked(ctx, sc, b.slot, func(l *bearerLeg) {
		l.Admitted = b.admitted
		l.Rules = sc.legRulesLocked(b, ruleSet{b.tft, b.rules})
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

	if p.n1 {
		sc.ClearPTIInUse(networkRequestedPTI)
	}

	s.unsentFlowChangeLocked(ctx, sc, &flowChange{parts: p.parts}, true)
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

	if !p.n1Answered {
		n1 = p.n1Msg
	}

	if !p.n2Answered {
		n2 = p.n2Msg
	}

	return n1, n2, n1 != nil || n2 != nil
}

func (s *SMF) realignUELocked(change *flowChange, b *dedicatedBearer) {
	ue := b.realignUE

	var (
		rules  fgs.QoSRules
		flows  fgs.QoSFlowDescriptions
		mapped fgs.MappedEPSBearerContexts
	)

	rules = b.qosRuleOps(ue.rules, ue.tft, b.rules, b.tft)

	ratesChanged := !ambrEqual(ue.mbr, b.mbr) || !ambrEqual(ue.gbr, b.gbr)
	if ratesChanged {
		if desc, err := b.qosFlowDescription(b.binding.QCI, fgs.QoSFlowOpModify, b.mbr, b.gbr); err == nil {
			flows = append(flows, desc)
		}
	}

	if m, err := b.mappedModify(b.binding.QCI, b.mbr, b.gbr, ratesChanged, ue.tft, b.tft); err == nil {
		mapped = m
	}

	if len(rules) == 0 && len(flows) == 0 && len(mapped) == 0 {
		b.realignUE = nil
		b.pruneRuleIdentities(b.rules)

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
	if p == nil || !p.n2 || p.n2Answered {
		return
	}

	logger.From(ctx, logger.SmfLog).Info("QoS flow modification interrupted by a handover; retrying after it (TS 24.501 §6.3.2.5 f)",
		logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID))

	sc.flowProcedure = nil
	sc.stopProcedureTimer()

	if p.n1 {
		sc.ClearPTIInUse(networkRequestedPTI)
	}

	s.unsentFlowChangeLocked(ctx, sc, &flowChange{parts: p.parts}, true)
	s.reconcileAfter(sc.Ref, time.Second)
}

func (sc *SMContext) heldFlowsLocked() []smfNgap.GBRQoSFlow {
	var flows []smfNgap.GBRQoSFlow

	for _, b := range sc.dedicated {
		if b.qfi == 0 || b.state != dedicatedActive || !b.ueHolds || b.withdraw || len(b.rules) == 0 {
			continue
		}

		flows = append(flows, gbrFlow(b, b.binding, b.mbr, b.gbr))
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

		wasAdmitted := b.admitted
		b.admitted = false
		b.realignRAN = false

		if preserveGBR || b.state != dedicatedActive || !wasAdmitted {
			continue
		}

		if !b.withdraw {
			lost = append(lost, b.rules...)
		}

		s.loseFlowLocked(ctx, sc, b)
	}

	s.reportLostRulesLocked(sc, lost)
}

func (s *SMF) loseFlowLocked(ctx context.Context, sc *SMContext, b *dedicatedBearer) {
	b.withdraw = true

	if err := s.updateLegLocked(ctx, sc, b.slot, func(l *bearerLeg) { l.Rules, l.Admitted = nil, false }); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("lost QoS flow's user plane not removed", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Error(err))
	}
}

func (s *SMF) reportLostRulesLocked(sc *SMContext, lost []PCCRule) {
	if len(lost) == 0 {
		return
	}

	reports := sc.ruleReportsLocked(lost)
	sc.recordFailedRulesLocked(lost)

	go s.reportFailedRules(sc, reports, BearerReleased)

	s.reconcileAfter(sc.Ref, 0)
}

func (s *SMF) admitFlowsLocked(ctx context.Context, sc *SMContext, accepted []uint8) {
	sc.userPlaneRequested = false

	var lost []PCCRule

	for _, b := range sc.dedicated {
		if b.qfi == 0 || b.state != dedicatedActive || !b.ueHolds || b.withdraw || len(b.rules) == 0 {
			continue
		}

		if slices.Contains(accepted, b.qfi) {
			b.admitted = true
			s.commitFlowLegLocked(ctx, sc, b)

			continue
		}

		lost = append(lost, b.rules...)
		s.loseFlowLocked(ctx, sc, b)

		logger.From(ctx, logger.SmfLog).Info("the NG-RAN node did not admit the QoS flow", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi))
	}

	s.reportLostRulesLocked(sc, lost)
}

func (s *SMF) releaseFlowsLocked(ctx context.Context, sc *SMContext, qfis []uint8) {
	var lost []PCCRule

	for _, b := range sc.dedicated {
		if b.qfi == 0 || !slices.Contains(qfis, b.qfi) || b.state != dedicatedActive {
			continue
		}

		b.admitted = false

		if !b.withdraw {
			lost = append(lost, b.rules...)
		}

		s.loseFlowLocked(ctx, sc, b)
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
			return b.ueHolds && slices.Contains(slices.Collect(maps.Values(b.qri)), r.Identifier)
		})
		if i < 0 {
			return nil, false
		}

		b := sc.dedicated[i]
		for id, qri := range b.qri {
			if qri == r.Identifier {
				deletion(b).rules = append(deletion(b).rules, id)
			}
		}
	}

	for _, f := range req.RequestedQoSFlows {
		i := slices.IndexFunc(sc.dedicated, func(b *dedicatedBearer) bool { return b.ueHolds && b.qfi == f.QFI })
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
		before := maps.Clone(b.tft)

		gone := d.rules
		if d.whole {
			gone = slices.Collect(maps.Keys(b.qri))
		}

		for _, id := range gone {
			rules = append(rules, fgs.QoSRule{Identifier: b.qri[id], OperationCode: fgs.QoSRuleOpDelete})

			if i := slices.IndexFunc(b.rules, func(r PCCRule) bool { return r.ID == id }); i >= 0 {
				lost = append(lost, b.rules[i])
			}
		}

		b.rules = slices.DeleteFunc(b.rules, func(r PCCRule) bool { return slices.Contains(gone, r.ID) })
		maps.DeleteFunc(b.tft, func(k filterKey, _ models.SDFFilter) bool { return slices.Contains(gone, k.rule) })

		if d.whole || len(b.rules) == 0 {
			flows = append(flows, fgs.QoSFlowDescription{QFI: b.qfi, OperationCode: fgs.QoSFlowOpDelete})
			mapped = append(mapped, b.mappedDelete()...)
			b.ueHolds = false
			s.loseFlowLocked(ctx, sc, b)

			continue
		}

		if m, err := b.mappedModify(b.binding.QCI, b.mbr, b.gbr, false, before, b.tft); err == nil {
			mapped = append(mapped, m...)
		}

		if err := s.setLegRulesLocked(ctx, sc, b, sc.legRulesLocked(b, ruleSet{b.tft, b.rules})); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not updated", logger.SUPI(sc.Supi.String()), zap.Uint8("qfi", b.qfi), zap.Error(err))
		}
	}

	n1, err := smfNas.BuildQoSFlowsModificationCommand(sc.PDUSessionID, pti, rules, flows, mapped)
	if err != nil {
		return nil, fmt.Errorf("build PDU Session Modification Command (N1): %w", err)
	}

	logger.From(ctx, logger.SmfLog).Info("accepted the UE's deletion of QoS rules (TS 24.501 §6.4.2.2)",
		logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID), zap.Int("qos_rules", len(rules)), zap.Int("qos_flows", len(flows)))

	sc.MarkPTIInUse(pti)

	supi, pduSessionID := sc.Supi, sc.PDUSessionID

	s.armRetransmit(ctx, sc, s.timerT3591(),
		func(ctx context.Context) error { return s.amf.ModifyN1N2(ctx, supi, pduSessionID, n1, nil) },
		func(ctx context.Context, sc *SMContext) {
			sc.ClearPTIInUse(pti)
			s.reconcileAfter(sc.Ref, 0)
		})

	for _, d := range deletions {
		d.flow.pruneRuleIdentities(d.flow.rules)
	}

	s.reportLostRulesLocked(sc, lost)

	return &UpdateResult{N1Msg: n1}, nil
}
