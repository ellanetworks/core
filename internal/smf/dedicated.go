// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"
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
	dedicatedAwaitingUE dedicatedState = iota
	dedicatedActivating
	dedicatedActive
	dedicatedReleasing
)

type bearerBinding struct {
	QCI uint8
	ARP models.Arp
}

type dedicatedBearer struct {
	binding       bearerBinding
	slot          uint8
	ebi           uint8
	state         dedicatedState
	rules         []PCCRule
	enb           models.FTEID
	awaitingSince time.Time

	tft       bearerTFT
	ruleIndex map[string]uint8

	qfi                uint8
	qri                map[string]uint8
	precedence         map[string]uint8
	admitted           bool
	ueHolds            bool
	fiveGSQoS          bool
	realignRAN         bool
	realignUE          *pendingModification
	withdraw           bool
	mbr                models.Ambr
	gbr                models.Ambr
	modification       *pendingModification
	modifyWaitingSince time.Time
	rejected           rejectedStep
}

type dedicatedAction struct {
	activate   *models.DedicatedBearerRequest
	modify     *models.DedicatedBearerModification
	ebi        uint8
	deactivate uint8
	sgwTEID    uint32
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

		if i := slices.IndexFunc(b.rules, func(r PCCRule) bool { return r.ID == id }); i >= 0 {
			return &b.rules[i]
		}
	}

	return nil
}

func (sc *SMContext) ruleReportsLocked(failed []PCCRule) []RuleReport {
	reports := make([]RuleReport, 0, len(failed))

	for _, r := range failed {
		report := RuleReport{Rule: r}

		if active := sc.installedRuleLocked(r.ID); active != nil && !samePCCRule(*active, r) {
			report.Active = new(*active)
		}

		reports = append(reports, report)
	}

	return reports
}

func (sc *SMContext) enforceableRulesLocked() ([]PCCRule, []PCCRule) {
	var rules, unsupported []PCCRule

	current := make(map[string]PCCRule, len(sc.policyDecision.Rules))

	for _, r := range sc.policyDecision.Rules {
		current[r.ID] = r

		if f, ok := sc.failedRules[r.ID]; ok && samePCCRule(f, r) {
			if active := sc.installedRuleLocked(r.ID); active != nil {
				rules = append(rules, *active)
			}

			continue
		}

		if !models.GBRQCI(r.QCI) {
			unsupported = append(unsupported, r)
			continue
		}

		rules = append(rules, r)
	}

	for id, f := range sc.failedRules {
		if r, ok := current[id]; !ok || !samePCCRule(f, r) {
			delete(sc.failedRules, id)
		}
	}

	return rules, unsupported
}

func samePCCRule(a, b PCCRule) bool {
	return a.ID == b.ID && a.Version == b.Version && a.QCI == b.QCI && a.ARP == b.ARP &&
		a.MBR.Uplink.Equal(b.MBR.Uplink) && a.MBR.Downlink.Equal(b.MBR.Downlink) &&
		a.GBR.Uplink.Equal(b.GBR.Uplink) && a.GBR.Downlink.Equal(b.GBR.Downlink) &&
		a.Gate == b.Gate && slices.Equal(a.Filters, b.Filters)
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
		key := b.binding
		if b.modification != nil {
			key = b.modification.binding
		} else if moved, ok := movedBinding(b, want, sc.dedicated); ok {
			key = moved
		}

		rules, ok := want[key]
		delete(want, key)

		switch {
		case !ok && b.state == dedicatedActive && b.modification == nil:
			b.state = dedicatedReleasing
			actions = append(actions, dedicatedAction{deactivate: b.ebi, sgwTEID: sc.Tunnel.bearerTEID(b.slot)})
		case !ok && b.state == dedicatedAwaitingUE:
			removed = append(removed, b)
		case ok && b.state == dedicatedAwaitingUE && time.Since(b.awaitingSince) > s.dedicatedAwaitLimit:
			b.rules = rules
			removed = append(removed, b)
			failed = append(failed, b.rules...)
		case ok && b.state == dedicatedAwaitingUE:
			b.rules = rules

			req, err := s.dedicatedRequest(ctx, sc, b)
			if err != nil {
				removed = append(removed, b)
				failed = append(failed, b.rules...)

				continue
			}

			b.state = dedicatedActivating

			actions = append(actions, dedicatedAction{activate: req, sgwTEID: req.SGW.TEID})
		case ok && b.state == dedicatedActive && b.modification == nil:
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
			failed = append(failed, b.rules...)

			continue
		}

		b.state = dedicatedActivating

		actions = append(actions, dedicatedAction{activate: req, sgwTEID: req.SGW.TEID})
	}

	if err := s.removeDedicatedLocked(ctx, sc, removed...); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("dedicated bearer user plane not removed", logger.SUPI(sc.Supi.String()), zap.Error(err))
	}

	sc.recordFailedRulesLocked(failed)

	return actions, sc.ruleReportsLocked(failed)
}

func movedBinding(b *dedicatedBearer, want map[bearerBinding][]PCCRule, bearers []*dedicatedBearer) (bearerBinding, bool) {
	if b.state != dedicatedActive || len(b.rules) == 0 {
		return bearerBinding{}, false
	}

	if _, ok := want[b.binding]; ok {
		return bearerBinding{}, false
	}

	for binding, rules := range want {
		if binding.QCI != b.binding.QCI || slices.ContainsFunc(bearers, func(o *dedicatedBearer) bool { return o.binding == binding }) {
			continue
		}

		if !slices.ContainsFunc(b.rules, func(r PCCRule) bool {
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

	if len(directionFilters(tft.list(), models.FilterUplink)) == 0 {
		return nil, fmt.Errorf("the rules have no uplink packet filter, which a dedicated bearer TFT needs (TS 24.301 §6.4.2.3)")
	}

	b := &dedicatedBearer{binding: binding, slot: slot, rules: rules}

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
	tft, err := assignFilters(b.slot, nil, b.rules)
	if err != nil {
		return nil, err
	}

	filters := tft.list()
	mbr, gbr := bearerRates(b.rules)
	b.tft, b.mbr, b.gbr = tft, mbr, gbr

	if err := s.setLegRulesLocked(ctx, sc, b, sc.legRulesLocked(b, ruleSet{tft, b.rules})); err != nil {
		return nil, fmt.Errorf("update the bearer's user plane: %w", err)
	}

	teid := sc.Tunnel.bearerTEID(b.slot)
	if teid == 0 {
		return nil, fmt.Errorf("the UPF assigned no uplink endpoint to the bearer")
	}

	mapped, err := sc.mappedFiveGSQoSLocked(b, nil, nil, b.rules, tft, true)
	if err != nil {
		logger.From(ctx, logger.SmfLog).Warn("dedicated bearer activated without its 5GS QoS; it stays in EPS",
			logger.SUPI(sc.Supi.String()), zap.Uint8("qci", b.binding.QCI), zap.Error(err))
	}

	b.fiveGSQoS = mapped != nil

	return &models.DedicatedBearerRequest{
		SessionRef:      sc.Ref,
		LinkedEBI:       sc.EBI,
		QCI:             b.binding.QCI,
		ARP:             b.binding.ARP,
		MBR:             mbr,
		GBR:             gbr,
		Filters:         filters,
		SGW:             models.FTEID{TEID: teid, Addr: sc.Tunnel.N3IPv4},
		SGWN3IPv6:       sc.Tunnel.N3IPv6,
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
				continue
			}

			logger.From(ctx, logger.SmfLog).Warn("dedicated bearer activation not sent", logger.SUPI(sc.Supi.String()), zap.Error(err))
			s.DedicatedBearerReleased(ctx, sc.Ref, a.sgwTEID)

			continue
		}

		if err := s.mme.DeactivateDedicatedBearer(ctx, imsi, a.deactivate, a.sgwTEID); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("dedicated bearer deactivation not sent; releasing it locally", logger.SUPI(sc.Supi.String()), zap.Uint8("ebi", a.deactivate), zap.Error(err))
			s.DedicatedBearerReleased(ctx, sc.Ref, a.sgwTEID)
		}
	}

	s.reportFailedRules(sc, failed, ResourcesNotAllocated)
}

func (s *SMF) dedicatedAwaitUE(sc *SMContext, teid uint32) {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	b := sc.dedicatedBySGWTEID(teid)
	if b == nil || b.state != dedicatedActivating {
		return
	}

	if b.awaitingSince.IsZero() {
		b.awaitingSince = time.Now()

		s.reconcileAfter(sc.Ref, s.dedicatedAwaitLimit)
	}

	b.state = dedicatedAwaitingUE
}

func (s *SMF) reconcileAfter(ref string, d time.Duration) {
	run := func() {
		if err := s.ReconcileSession(context.Background(), ref); err != nil {
			logger.SmfLog.Warn("dedicated bearer reconcile failed", logger.SMContextRef(ref), zap.Error(err))
		}
	}

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
	}
	sc.Mutex.Unlock()

	if activated {
		logger.From(ctx, logger.SmfLog).Info("dedicated bearer active", logger.SUPI(sc.Supi.String()), zap.Uint8("ebi", ebi), zap.Uint8("qci", b.binding.QCI))

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

	if err := s.updateLegLocked(ctx, sc, b.slot, func(l *bearerLeg) { l.AN = anchorFromFTEID(enb) }); err != nil {
		return nil, fmt.Errorf("forward the bearer's downlink: %w", err)
	}

	b.enb = enb

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
		failed = slices.Clone(b.rules)

		if b.modification != nil {
			failed = append(failed, addedOrChangedRules(b.modification.rules, b.rules)...)
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
