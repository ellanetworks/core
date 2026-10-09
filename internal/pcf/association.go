// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf

import (
	"cmp"
	"context"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/ellanetworks/core/diameter/rx"
	"github.com/ellanetworks/core/internal/smf"
	"go.uber.org/zap"
)

const ipv6PrefixBits = 64

type association struct {
	ref        string
	context    smf.PolicyContext
	decision   *smf.PolicyDecision
	rxSessions map[string]*rxSession
}

type Enforcer interface {
	UpdateNotify(ctx context.Context, ref string, d *smf.PolicyDecision) error
}

func addressKey(addr netip.Addr) netip.Addr {
	addr = addr.Unmap()
	if addr.Is4() {
		return addr
	}

	return netip.PrefixFrom(addr, ipv6PrefixBits).Masked().Addr()
}

func (a *association) keys() []netip.Addr {
	var keys []netip.Addr

	if a.context.IPv4.IsValid() {
		keys = append(keys, addressKey(a.context.IPv4))
	}

	if a.context.IPv6Prefix.IsValid() {
		keys = append(keys, addressKey(a.context.IPv6Prefix.Addr()))
	}

	return keys
}

func (p *PCF) SetEnforcer(e Enforcer) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.enforcer = e
}

func (p *PCF) CreateAssociation(ctx context.Context, ref string, c smf.PolicyContext) (*smf.PolicyDecision, error) {
	d, err := p.decide(ctx, c)
	if err != nil {
		return nil, err
	}

	p.TerminateAssociation(ref)

	p.mu.Lock()
	defer p.mu.Unlock()

	d.Revision = p.nextRevisionLocked()
	assoc := &association{ref: ref, context: c, decision: d, rxSessions: make(map[string]*rxSession)}
	p.associations[ref] = assoc

	for _, k := range assoc.keys() {
		p.byAddress[k] = assoc
	}

	return d, nil
}

func (p *PCF) UpdateAssociation(ctx context.Context, ref string, subscribed smf.SubscribedQoS) (*smf.PolicyDecision, error) {
	p.mu.Lock()
	a, ok := p.associations[ref]

	var c smf.PolicyContext
	if ok {
		c = a.context
	}
	p.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("no policy association %q", ref)
	}

	c.Subscribed = subscribed

	d, err := p.decide(ctx, c)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.associations[ref] != a {
		return nil, fmt.Errorf("policy association %q ended during its update", ref)
	}

	d.Revision = p.nextRevisionLocked()
	d.Rules = a.rules()
	a.context, a.decision = c, d

	return d, nil
}

func (p *PCF) ReportEnforcementFailure(ref string, reports []smf.RuleReport, cause smf.EnforcementFailure) {
	ruleIDs := make([]string, 0, len(reports))
	for _, r := range reports {
		ruleIDs = append(ruleIDs, r.Rule.ID)
	}

	p.log.Info("SMF could not enforce the policy decision", zap.String("association", ref), zap.Strings("rules", ruleIDs))

	if len(reports) == 0 {
		return
	}

	type failure struct {
		session *rxSession
		action  rx.SpecificAction
		flows   []rx.Flows
	}

	p.mu.Lock()

	a, ok := p.associations[ref]
	if !ok {
		p.mu.Unlock()
		return
	}

	var failures []failure

	for _, s := range a.rxSessions {
		if s.aborted {
			continue
		}

		flows := s.applyReports(reports)
		if len(flows) == 0 {
			continue
		}

		if len(s.rules) == 0 {
			s.aborted = true
			failures = append(failures, failure{session: s})

			continue
		}

		action, subscribed := failureAction(s.actions, cause)
		if !subscribed {
			continue
		}

		failures = append(failures, failure{session: s, action: action, flows: flows})
	}

	d := p.refreshRulesLocked(a)
	p.mu.Unlock()

	p.notify(ref, d)

	for _, f := range failures {
		if f.session.aborted {
			go p.abort(f.session, rx.AbortBearerReleased)
		} else {
			go p.reportFailedFlows(f.session, f.action, f.flows)
		}
	}
}

func (s *rxSession) applyReports(reports []smf.RuleReport) []rx.Flows {
	type affected struct {
		flows  []uint32
		active bool
	}

	byComponent := make(map[uint32]*affected)

	for k, r := range s.rules {
		i := slices.IndexFunc(reports, func(f smf.RuleReport) bool { return samePCCRule(f.Rule, r) })
		if i < 0 {
			continue
		}

		a, ok := byComponent[k.component]
		if !ok {
			a = &affected{active: true}
			byComponent[k.component] = a
		}

		a.flows = append(a.flows, k.flow)

		if active := reports[i].Active; active != nil && active.ID == r.ID {
			s.rules[k] = *active
		} else {
			delete(s.rules, k)

			a.active = false
		}
	}

	flows := make([]rx.Flows, 0, len(byComponent))

	for n, a := range byComponent {
		status := rx.MediaComponentInactive
		if a.active {
			status = rx.MediaComponentActive
		}

		slices.Sort(a.flows)
		flows = append(flows, rx.Flows{MediaComponentNumber: n, FlowNumbers: a.flows, MediaComponentStatus: &status})
	}

	slices.SortFunc(flows, func(x, y rx.Flows) int { return cmp.Compare(x.MediaComponentNumber, y.MediaComponentNumber) })

	return flows
}

func failureAction(actions []rx.SpecificAction, cause smf.EnforcementFailure) (rx.SpecificAction, bool) {
	preference := []rx.SpecificAction{rx.ActionIndicationOfFailedResourcesAllocation, rx.ActionIndicationOfReleaseOfBearer}
	if cause == smf.BearerReleased {
		preference = []rx.SpecificAction{rx.ActionIndicationOfReleaseOfBearer, rx.ActionIndicationOfFailedResourcesAllocation}
	}

	for _, a := range preference {
		if slices.Contains(actions, a) {
			return a, true
		}
	}

	return 0, false
}

func (p *PCF) Reconcile(ctx context.Context) {
	p.mu.Lock()
	enforcer := p.enforcer

	pending := make([]*association, 0, len(p.associations))
	for _, a := range p.associations {
		pending = append(pending, a)
	}
	p.mu.Unlock()

	if enforcer == nil {
		return
	}

	for _, a := range pending {
		p.mu.Lock()
		c, current := a.context, a.decision
		p.mu.Unlock()

		d, err := p.decide(ctx, c)
		if err != nil {
			p.log.Debug("policy decision unavailable, keeping the association's decision", zap.String("association", a.ref), zap.Error(err))
			continue
		}

		if sameDecision(d, current) {
			continue
		}

		p.mu.Lock()
		if p.associations[a.ref] != a || a.decision != current {
			p.mu.Unlock()
			continue
		}

		d.Revision = p.nextRevisionLocked()
		d.Rules = a.rules()
		a.decision = d
		p.mu.Unlock()

		if err := enforcer.UpdateNotify(ctx, a.ref, d); err != nil {
			p.log.Warn("policy update not delivered to the SMF", zap.String("association", a.ref), zap.Error(err))
		}
	}
}

func (p *PCF) TerminateAssociation(ref string) {
	p.mu.Lock()

	a, ok := p.associations[ref]
	if !ok {
		p.mu.Unlock()
		return
	}

	delete(p.associations, ref)
	p.unindexLocked(a)

	aborted := make([]*rxSession, 0, len(a.rxSessions))

	for _, s := range a.rxSessions {
		s.aborted = true
		aborted = append(aborted, s)
	}

	p.mu.Unlock()

	for _, s := range aborted {
		go p.abort(s, rx.AbortBearerReleased)
	}
}

func (a *association) rules() []smf.PCCRule {
	var rules []smf.PCCRule

	for _, s := range a.rxSessions {
		if s.aborted {
			continue
		}

		for _, r := range s.rules {
			rules = append(rules, r)
		}
	}

	slices.SortFunc(rules, func(x, y smf.PCCRule) int { return strings.Compare(x.ID, y.ID) })

	return rules
}

func (p *PCF) refreshRulesLocked(a *association) *smf.PolicyDecision {
	rules := a.rules()
	if slices.EqualFunc(rules, a.decision.Rules, samePCCRule) {
		return nil
	}

	d := *a.decision
	d.Rules = rules
	d.Revision = p.nextRevisionLocked()
	a.decision = &d

	ids := make([]string, 0, len(rules))
	for _, r := range rules {
		ids = append(ids, fmt.Sprintf("%s@%d", r.ID, r.Version))
	}

	p.log.Debug("policy decision updated", zap.String("association", a.ref), zap.Uint64("revision", d.Revision), zap.Strings("rules", ids))

	return &d
}

func (p *PCF) notify(ref string, d *smf.PolicyDecision) {
	if d == nil {
		return
	}

	p.mu.Lock()
	enforcer := p.enforcer
	p.mu.Unlock()

	if enforcer == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), p.notifyTimeout)
		defer cancel()

		if err := enforcer.UpdateNotify(ctx, ref, d); err != nil {
			p.log.Warn("policy update not delivered to the SMF", zap.String("association", ref), zap.Uint64("revision", d.Revision), zap.Error(err))
		}
	}()
}

func (p *PCF) nextRevisionLocked() uint64 {
	p.revision++

	return p.revision
}

func (p *PCF) unindexLocked(a *association) {
	for _, k := range a.keys() {
		if p.byAddress[k] == a {
			delete(p.byAddress, k)
		}
	}
}

func (p *PCF) boundAssociationLocked(dnn string, addr netip.Addr) *association {
	if !addr.IsValid() {
		return nil
	}

	a, ok := p.byAddress[addressKey(addr)]
	if !ok || !strings.EqualFold(a.context.Dnn, dnn) {
		return nil
	}

	return a
}

func samePCCRule(a, b smf.PCCRule) bool {
	return a.ID == b.ID && a.Version == b.Version && a.QCI == b.QCI && a.ARP == b.ARP &&
		a.MBR.Uplink.Equal(b.MBR.Uplink) && a.MBR.Downlink.Equal(b.MBR.Downlink) &&
		a.GBR.Uplink.Equal(b.GBR.Uplink) && a.GBR.Downlink.Equal(b.GBR.Downlink) &&
		a.Gate == b.Gate && slices.Equal(a.Filters, b.Filters)
}

func sameDecision(a, b *smf.PolicyDecision) bool {
	return a.PolicyID == b.PolicyID && a.Var5qi == b.Var5qi && a.Arp == b.Arp &&
		a.SessionAMBR.Uplink.Equal(b.SessionAMBR.Uplink) && a.SessionAMBR.Downlink.Equal(b.SessionAMBR.Downlink)
}
