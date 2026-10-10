// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
)

type RerouteReason int

const (
	RerouteFailover RerouteReason = iota + 1
	RerouteTooBusy
	RerouteUnableToDeliver
	RerouteRedirect
)

func (r RerouteReason) String() string {
	switch r {
	case RerouteFailover:
		return "failover"
	case RerouteTooBusy:
		return "too_busy"
	case RerouteUnableToDeliver:
		return "unable_to_deliver"
	case RerouteRedirect:
		return "redirect"
	default:
		return fmt.Sprintf("reroute(%d)", int(r))
	}
}

type Reroute struct {
	Reason        RerouteReason
	From          string
	ApplicationID uint32
	CommandCode   uint32
}

type candidate struct {
	peer *peer
	host string
}

type sending struct {
	n        *Node
	o        requestOptions
	m        Message
	host     string
	realm    string
	fixed    *candidate
	tried    map[*peer]bool
	held     []*peer
	last     *Message
	redirect *following
}

type following struct {
	from *peer
	app  Application
	r    Redirect
	next int
}

func (n *Node) Send(ctx context.Context, req *Message, opts ...RequestOption) (*Message, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, n.cfg.RequestTimeout)
		defer cancel()
	}

	s := &sending{
		n:     n,
		o:     applyOptions(opts),
		m:     n.newRequest(req),
		host:  strings.ToLower(avpString(req, AVPDestinationHost)),
		realm: strings.ToLower(avpString(req, AVPDestinationRealm)),
		tried: make(map[*peer]bool),
	}

	defer s.release()

	return s.run(ctx)
}

func (s *sending) run(ctx context.Context) (*Message, error) {
	for {
		cand, c, err := s.next(ctx)
		if err != nil {
			if s.last != nil && errors.Is(err, errExhausted) {
				return s.last, nil
			}

			return nil, err
		}

		m := s.m
		if cand.host != "" {
			m.AVPs = withDestinationHost(s.m.AVPs, cand.host)
		}

		ans, err := c.exchange(ctx, &m, s.fixed == nil || s.redirect.remaining())

		switch {
		case errors.Is(err, errConnClosed), errors.Is(err, errConnSuspect):
			if s.redirect.remaining() {
				s.tried[cand.peer] = true
			}

			s.m.Flags |= FlagRetransmit
			s.reroute(RerouteFailover, cand.peer)

			continue
		case err != nil:
			return nil, err
		}

		code, _ := protocolResult(ans)

		switch code {
		case ResultRedirectIndication:
			if s.redirect != nil {
				return ans, nil
			}

			if err := s.follow(ctx, cand.peer, ans); err != nil {
				s.n.logger.Warn("not following a Diameter redirect", slog.String("peer", cand.peer.name()), slog.Any("error", err))
				return ans, nil
			}
		case ResultTooBusy, ResultUnableToDeliver:
			if s.fixed != nil && !s.redirect.remaining() {
				return ans, nil
			}

			reason := RerouteTooBusy
			if code == ResultUnableToDeliver {
				reason = RerouteUnableToDeliver
			}

			s.tried[cand.peer] = true
			s.last = ans
			s.m.Flags |= FlagRetransmit
			s.reroute(reason, cand.peer)
		default:
			return ans, nil
		}
	}
}

var errExhausted = errors.New("diameter: every route candidate answered")

func (s *sending) next(ctx context.Context) (candidate, *Conn, error) {
	n := s.n

	for {
		n.mu.Lock()

		if n.closed {
			n.mu.Unlock()
			return candidate{}, nil, ErrClosed
		}

		if s.redirect.remaining() && s.unusableLocked(s.fixed.peer) {
			n.mu.Unlock()

			if err := s.advance(ctx); err != nil && s.last != nil {
				return candidate{}, nil, errExhausted
			}

			continue
		}

		cands, err := s.candidatesLocked()
		if err != nil {
			n.mu.Unlock()
			return candidate{}, nil, err
		}

		waitable, unsupported := n.cfg.AcceptUnknownPeers, 0

		for _, cand := range cands {
			p := cand.peer

			c := p.available()
			if c == nil {
				if p.suppress {
					p.suppress = false
					signal(p.kick)
				}

				waitable = waitable || p.cfg != nil

				continue
			}

			if !c.supports(s.m.ApplicationID) {
				unsupported++
				continue
			}

			n.mu.Unlock()

			return cand, c, nil
		}

		changed := n.peersChanged
		n.mu.Unlock()

		switch {
		case len(cands) > 0 && unsupported == len(cands):
			return candidate{}, nil, ErrApplicationUnsupported
		case !waitable || (s.o.failFast && s.redirect == nil):
			return candidate{}, nil, ErrNotConnected
		}

		select {
		case <-changed:
		case <-ctx.Done():
			return candidate{}, nil, fmt.Errorf("%w: %w", ErrNotConnected, ctx.Err())
		}
	}
}

func (s *sending) candidatesLocked() ([]candidate, error) {
	n := s.n

	if s.fixed != nil {
		if s.fixed.peer.removed {
			return nil, fmt.Errorf("%w: %s is gone", ErrUnableToDeliver, s.fixed.peer.name())
		}

		return []candidate{*s.fixed}, nil
	}

	if s.host != "" {
		if p, ok := n.byHost[s.host]; ok {
			s.fixed = &candidate{peer: p}
			return []candidate{*s.fixed}, nil
		}
	} else if e, ok := n.cacheLookupLocked(&s.m); ok {
		s.hold(e.target)
		s.fixed = &candidate{peer: e.target, host: e.host}

		return []candidate{*s.fixed}, nil
	}

	key := routeKey{realm: s.realm, app: s.m.ApplicationID}

	entries := n.routeEntriesLocked(key, s.host == "")
	if len(entries) == 0 {
		if s.last != nil {
			return nil, errExhausted
		}

		if s.realm != "" && s.host == "" && n.cfg.AcceptUnknownPeers {
			return nil, nil
		}

		return nil, fmt.Errorf("%w: no route to realm %q for application %d", ErrUnableToDeliver, s.realm, s.m.ApplicationID)
	}

	var cands []candidate

	for _, p := range n.orderedLocked(key, entries) {
		cand := candidate{peer: p}

		if s.host == "" && p.host != "" {
			if e, ok := n.cache[hostCacheKey(p.host)]; ok {
				cand = candidate{peer: e.target, host: e.host}
			}
		}

		if s.tried[cand.peer] || slices.ContainsFunc(cands, func(c candidate) bool { return c.peer == cand.peer }) {
			continue
		}

		if cand.peer != p {
			s.hold(cand.peer)
		}

		cands = append(cands, cand)
	}

	if len(cands) == 0 {
		return nil, errExhausted
	}

	return cands, nil
}

func (s *sending) follow(ctx context.Context, from *peer, ans *Message) error {
	r, err := ParseRedirect(ans)
	if err != nil {
		return err
	}

	n := s.n

	n.mu.Lock()
	app, ok := applicationOf(from, s.m.ApplicationID)
	n.mu.Unlock()

	if !ok {
		return fmt.Errorf("%w: application %d", ErrApplicationUnsupported, s.m.ApplicationID)
	}

	s.redirect = &following{from: from, app: app, r: r}

	if err := s.advance(ctx); err != nil {
		s.redirect = nil
		return err
	}

	s.m.Flags |= FlagRetransmit
	s.reroute(RerouteRedirect, from)

	return nil
}

func (s *sending) advance(ctx context.Context) error {
	f := s.redirect

	var errs []error

	for f.next < len(f.r.Hosts) {
		u := f.r.Hosts[f.next]
		f.next++

		if u.Secure {
			errs = append(errs, fmt.Errorf("%s needs transport security", u))
			continue
		}

		target, err := s.redirectPeer(ctx, u, f.app)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if s.tried[target] {
			continue
		}

		s.fixed = &candidate{peer: target, host: u.Host}
		s.remember(*s.fixed)

		return nil
	}

	return errors.Join(errs...)
}

func (f *following) remaining() bool {
	return f != nil && f.next < len(f.r.Hosts)
}

func (s *sending) unusableLocked(p *peer) bool {
	if s.tried[p] || p.removed {
		return true
	}

	if c := p.available(); c != nil {
		return !c.supports(s.m.ApplicationID)
	}

	return !p.dialing && p.initiator == nil && p.parked == nil
}

func (s *sending) remember(cand candidate) {
	f := s.redirect
	if f.r.Usage == DontCache || f.r.MaxCacheTime <= 0 {
		return
	}

	key, ok := s.redirectCacheKey(f.r.Usage, f.from)
	if !ok {
		return
	}

	s.n.mu.Lock()
	defer s.n.mu.Unlock()

	if !cand.peer.removed {
		s.n.cacheStoreLocked(key, cand.peer, cand.host, f.r.MaxCacheTime)
	}
}

func (s *sending) redirectPeer(ctx context.Context, u URI, app Application) (*peer, error) {
	n := s.n

	n.mu.Lock()
	if p, ok := n.byHost[strings.ToLower(u.Host)]; ok && !p.removed {
		s.hold(p)
		n.mu.Unlock()

		return p, nil
	}
	n.mu.Unlock()

	addrs, err := n.cfg.Resolver.LookupNetIP(ctx, "ip", u.Host)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", u.Host, err)
	}

	addrs = slices.Clone(addrs)
	for i, a := range addrs {
		addrs[i] = a.Unmap()
	}

	addrs = slices.DeleteFunc(addrs, func(a netip.Addr) bool { return !a.IsValid() || a.IsUnspecified() })

	if len(addrs) == 0 {
		return nil, fmt.Errorf("resolve %s: no address", u.Host)
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return nil, ErrClosed
	}

	p := n.dynamicPeerLocked(u, app, addrs)
	s.hold(p)

	return p, nil
}

func (s *sending) redirectCacheKey(usage RedirectHostUsage, from *peer) (cacheKey, bool) {
	if usage == AllHost {
		if from.host == "" {
			return cacheKey{}, false
		}

		return hostCacheKey(from.host), true
	}

	return requestCacheKey(usage, &s.m)
}

func (s *sending) hold(p *peer) {
	if slices.Contains(s.held, p) {
		return
	}

	s.n.acquireLocked(p)
	s.held = append(s.held, p)
}

func (s *sending) release() {
	if len(s.held) == 0 {
		return
	}

	s.n.mu.Lock()
	defer s.n.mu.Unlock()

	for _, p := range s.held {
		s.n.releaseLocked(p)
	}
}

func (s *sending) reroute(reason RerouteReason, from *peer) {
	if s.n.cfg.OnReroute == nil {
		return
	}

	s.n.cfg.OnReroute(Reroute{Reason: reason, From: from.name(), ApplicationID: s.m.ApplicationID, CommandCode: s.m.CommandCode})
}

func applicationOf(p *peer, id uint32) (Application, bool) {
	var apps []Application

	switch {
	case p.open != nil:
		apps = p.open.localApps
	case p.cfg != nil:
		apps = p.cfg.Applications
	}

	for _, a := range apps {
		if a.ID == id {
			return a, true
		}
	}

	return Application{}, false
}

func withDestinationHost(avps []AVP, host string) []AVP {
	out := make([]AVP, 0, len(avps)+1)
	set := false

	for _, a := range avps {
		if a.Code == AVPDestinationHost && a.VendorID == 0 {
			if !set {
				out = append(out, UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, host))
				set = true
			}

			continue
		}

		if a.Code == AVPDestinationRealm && a.VendorID == 0 && !set {
			out = append(out, UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, host))
			set = true
		}

		out = append(out, a)
	}

	if !set {
		out = append(out, UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, host))
	}

	return out
}
