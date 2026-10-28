// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/rx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/ellanetworks/core/internal/smf"
	"go.uber.org/zap"
)

const (
	supportedRxFeatures = rx.FeatureRel8 | rx.FeatureMCPTT
	rxRedirectCacheTime = 24 * time.Hour
)

var (
	ErrDiameterUnavailable = errors.New("the Diameter node is not running")
	ErrNoOwner             = errors.New("no node holds the address")
	errRxSessionUnknown    = errors.New("no such Rx session")
)

type rxSession struct {
	id         string
	ref        string
	host       string
	realm      string
	aborted    bool
	actions    []rx.SpecificAction
	service    serviceContext
	components map[uint32]rx.MediaComponent
	rules      map[flowKey]smf.PCCRule
}

func (p *PCF) registerRx(d Diameter) {
	d.Handle(rx.ApplicationID, rx.CommandAA, diameternode.Handler(p.AA))
	d.Handle(rx.ApplicationID, rx.CommandSessionTermination, diameternode.Handler(p.SessionTermination))
}

func (p *PCF) AA(ctx context.Context, id diameter.Identity, req *diameter.Message) *diameter.Message {
	r, err := rx.ParseAARequest(req)
	if err != nil {
		return rx.NewErrorAnswer(req, id, err, 0)
	}

	features := r.Features & supportedRxFeatures

	switch {
	case rx.RequiresUnsupportedFeatures(req, supportedRxFeatures):
		return rx.NewAnswer(req, id, tgpp.Experimental(tgpp.ResultErrorFeatureUnsupported), 0)
	case r.RequestType != nil && *r.RequestType == rx.RequestPCSCFRestoration:
		return rx.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnableToComply}, features)
	}

	sessionID := sessionIDOf(req)

	switch err := p.updateRx(sessionID, r); {
	case err == nil:
		return p.aaAnswer(req, id, features)
	case errors.Is(err, errFilterRestrictions):
		p.log.Info("Rx media refused", zap.String("session", sessionID), zap.Error(err))
		return rx.NewAnswer(req, id, tgpp.Experimental(tgpp.ResultFilterRestrictions), features)
	case r.RequestType != nil && *r.RequestType == rx.RequestUpdate:
		return rx.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnknownSessionID}, features)
	}

	af := tgpp.ParseEnvelope(req).Origin
	s := &rxSession{id: sessionID, host: af.OriginHost, realm: af.OriginRealm, actions: r.SpecificActions, components: make(map[uint32]rx.MediaComponent), rules: make(map[flowKey]smf.PCCRule)}

	p.mu.Lock()
	err = p.applyServiceInfoLocked(s, r)
	p.mu.Unlock()

	if err != nil {
		p.log.Info("Rx media refused", zap.String("session", sessionID), zap.Error(err))
		return rx.NewAnswer(req, id, tgpp.Experimental(tgpp.ResultFilterRestrictions), features)
	}

	ue := r.FramedIPAddress
	if !ue.IsValid() {
		ue = r.FramedIPv6Address
	}

	if !p.bindRx(s, ue, r.CalledStationID) {
		return p.notHosted(ctx, req, id, ue, features)
	}

	p.log.Debug("Rx session opened", zap.String("session", sessionID), zap.Stringer("ue", ue), zap.String("af", af.OriginHost))

	return p.aaAnswer(req, id, features)
}

func (p *PCF) notHosted(ctx context.Context, req *diameter.Message, id diameter.Identity, ue netip.Addr, features rx.Features) *diameter.Message {
	sessionID := sessionIDOf(req)

	p.mu.Lock()
	owners := p.owners
	p.mu.Unlock()

	if _, addressed := req.Find(diameter.AVPDestinationHost, 0); !addressed && owners != nil && ue.IsValid() {
		owner, err := owners.Owner(ctx, addressKey(ue))

		switch {
		case err == nil && !owner.Local:
			ans, err := diameter.NewRedirectAnswer(req, id, diameter.Redirect{
				Hosts:        []diameter.URI{owner.URI},
				Usage:        diameter.AllSession,
				MaxCacheTime: rxRedirectCacheTime,
			})
			if err == nil {
				p.log.Debug("Rx session redirected to the node of the UE", zap.String("session", sessionID), zap.Stringer("ue", ue),
					zap.String("node", owner.URI.Host))

				return ans
			}

			p.log.Warn("Rx redirect not built", zap.String("session", sessionID), zap.Error(err))
		case err != nil && !errors.Is(err, ErrNoOwner):
			p.log.Warn("Rx session owner unknown", zap.String("session", sessionID), zap.Stringer("ue", ue), zap.Error(err))
		}
	}

	p.log.Debug("Rx session binding failed", zap.String("session", sessionID), zap.Stringer("ue", ue))

	return rx.NewAnswer(req, id, tgpp.Experimental(tgpp.ResultIPCANSessionNotAvailable), features)
}

func (p *PCF) SessionTermination(ctx context.Context, id diameter.Identity, req *diameter.Message) *diameter.Message {
	if _, err := rx.ParseSessionTerminationRequest(req); err != nil {
		return rx.NewErrorAnswer(req, id, err, 0)
	}

	sessionID := sessionIDOf(req)

	p.mu.Lock()
	s, ok := p.rxSessions[sessionID]
	p.mu.Unlock()

	if !ok {
		return rx.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnknownSessionID}, 0)
	}

	p.forgetRx(s)

	p.log.Debug("Rx session terminated", zap.String("session", sessionID))

	ans, err := rx.NewSessionTerminationAnswer(req, id, rx.SessionTerminationAnswer{Result: tgpp.Result{Code: diameter.ResultSuccess}})
	if err != nil {
		return rx.NewErrorAnswer(req, id, err, 0)
	}

	return ans
}

func (p *PCF) bindRx(s *rxSession, ue netip.Addr, apn string) bool {
	p.mu.Lock()

	a := p.boundIMSAssociationLocked(ue, apn)
	if a == nil {
		p.mu.Unlock()
		return false
	}

	s.ref = a.ref
	p.rxSessions[s.id] = s
	a.rxSessions[s.id] = s
	d := p.refreshRulesLocked(a)
	p.mu.Unlock()

	p.notify(a.ref, d)

	return true
}

func (p *PCF) updateRx(id string, r rx.AARequest) error {
	p.mu.Lock()

	s, ok := p.rxSessions[id]
	if !ok || s.aborted {
		p.mu.Unlock()
		return errRxSessionUnknown
	}

	if err := p.applyServiceInfoLocked(s, r); err != nil {
		p.mu.Unlock()
		return err
	}

	var d *smf.PolicyDecision
	if a, ok := p.associations[s.ref]; ok {
		d = p.refreshRulesLocked(a)
	}
	p.mu.Unlock()

	p.notify(s.ref, d)

	return nil
}

func (p *PCF) applyServiceInfoLocked(s *rxSession, r rx.AARequest) error {
	merged := make([]rx.MediaComponent, 0, len(r.MediaComponents))
	for _, c := range r.MediaComponents {
		merged = append(merged, overlayComponent(s.components[c.Number], c))
	}

	svc := s.service.updated(r)

	if _, err := mediaRules(s.id, svc, merged); err != nil {
		return err
	}

	serviceChanged := svc != s.service
	s.service = svc

	for _, c := range merged {
		s.components[c.Number] = c
	}

	if r.ServiceInfoStatus != rx.ServiceInfoFinal {
		return nil
	}

	for n, c := range s.components {
		if !slices.ContainsFunc(merged, func(m rx.MediaComponent) bool { return m.Number == n }) && (!s.hasRulesFor(n) || serviceChanged) {
			merged = append(merged, c)
		}
	}

	if len(merged) == 0 {
		return nil
	}

	p.log.Debug("Rx media components", zap.String("session", s.id), zap.Strings("components", describeComponents(merged)))

	rules, err := mediaRules(s.id, svc, merged)
	if err != nil {
		return err
	}

	forked := r.SIPForkingIndication == rx.ForkingSeveralDialogues

	for k, prev := range s.rules {
		if !slices.ContainsFunc(merged, func(c rx.MediaComponent) bool { return c.Number == k.component }) {
			continue
		}

		if next, ok := rules[k]; ok && forked {
			rules[k] = forkedRule(prev, next)
		} else if !ok && forked {
			rules[k] = prev
		}
	}

	rules = p.versionedLocked(s.rules, rules)

	maps.DeleteFunc(s.rules, func(k flowKey, _ smf.PCCRule) bool {
		return slices.ContainsFunc(merged, func(c rx.MediaComponent) bool { return c.Number == k.component })
	})
	maps.Copy(s.rules, rules)

	return nil
}

func (s *rxSession) hasRulesFor(component uint32) bool {
	for k := range s.rules {
		if k.component == component {
			return true
		}
	}

	return false
}

func (p *PCF) versionedLocked(current, next map[flowKey]smf.PCCRule) map[flowKey]smf.PCCRule {
	for n, r := range next {
		if old, ok := current[n]; ok {
			r.Version = old.Version
			if old.Equal(r) {
				next[n] = r
				continue
			}
		}

		r.Version = p.nextRevisionLocked()
		next[n] = r
	}

	return next
}

func (p *PCF) forgetRx(s *rxSession) {
	p.mu.Lock()

	if p.rxSessions[s.id] == s {
		delete(p.rxSessions, s.id)
	}

	var d *smf.PolicyDecision

	if a, ok := p.associations[s.ref]; ok && a.rxSessions[s.id] == s {
		delete(a.rxSessions, s.id)
		d = p.refreshRulesLocked(a)
	}
	p.mu.Unlock()

	p.notify(s.ref, d)
}

func (p *PCF) abort(s *rxSession, cause rx.AbortCause) {
	time.AfterFunc(p.abortedRetention, func() { p.forgetRx(s) })

	ctx, cancel := context.WithTimeout(context.Background(), p.abortTimeout)
	defer cancel()

	err := p.send(ctx, s, func(env tgpp.Envelope) (*diameter.Message, error) {
		return rx.NewAbortSessionRequest(env, rx.AbortSessionRequest{Cause: cause})
	})
	if err != nil {
		p.log.Warn("Rx session abort failed", zap.String("session", s.id), zap.String("af", s.host), zap.Error(err))
		p.forgetRx(s)

		return
	}

	p.log.Info("Rx session aborted", zap.String("session", s.id), zap.String("af", s.host), zap.Stringer("cause", cause))
}

func (p *PCF) reportFailedFlows(s *rxSession, action rx.SpecificAction, flows []rx.Flows) {
	ctx, cancel := context.WithTimeout(context.Background(), p.abortTimeout)
	defer cancel()

	err := p.send(ctx, s, func(env tgpp.Envelope) (*diameter.Message, error) {
		return rx.NewReAuthRequest(env, rx.ReAuthRequest{
			SpecificActions: []rx.SpecificAction{action},
			Flows:           flows,
		})
	})
	if err != nil {
		p.log.Warn("Rx failed resources report not delivered", zap.String("session", s.id), zap.String("af", s.host), zap.Error(err))
	}
}

func (p *PCF) send(ctx context.Context, s *rxSession, build func(tgpp.Envelope) (*diameter.Message, error)) error {
	node := p.node()
	if node == nil {
		return ErrDiameterUnavailable
	}

	req, err := build(tgpp.Envelope{
		SessionID:        s.id,
		Origin:           node.Identity(),
		DestinationHost:  s.host,
		DestinationRealm: s.realm,
	})
	if err != nil {
		return err
	}

	ans, err := node.Send(ctx, req)
	if err != nil {
		return err
	}

	r, err := tgpp.ParseResult(ans)
	if err != nil {
		return err
	}

	if r.Failure() {
		return &rx.ResultError{Result: r}
	}

	return nil
}

func (p *PCF) aaAnswer(req *diameter.Message, id diameter.Identity, features rx.Features) *diameter.Message {
	ans, err := rx.NewAAAnswer(req, id, rx.AAAnswer{Result: tgpp.Result{Code: diameter.ResultSuccess}, Features: features})
	if err != nil {
		return rx.NewErrorAnswer(req, id, err, 0)
	}

	return ans
}

func sessionIDOf(req *diameter.Message) string {
	a, _ := req.Find(diameter.AVPSessionID, 0)
	return a.UTF8String()
}

func describeComponents(components []rx.MediaComponent) []string {
	out := make([]string, 0, len(components))

	for _, c := range components {
		d := fmt.Sprintf("%d", c.Number)
		if c.Type != nil {
			d += fmt.Sprintf(" type=%d", *c.Type)
		}

		if c.FlowStatus != nil {
			d += fmt.Sprintf(" status=%d", *c.FlowStatus)
		}

		for _, sub := range c.SubComponents {
			d += fmt.Sprintf(" [%d flows=%d", sub.FlowNumber, len(sub.FlowDescriptions))
			if sub.FlowStatus != nil {
				d += fmt.Sprintf(" status=%d", *sub.FlowStatus)
			}

			if sub.FlowUsage != nil {
				d += fmt.Sprintf(" usage=%d", *sub.FlowUsage)
			}

			d += "]"
		}

		out = append(out, d)
	}

	return out
}
