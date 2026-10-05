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
	"time"
)

type PeerState int

const (
	PeerDown PeerState = iota
	PeerConnecting
	PeerOpen
	PeerSuspect
	PeerReopen
	PeerClosing
)

func (s PeerState) String() string {
	switch s {
	case PeerDown:
		return "down"
	case PeerConnecting:
		return "connecting"
	case PeerOpen:
		return "open"
	case PeerSuspect:
		return "suspect"
	case PeerReopen:
		return "reopen"
	case PeerClosing:
		return "closing"
	default:
		return fmt.Sprintf("state(%d)", int(s))
	}
}

type PeerStatus struct {
	ID           string
	Host         string
	Realm        string
	Transport    Transport
	RemoteAddr   netip.Addr
	State        PeerState
	Since        time.Time
	LastError    string
	Applications []Application
	Configured   bool
}

type peer struct {
	id   string
	cfg  *Peer
	host string

	lastHost  string
	lastRealm string
	lastAddr  netip.Addr
	lastError string

	open      *Conn
	initiator *Conn
	parked    *Conn

	everOpen bool
	suppress bool
	removed  bool

	reported PeerState
	since    time.Time

	kick chan struct{}
	stop chan struct{}
}

func newConfiguredPeer(cfg Peer) *peer {
	return &peer{
		id:    cfg.ID,
		cfg:   &cfg,
		host:  strings.ToLower(cfg.Host),
		since: time.Now(),
		kick:  make(chan struct{}, 1),
		stop:  make(chan struct{}),
	}
}

func (p *peer) key() string {
	if p.cfg != nil {
		return "id:" + p.id
	}

	return "host:" + p.host
}

func (p *peer) conns() []*Conn {
	var out []*Conn

	for _, c := range []*Conn{p.open, p.initiator, p.parked} {
		if c != nil {
			out = append(out, c)
		}
	}

	return out
}

func (p *peer) available() *Conn {
	if p.open != nil && p.open.available.Load() {
		return p.open
	}

	return nil
}

func (p *peer) matches(addr netip.Addr, kind Transport, host string) bool {
	if p.cfg == nil || !slices.Contains(p.cfg.Transports, kind) || !slices.Contains(p.cfg.Addresses, addr.Unmap()) {
		return false
	}

	return p.cfg.Host == "" || host == "" || strings.EqualFold(p.cfg.Host, host)
}

func (p *peer) state() PeerState {
	switch {
	case p.open != nil:
		return p.open.peerState()
	case p.initiator != nil || p.parked != nil:
		return PeerConnecting
	default:
		return PeerDown
	}
}

func (p *peer) statusLocked() PeerStatus {
	s := PeerStatus{
		ID:         p.id,
		Host:       p.lastHost,
		Realm:      p.lastRealm,
		RemoteAddr: p.lastAddr,
		State:      p.state(),
		Since:      p.since,
		LastError:  p.lastError,
		Configured: p.cfg != nil,
	}

	if p.cfg != nil && s.Host == "" {
		s.Host = p.cfg.Host
	}

	if c := p.open; c != nil {
		s.Host, s.Realm, s.RemoteAddr, s.Transport = c.peerHost, c.peerRealm, c.t.remoteAddr(), c.t.kind()
		s.Applications = c.commonApplications()
	}

	return s
}

func (n *Node) reportLocked(p *peer) {
	state := p.state()
	if state == p.reported {
		return
	}

	p.reported = state
	p.since = time.Now()

	if n.cfg.OnPeerStateChange == nil {
		return
	}

	n.reports = append(n.reports, p.statusLocked())

	if !n.reporting {
		n.reporting = true
		n.goroutines.Add(1)

		go n.deliverReports()
	}
}

func (n *Node) deliverReports() {
	defer n.goroutines.Done()

	for {
		n.mu.Lock()

		batch := n.reports
		n.reports = nil

		if len(batch) == 0 {
			n.reporting = false
			n.mu.Unlock()

			return
		}
		n.mu.Unlock()

		for _, status := range batch {
			n.cfg.OnPeerStateChange(status)
		}
	}
}

func (n *Node) unregisterLocked(p *peer) {
	delete(n.byID, p.id)

	if p.host != "" && n.byHost[p.host] == p {
		delete(n.byHost, p.host)
	}

	p.removed = true

	if p.stop != nil {
		close(p.stop)
	}
}

func (n *Node) dropUnknownLocked(p *peer) {
	if n.byHost[p.host] == p {
		delete(n.byHost, p.host)
	}

	p.removed = true
}

type cerOutcome int

const (
	cerOpen cerOutcome = iota
	cerParked
	cerReject
	cerDisconnect
)

type cerDecision struct {
	outcome   cerOutcome
	result    uint32
	apps      []Application
	common    map[uint32]bool
	displaced []*Conn
}

func (n *Node) acceptCER(c *Conn, host string, cer *Message) cerDecision {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return cerDecision{outcome: cerReject, result: ResultTooBusy}
	}

	key := strings.ToLower(host)
	remote, kind := c.t.remoteAddr(), c.t.kind()

	p, reject := n.identifyLocked(key, remote, kind)
	if reject {
		return cerDecision{outcome: cerReject, result: ResultUnknownPeer}
	}

	if p == nil {
		return n.acceptUnknownLocked(c, key, cer)
	}

	apps := p.cfg.Applications
	c.localApps = apps

	common := commonApplications(cer.AVPs, apps)
	if len(common) == 0 {
		return cerDecision{outcome: cerReject, result: ResultNoCommonApplication, apps: apps}
	}

	displaced, ok := n.bindHostLocked(p, key)
	if !ok {
		return cerDecision{outcome: cerReject, result: ResultUnknownPeer, apps: apps}
	}

	d := cerDecision{apps: apps, common: common, displaced: displaced}

	if p.open != nil || p.parked != nil {
		d.outcome = cerDisconnect

		return d
	}

	c.peer = p
	c.common = common

	if p.initiator != nil {
		if !winsElection(n.cfg.Identity.OriginHost, host) {
			p.parked = c
			d.outcome = cerParked

			n.reportLocked(p)

			return d
		}

		d.displaced = append(d.displaced, p.initiator)
		p.initiator = nil
	}

	n.openLocked(p, c)

	d.outcome = cerOpen

	return d
}

func winsElection(local, remote string) bool {
	return strings.ToLower(local) > strings.ToLower(remote)
}

func (n *Node) identifyLocked(key string, remote netip.Addr, kind Transport) (*peer, bool) {
	var hostless *peer

	for _, p := range n.byID {
		configuredHost := p.cfg.Host != ""
		learnedHost := p.host != "" && p.host == key

		switch {
		case configuredHost && strings.EqualFold(p.cfg.Host, key):
			return p, !p.matches(remote, kind, key)
		case !configuredHost && learnedHost && !p.matches(remote, kind, key):
			return nil, true
		case !configuredHost && p.matches(remote, kind, key) && (hostless == nil || learnedHost):
			hostless = p
		}
	}

	return hostless, false
}

func (n *Node) acceptUnknownLocked(c *Conn, key string, cer *Message) cerDecision {
	if !n.cfg.AcceptUnknownPeers {
		return cerDecision{outcome: cerReject, result: ResultUnknownPeer}
	}

	apps := n.cfg.UnknownPeerApplications
	c.localApps = apps

	common := commonApplications(cer.AVPs, apps)
	if len(common) == 0 {
		return cerDecision{outcome: cerReject, result: ResultNoCommonApplication, apps: apps}
	}

	if _, exists := n.byHost[key]; exists {
		return cerDecision{outcome: cerDisconnect, apps: apps}
	}

	p := &peer{host: key, since: time.Now()}
	n.byHost[key] = p
	c.peer = p
	c.common = common
	n.openLocked(p, c)

	return cerDecision{outcome: cerOpen, apps: apps, common: common}
}

func (n *Node) bindHostLocked(p *peer, key string) ([]*Conn, bool) {
	if p.host == key && n.byHost[key] == p {
		return nil, true
	}

	var displaced []*Conn

	if other, ok := n.byHost[key]; ok && other != p {
		if other.cfg != nil {
			return nil, false
		}

		displaced = other.conns()
		n.dropUnknownLocked(other)
	}

	if p.host != "" && n.byHost[p.host] == p {
		delete(n.byHost, p.host)
	}

	p.host = key
	n.byHost[key] = p

	return displaced, true
}

func (n *Node) acceptCEA(c *Conn, host string, common map[uint32]bool) ([]*Conn, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	p := c.peer
	if n.closed || p.removed || p.initiator != c || p.open != nil {
		return nil, false
	}

	if p.cfg.Host != "" && !strings.EqualFold(p.cfg.Host, host) {
		p.lastError = fmt.Sprintf("peer answered as %s, expected %s", host, p.cfg.Host)
		return nil, false
	}

	displaced, ok := n.bindHostLocked(p, strings.ToLower(host))
	if !ok {
		p.lastError = fmt.Sprintf("host %s belongs to another configured peer", host)
		return nil, false
	}

	p.initiator = nil
	c.common = common

	if p.parked != nil {
		displaced = append(displaced, p.parked)
		p.parked = nil
	}

	n.openLocked(p, c)

	return displaced, true
}

func (n *Node) openLocked(p *peer, c *Conn) {
	c.reopen = p.everOpen && p.cfg != nil
	p.everOpen = true
	p.open = c
	p.suppress = false
	p.lastError = ""
	p.lastHost, p.lastRealm, p.lastAddr = c.peerHost, c.peerRealm, c.t.remoteAddr()
}

func (n *Node) connDown(c *Conn) {
	var promote *Conn

	n.mu.Lock()

	delete(n.conns, c)

	if p := c.peer; p != nil {
		switch c {
		case p.initiator:
			p.initiator = nil

			if p.parked != nil && !p.removed && !n.closed {
				promote = p.parked
				p.parked = nil
				n.openLocked(p, promote)
			}
		case p.parked:
			p.parked = nil
		case p.open:
			p.open = nil

			if cause := c.dprCause.Load(); cause == int64(DisconnectCauseBusy) || cause == int64(DisconnectCauseDoNotWantToTalkToYou) {
				p.suppress = true
			}

			if p.cfg == nil {
				n.dropUnknownLocked(p)
			}
		}

		if reason := c.errorText(); reason != "" {
			p.lastError = reason
		}

		n.reportLocked(p)
	}

	n.notifyPeersLocked()
	n.mu.Unlock()

	if promote != nil {
		promote.completeParked()
	}
}

func (n *Node) connStateChanged(c *Conn) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if p := c.peer; p != nil && p.open == c {
		n.reportLocked(p)
	}

	n.notifyPeersLocked()
}

func (n *Node) maintain(p *peer) {
	defer n.goroutines.Done()

	var backoff time.Duration

	for {
		n.mu.Lock()
		stopped := p.removed || n.closed
		suppressed := p.open == nil && p.suppress

		var current <-chan struct{}
		if p.open != nil {
			current = p.open.done
		}
		n.mu.Unlock()

		switch {
		case stopped:
			return
		case current != nil:
			if !n.waitPeer(p, current, nil) {
				return
			}

			backoff = n.nextBackoff(backoff)
		case suppressed:
			if !n.waitPeer(p, nil, p.kick) {
				return
			}

			continue
		default:
			if c := n.dial(p); c != nil {
				opened := time.Now()

				if !n.waitPeer(p, c.done, nil) {
					return
				}

				if time.Since(opened) >= n.cfg.MaxReconnectInterval {
					backoff = 0
				}
			}

			backoff = n.nextBackoff(backoff)
		}

		timer := time.NewTimer(backoff)

		select {
		case <-p.stop:
			timer.Stop()
			return
		case <-n.baseCtx.Done():
			timer.Stop()
			return
		case <-p.kick:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (n *Node) waitPeer(p *peer, done <-chan struct{}, kick <-chan struct{}) bool {
	select {
	case <-p.stop:
		return false
	case <-n.baseCtx.Done():
		return false
	case <-done:
		return true
	case <-kick:
		return true
	}
}

func (n *Node) nextBackoff(current time.Duration) time.Duration {
	next := min(max(2*current, n.cfg.ReconnectInterval), n.cfg.MaxReconnectInterval)
	return next + randomJitter(next/5)
}

func (n *Node) dial(p *peer) *Conn {
	t, err := n.connect(p.cfg)
	if err != nil {
		n.mu.Lock()
		p.lastError = err.Error()
		n.mu.Unlock()

		n.logger.Warn("failed to connect to Diameter peer", slog.String("peer", p.id), slog.Any("error", err))

		return nil
	}

	n.mu.Lock()

	if p.open != nil || p.removed || n.closed {
		n.mu.Unlock()

		_ = t.abort()

		return nil
	}

	c := newConn(n, t, p)
	p.initiator = c
	n.conns[c] = struct{}{}
	n.goroutines.Add(1)
	n.reportLocked(p)
	n.mu.Unlock()

	c.start()

	select {
	case <-c.opened:
		return c
	case <-c.done:
		return nil
	}
}

func (n *Node) connect(cfg *Peer) (transport, error) {
	local := n.localAddresses(cfg.Addresses)

	var errs []error

	for _, kind := range cfg.Transports {
		t, err := n.connectOver(kind, local, cfg.Addresses, cfg.Dial.Port)
		if err == nil {
			return t, nil
		}

		errs = append(errs, fmt.Errorf("%s: %w", kind, err))
	}

	return nil, errors.Join(errs...)
}

func (n *Node) connectOver(kind Transport, local, remote []netip.Addr, port uint16) (transport, error) {
	ctx, cancel := context.WithTimeout(n.baseCtx, n.cfg.HandshakeTimeout)
	defer cancel()

	switch kind {
	case TransportSCTP:
		return dialSCTP(ctx, local, remote, port, n.logger)
	case TransportTCP:
		return dialTCP(ctx, local, remote, port)
	default:
		return nil, fmt.Errorf("diameter: unsupported transport %s", kind)
	}
}

func (n *Node) localAddresses(remote []netip.Addr) []netip.Addr {
	var v4, v6 bool

	for _, a := range remote {
		if a.Is4() {
			v4 = true
		} else {
			v6 = true
		}
	}

	var out []netip.Addr

	for _, a := range n.cfg.Identity.HostIPAddresses {
		if (a.Is4() && v4) || (!a.Is4() && v6) {
			out = append(out, a)
		}
	}

	return out
}

func commonApplications(avps []AVP, apps []Application) map[uint32]bool {
	var offered []uint32

	for _, a := range avps {
		switch a.Code {
		case AVPAuthApplicationID, AVPAcctApplicationID:
			if id, err := a.Unsigned32(); err == nil && a.VendorID == 0 {
				offered = append(offered, id)
			}
		case AVPVendorSpecificApplicationID:
			if a.VendorID != 0 {
				continue
			}

			inner, err := a.Grouped()
			if err != nil {
				continue
			}

			for _, code := range []uint32{AVPAuthApplicationID, AVPAcctApplicationID} {
				if ia, ok := Find(inner, code, 0); ok {
					if id, err := ia.Unsigned32(); err == nil {
						offered = append(offered, id)
					}
				}
			}
		}
	}

	common := make(map[uint32]bool)

	for _, id := range offered {
		if id == RelayApplicationID {
			for _, app := range apps {
				common[app.ID] = true
			}

			continue
		}

		for _, app := range apps {
			if app.ID == id {
				common[id] = true
			}
		}
	}

	return common
}
