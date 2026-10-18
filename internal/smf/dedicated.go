// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

const (
	maxDedicatedBearers       = 4
	maxDedicatedFilters       = 15
	dedicatedFilterPrecedence = 32
)

var ErrDedicatedBearerUnknown = errors.New("no such dedicated bearer")

type dedicatedState uint8

const (
	dedicatedPlanned dedicatedState = iota
	dedicatedAwaitingUE
	dedicatedActivating
	dedicatedActive
	dedicatedReleasing
	dedicatedRemovalDeferred
)

type bearerBinding struct {
	QCI uint8
	ARP models.Arp
}

type dedicatedBearer struct {
	slot          uint8
	ebi           uint8
	qfi           uint8
	state         dedicatedState
	deferredSince time.Time

	installed bearerConfig
	pending   *bearerConfig
	ue        *bearerConfig
	ran       *bearerConfig
	refused   *bearerConfig

	ruleIndex    map[string]uint8
	ruleIdentity map[string]ruleIdentity
	fiveGSQoS    bool
}

func (c bearerConfig) clone() *bearerConfig {
	c.rules = slices.Clone(c.rules)
	c.tft = maps.Clone(c.tft)

	return &c
}

func (b *dedicatedBearer) target() bearerConfig {
	if b.pending != nil {
		return *b.pending
	}

	return b.installed
}

func (b *dedicatedBearer) enforces() bool {
	return len(b.installed.rules) > 0
}

func (b *dedicatedBearer) ueBehind() bool {
	if b.ue == nil {
		return false
	}

	ids := func(rules []PCCRule) []string {
		out := make([]string, 0, len(rules))
		for _, r := range rules {
			out = append(out, r.ID)
		}

		slices.Sort(out)

		return out
	}

	return b.ue.binding.QCI != b.installed.binding.QCI || !b.ue.mbr.Equal(b.installed.mbr) || !b.ue.gbr.Equal(b.installed.gbr) ||
		!maps.Equal(b.ue.tft, b.installed.tft) || !slices.Equal(ids(b.ue.rules), ids(b.installed.rules))
}

func (b *dedicatedBearer) ranBehind() bool {
	return b.ran != nil && (b.ran.binding != b.installed.binding || !b.ran.mbr.Equal(b.installed.mbr) || !b.ran.gbr.Equal(b.installed.gbr))
}

type dedicatedAction struct {
	activate *models.DedicatedBearerRequest
	modify   *models.DedicatedBearerModification
	ebi      uint8
	sgwTEID  uint32
}

func bindRules(rules []PCCRule) map[bearerBinding][]PCCRule {
	groups := make(map[bearerBinding][]PCCRule)

	for _, r := range rules {
		k := bearerBinding{QCI: r.QCI, ARP: r.ARP}
		groups[k] = append(groups[k], r)
	}

	return groups
}

func bearerRates(rules []PCCRule) (mbr, gbr models.Ambr) {
	var mbrUL, mbrDL, gbrUL, gbrDL uint64

	for _, r := range rules {
		mbrUL += r.MBR.Uplink.Bps()
		mbrDL += r.MBR.Downlink.Bps()
		gbrUL += r.GBR.Uplink.Bps()
		gbrDL += r.GBR.Downlink.Bps()
	}

	return models.Ambr{Uplink: models.BitRateFromBps(mbrUL), Downlink: models.BitRateFromBps(mbrDL)},
		models.Ambr{Uplink: models.BitRateFromBps(gbrUL), Downlink: models.BitRateFromBps(gbrDL)}
}

func (sc *SMContext) dedicatedBySGWTEID(teid uint32) *dedicatedBearer {
	if sc.Tunnel == nil || teid == 0 {
		return nil
	}

	for _, b := range sc.dedicated {
		if sc.Tunnel.bearerTEID(b.slot) == teid {
			return b
		}
	}

	return nil
}

func (sc *SMContext) freeDedicatedSlot() (uint8, bool) {
	for slot := range uint8(maxDedicatedBearers) {
		inUse := slices.ContainsFunc(sc.dedicated, func(b *dedicatedBearer) bool { return b.slot == slot }) ||
			slices.ContainsFunc(sc.Tunnel.Bearers, func(l bearerLeg) bool { return l.Slot == slot })
		if !inUse {
			return slot, true
		}
	}

	return 0, false
}

func (sc *SMContext) dedicatedReady() bool {
	return sc.Access == Access4G && sc.EBI != 0 && sc.Tunnel != nil && sc.PolicyData != nil &&
		sc.PFCPContext != nil && sc.PFCPContext.Established &&
		!sc.releasing && sc.pending == nil && !sc.activating
}

func (sc *SMContext) installedRuleLocked(id string) *PCCRule {
	for _, b := range sc.dedicated {
		if b.state != dedicatedActive {
			continue
		}

		if i := slices.IndexFunc(b.installed.rules, func(r PCCRule) bool { return r.ID == id }); i >= 0 {
			return &b.installed.rules[i]
		}
	}

	return nil
}

func (sc *SMContext) ruleReportsLocked(failed []PCCRule) []RuleReport {
	reports := make([]RuleReport, 0, len(failed))

	for _, r := range failed {
		report := RuleReport{Rule: r}

		if active := sc.installedRuleLocked(r.ID); active != nil && !active.Equal(r) {
			report.Active = new(*active)
		}

		reports = append(reports, report)
	}

	return reports
}

func (sc *SMContext) gbrQoS(qci uint8) bool {
	if sc.Access == Access5G {
		return models.GBRFiveQI(qci)
	}

	return models.GBRQCI(qci)
}

func (sc *SMContext) enforceableRulesLocked() ([]PCCRule, []PCCRule) {
	var rules, unsupported []PCCRule

	current := make(map[string]PCCRule, len(sc.policyDecision.Rules))

	for _, r := range sc.policyDecision.Rules {
		current[r.ID] = r

		if f, ok := sc.failedRules[r.ID]; ok && f.Equal(r) {
			if active := sc.installedRuleLocked(r.ID); active != nil {
				rules = append(rules, *active)
			}

			continue
		}

		if !sc.gbrQoS(r.QCI) || slices.ContainsFunc(r.Filters, func(f models.SDFFilter) bool { return !f.Constrained() }) {
			unsupported = append(unsupported, r)
			continue
		}

		rules = append(rules, r)
	}

	for id, f := range sc.failedRules {
		if r, ok := current[id]; !ok || !f.Equal(r) {
			delete(sc.failedRules, id)
		}
	}

	return rules, unsupported
}

func (s *SMF) planDedicatedLocked(ctx context.Context, sc *SMContext) ([]dedicatedAction, []RuleReport) {
	if sc.Access == Access5G {
		return nil, s.planFlowsLocked(ctx, sc)
	}

	if !sc.dedicatedReady() || sc.policyDecision == nil {
		return nil, nil
	}

	enforceable, failed := sc.enforceableRulesLocked()
	want := bindRules(enforceable)

	var (
		actions []dedicatedAction
		removed []*dedicatedBearer
	)

	for _, b := range sc.dedicated {
		key := b.target().binding
		if b.pending == nil {
			if moved, ok := movedBinding(b, want, sc.dedicated); ok {
				key = moved
			}
		}

		rules, ok := want[key]
		delete(want, key)

		switch {
		case !ok && b.state == dedicatedActive && b.pending == nil:
			b.state = dedicatedReleasing
			actions = append(actions, dedicatedAction{ebi: b.ebi, sgwTEID: sc.Tunnel.bearerTEID(b.slot)})
		case !ok && b.state == dedicatedAwaitingUE:
			removed = append(removed, b)
		case ok && b.state == dedicatedAwaitingUE && time.Since(b.deferredSince) > s.dedicatedAwaitLimit:
			removed = append(removed, b)
			failed = append(failed, rules...)
		case ok && b.state == dedicatedAwaitingUE:
			b.pending = &bearerConfig{binding: key, rules: rules}

			req, err := s.dedicatedRequest(ctx, sc, b)
			if err != nil {
				removed = append(removed, b)
				failed = append(failed, rules...)

				continue
			}

			b.state = dedicatedActivating

			actions = append(actions, dedicatedAction{activate: req, sgwTEID: req.Bearer.SGW.TEID})
		case ok && b.state == dedicatedActive && b.pending == nil:
			act, notApplied := s.planModificationLocked(ctx, sc, b, key, rules)
			failed = append(failed, notApplied...)

			if act != nil {
				actions = append(actions, *act)
			}
		}
	}

	for binding, rules := range want {
		b, err := s.addDedicatedLocked(ctx, sc, binding, rules)
		if err != nil {
			logger.From(ctx, logger.SmfLog).Warn("dedicated bearer not set up", logger.SUPI(sc.Supi.String()), zap.Uint8("qci", binding.QCI), zap.Error(err))

			failed = append(failed, rules...)

			continue
		}

		req, err := s.dedicatedRequest(ctx, sc, b)
		if err != nil {
			removed = append(removed, b)
			failed = append(failed, rules...)

			continue
		}

		b.state = dedicatedActivating

		actions = append(actions, dedicatedAction{activate: req, sgwTEID: req.Bearer.SGW.TEID})
	}

	if err := s.removeDedicatedLocked(ctx, sc, removed...); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("dedicated bearer user plane not removed", logger.SUPI(sc.Supi.String()), zap.Error(err))
	}

	sc.recordFailedRulesLocked(failed)

	return actions, sc.ruleReportsLocked(failed)
}

func movedBinding(b *dedicatedBearer, want map[bearerBinding][]PCCRule, bearers []*dedicatedBearer) (bearerBinding, bool) {
	if b.state != dedicatedActive || len(b.installed.rules) == 0 {
		return bearerBinding{}, false
	}

	if _, ok := want[b.installed.binding]; ok {
		return bearerBinding{}, false
	}

	for binding, rules := range want {
		if binding.QCI != b.installed.binding.QCI || slices.ContainsFunc(bearers, func(o *dedicatedBearer) bool { return o.installed.binding == binding }) {
			continue
		}

		if !slices.ContainsFunc(b.installed.rules, func(r PCCRule) bool {
			return !slices.ContainsFunc(rules, func(w PCCRule) bool { return w.ID == r.ID })
		}) {
			return binding, true
		}
	}

	return bearerBinding{}, false
}

func (sc *SMContext) recordFailedRulesLocked(failed []PCCRule) {
	if len(failed) == 0 {
		return
	}

	if sc.failedRules == nil {
		sc.failedRules = make(map[string]PCCRule)
	}

	for _, r := range failed {
		sc.failedRules[r.ID] = r
	}
}

func (s *SMF) addDedicatedLocked(ctx context.Context, sc *SMContext, binding bearerBinding, rules []PCCRule) (*dedicatedBearer, error) {
	slot, ok := sc.freeDedicatedSlot()
	if !ok {
		return nil, fmt.Errorf("all %d dedicated bearers are in use", maxDedicatedBearers)
	}

	tft, err := assignFilters(slot, nil, rules)
	if err != nil {
		return nil, err
	}

	b := &dedicatedBearer{slot: slot, pending: &bearerConfig{binding: binding, rules: rules}}

	next := sc.Tunnel.dataPlane
	next.Bearers = append(slices.Clone(next.Bearers), bearerLeg{Slot: slot, Rules: sc.legRulesLocked(b, ruleSet{tft, rules})})

	if err := next.checkSDFCapacity(); err != nil {
		return nil, err
	}

	if err := s.applyDataPlane(ctx, sc, next, sc.policyID()); err != nil {
		return nil, fmt.Errorf("add the bearer's user plane: %w", err)
	}

	sc.dedicated = append(sc.dedicated, b)

	return b, nil
}

func (s *SMF) removeDedicatedLocked(ctx context.Context, sc *SMContext, bearers ...*dedicatedBearer) error {
	sc.dedicated = slices.DeleteFunc(sc.dedicated, func(b *dedicatedBearer) bool { return slices.Contains(bearers, b) })

	if sc.Access == Access5G {
		var ebis []uint8

		for _, b := range bearers {
			if b.ebi != 0 {
				ebis = append(ebis, b.ebi)
			}
		}

		if len(ebis) > 0 {
			s.amf.ReleaseEPSBearerIdentities(sc.Supi, sc.PDUSessionID, sc.Ref, ebis)
		}
	}

	if sc.Tunnel == nil || sc.PFCPContext == nil || !sc.PFCPContext.Established {
		return nil
	}

	next := sc.Tunnel.dataPlane
	next.Bearers = slices.DeleteFunc(slices.Clone(next.Bearers), func(l bearerLeg) bool {
		return !slices.ContainsFunc(sc.dedicated, func(b *dedicatedBearer) bool { return b.slot == l.Slot })
	})

	if len(next.Bearers) == len(sc.Tunnel.Bearers) {
		return nil
	}

	return s.applyDataPlane(ctx, sc, next, sc.policyID())
}

func (s *SMF) updateLegLocked(ctx context.Context, sc *SMContext, slot uint8, update func(*bearerLeg)) error {
	next := sc.Tunnel.dataPlane
	next.Bearers = slices.Clone(next.Bearers)

	i := slices.IndexFunc(next.Bearers, func(l bearerLeg) bool { return l.Slot == slot })
	if i < 0 {
		return fmt.Errorf("dedicated bearer slot %d has no user plane", slot)
	}

	update(&next.Bearers[i])

	if err := next.checkSDFCapacity(); err != nil {
		return err
	}

	return s.applyDataPlane(ctx, sc, next, sc.policyID())
}

func (s *SMF) setLegRulesLocked(ctx context.Context, sc *SMContext, b *dedicatedBearer, rules []ruleLeg) error {
	i := slices.IndexFunc(sc.Tunnel.Bearers, func(l bearerLeg) bool { return l.Slot == b.slot })
	if i < 0 {
		return fmt.Errorf("dedicated bearer slot %d has no user plane", b.slot)
	}

	if slices.EqualFunc(sc.Tunnel.Bearers[i].Rules, rules, sameRuleLeg) {
		return nil
	}

	return s.updateLegLocked(ctx, sc, b.slot, func(l *bearerLeg) { l.Rules = rules })
}

func (s *SMF) dedicatedRequest(ctx context.Context, sc *SMContext, b *dedicatedBearer) (*models.DedicatedBearerRequest, error) {
	t := b.pending

	tft, err := assignFilters(b.slot, nil, t.rules)
	if err != nil {
		return nil, err
	}

	filters := tft.list()
	mbr, gbr := bearerRates(t.rules)
	t.tft, t.mbr, t.gbr = tft, mbr, gbr

	if err := s.setLegRulesLocked(ctx, sc, b, sc.legRulesLocked(b, ruleSet{tft, t.rules})); err != nil {
		return nil, fmt.Errorf("update the bearer's user plane: %w", err)
	}

	teid := sc.Tunnel.bearerTEID(b.slot)
	if teid == 0 {
		return nil, fmt.Errorf("the UPF assigned no uplink endpoint to the bearer")
	}

	mapped, err := sc.mappedFiveGSQoSLocked(b, t.binding.QCI, nil, nil, t.rules, tft, true)
	if err != nil {
		logger.From(ctx, logger.SmfLog).Warn("dedicated bearer activated without its 5GS QoS; it stays in EPS",
			logger.SUPI(sc.Supi.String()), zap.Uint8("qci", t.binding.QCI), zap.Error(err))
	}

	b.fiveGSQoS = mapped != nil

	return &models.DedicatedBearerRequest{
		SessionRef: sc.Ref,
		LinkedEBI:  sc.EBI,
		Bearer: models.DedicatedBearerContext{
			QCI:       t.binding.QCI,
			ARP:       t.binding.ARP,
			MBR:       mbr,
			GBR:       gbr,
			Filters:   filters,
			SGW:       models.FTEID{TEID: teid, Addr: sc.Tunnel.N3IPv4},
			SGWN3IPv6: sc.Tunnel.N3IPv6,
		},
		MappedFiveGSQoS: mapped,
	}, nil
}

func (s *SMF) runDedicated(ctx context.Context, sc *SMContext, actions []dedicatedAction, failed []RuleReport) {
	imsi := sc.Supi.IMSI()

	for _, a := range actions {
		if a.modify != nil {
			if err := s.mme.ModifyDedicatedBearer(ctx, imsi, a.ebi, *a.modify); err != nil {
				s.modificationNotSent(ctx, sc, a.sgwTEID, err)
			}

			continue
		}

		if a.activate != nil {
			err := s.mme.ActivateDedicatedBearer(ctx, imsi, *a.activate)
			if err == nil {
				continue
			}

			if errors.Is(err, ErrUENotReachable) {
				s.dedicatedAwaitUE(sc, a.sgwTEID)
				s.retryAfterMobility(sc, err)

				continue
			}

			logger.From(ctx, logger.SmfLog).Warn("dedicated bearer activation not sent", logger.SUPI(sc.Supi.String()), zap.Error(err))
			s.DedicatedBearerReleased(ctx, sc.Ref, a.sgwTEID)

			continue
		}

		if err := s.mme.DeactivateDedicatedBearer(ctx, imsi, a.ebi, a.sgwTEID); errors.Is(err, ErrUENotReachable) {
			deactivationNotSent(sc, a.sgwTEID)
			s.retryAfterMobility(sc, err)
		} else if err != nil {
			logger.From(ctx, logger.SmfLog).Warn("dedicated bearer deactivation not sent; releasing it locally", logger.SUPI(sc.Supi.String()), zap.Uint8("ebi", a.ebi), zap.Error(err))
			s.DedicatedBearerReleased(ctx, sc.Ref, a.sgwTEID)
		}
	}

	s.reportFailedRules(sc, failed, ResourcesNotAllocated)
}

func deactivationNotSent(sc *SMContext, teid uint32) {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if b := sc.dedicatedBySGWTEID(teid); b != nil && b.state == dedicatedReleasing {
		b.state = dedicatedActive
	}
}

func (s *SMF) dedicatedAwaitUE(sc *SMContext, teid uint32) {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	b := sc.dedicatedBySGWTEID(teid)
	if b == nil || b.state != dedicatedActivating {
		return
	}

	if b.deferredSince.IsZero() {
		b.deferredSince = time.Now()

		s.reconcileAfter(sc.Ref, s.dedicatedAwaitLimit)
	}

	b.state = dedicatedAwaitingUE
}

func (s *SMF) retryAfterMobility(sc *SMContext, err error) {
	if errors.Is(err, ErrHandoverInProgress) {
		s.reconcileAfter(sc.Ref, mobilityRetryInterval)
	}
}

func (s *SMF) reconcileAfter(ref string, d time.Duration) {
	run := func() { s.replanBearers(context.Background(), ref) }

	if d == 0 {
		go run()
		return
	}

	time.AfterFunc(d+time.Second, run)
}

func (s *SMF) reportFailedRules(sc *SMContext, reports []RuleReport, cause EnforcementFailure) {
	if len(reports) == 0 || s.pcf == nil {
		return
	}

	s.pcf.ReportEnforcementFailure(sc.Ref, reports, cause)
}

func (s *SMF) DedicatedBearerActivated(ctx context.Context, ref string, sgwTEID uint32, ebi uint8, enb models.FTEID) error {
	sc := s.GetSession(ref)
	if sc == nil {
		return ErrSMContextNotFound
	}

	sc.Mutex.Lock()

	b, err := s.moveDedicatedLocked(ctx, sc, sgwTEID, enb)
	if err != nil {
		sc.Mutex.Unlock()
		return err
	}

	b.ebi = ebi

	activated := b.state == dedicatedActivating || b.state == dedicatedAwaitingUE
	if activated {
		b.state = dedicatedActive
		b.deferredSince = time.Time{}

		if b.pending != nil {
			b.installed, b.pending = *b.pending, nil
		}
	}
	sc.Mutex.Unlock()

	if activated {
		logger.From(ctx, logger.SmfLog).Info("dedicated bearer active", logger.SUPI(sc.Supi.String()), zap.Uint8("ebi", ebi), zap.Uint8("qci", b.installed.binding.QCI))

		s.reconcileAfter(ref, 0)
	}

	return nil
}

func (s *SMF) DedicatedBearerMoved(ctx context.Context, ref string, sgwTEID uint32, enb models.FTEID) error {
	sc := s.GetSession(ref)
	if sc == nil {
		return ErrSMContextNotFound
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	_, err := s.moveDedicatedLocked(ctx, sc, sgwTEID, enb)

	return err
}

func (s *SMF) moveDedicatedLocked(ctx context.Context, sc *SMContext, sgwTEID uint32, enb models.FTEID) (*dedicatedBearer, error) {
	b := sc.dedicatedBySGWTEID(sgwTEID)
	if b == nil || b.state == dedicatedReleasing {
		return nil, ErrDedicatedBearerUnknown
	}

	an := anchorFromFTEID(enb)

	if i := slices.IndexFunc(sc.Tunnel.Bearers, func(l bearerLeg) bool { return l.Slot == b.slot }); i < 0 || !sc.Tunnel.Bearers[i].AN.equal(an) {
		if err := s.updateLegLocked(ctx, sc, b.slot, func(l *bearerLeg) { l.AN = an }); err != nil {
			return nil, fmt.Errorf("forward the bearer's downlink: %w", err)
		}
	}

	return b, nil
}

func (s *SMF) DedicatedBearerReleased(ctx context.Context, ref string, sgwTEID uint32) {
	sc := s.GetSession(ref)
	if sc == nil {
		return
	}

	sc.Mutex.Lock()

	b := sc.dedicatedBySGWTEID(sgwTEID)
	if b == nil {
		sc.Mutex.Unlock()
		return
	}

	var failed []PCCRule

	if b.state != dedicatedReleasing {
		failed = slices.Clone(b.installed.rules)

		if b.pending != nil {
			failed = append(failed, addedOrChangedRules(b.pending.rules, b.installed.rules)...)
		}
	}

	cause := ResourcesNotAllocated
	if b.state == dedicatedActive {
		cause = BearerReleased
	}

	if err := s.removeDedicatedLocked(ctx, sc, b); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("dedicated bearer user plane not removed", logger.SUPI(sc.Supi.String()), zap.Error(err))
	}

	reports := sc.ruleReportsLocked(failed)
	sc.recordFailedRulesLocked(failed)
	sc.Mutex.Unlock()

	logger.From(ctx, logger.SmfLog).Info("dedicated bearer released", logger.SUPI(sc.Supi.String()), zap.Uint8("ebi", b.ebi), zap.Bool("failed", len(failed) > 0))

	s.reportFailedRules(sc, reports, cause)

	if len(failed) == 0 {
		s.reconcileAfter(ref, 0)
	}
}

func (s *SMF) DedicatedBearerWithoutFiveGSQoS(ctx context.Context, ref string, sgwTEID uint32) {
	sc := s.GetSession(ref)
	if sc == nil {
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if b := sc.dedicatedBySGWTEID(sgwTEID); b != nil {
		b.fiveGSQoS = false

		logger.From(ctx, logger.SmfLog).Info("dedicated bearer signalled without its 5GS QoS; it stays in EPS",
			logger.SUPI(sc.Supi.String()), zap.Uint8("ebi", b.ebi))
	}
}
