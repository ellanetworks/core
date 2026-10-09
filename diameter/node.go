// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultWatchdogInterval      = 30 * time.Second
	DefaultHandshakeTimeout      = 10 * time.Second
	DefaultReconnectInterval     = time.Second
	DefaultMaxReconnectInterval  = 30 * time.Second
	DefaultRequestTimeout        = 30 * time.Second
	DefaultMaxConcurrentRequests = 256
	DefaultMaxPendingConnections = 64
	DefaultMaxDuplicateEntries   = 16384
)

const minWatchdogInterval = 6 * time.Second

var (
	ErrClosed                 = errors.New("diameter: node closed")
	ErrNotConnected           = errors.New("diameter: peer not connected")
	ErrUnknownPeer            = errors.New("diameter: unknown peer")
	ErrApplicationUnsupported = errors.New("diameter: application not negotiated with peer")
	ErrUnableToDeliver        = errors.New("diameter: no peer can deliver the request")
)

type Identity struct {
	OriginHost      string
	OriginRealm     string
	HostIPAddresses []netip.Addr
	VendorID        uint32
	ProductName     string
}

type Application struct {
	ID       uint32
	VendorID uint32
}

type Handler interface {
	ServeDiameter(ctx context.Context, c *Conn, req *Message) *Message
}

type HandlerFunc func(ctx context.Context, c *Conn, req *Message) *Message

func (f HandlerFunc) ServeDiameter(ctx context.Context, c *Conn, req *Message) *Message {
	return f(ctx, c, req)
}

type Config struct {
	Identity                    Identity
	Handler                     Handler
	AcceptUnknownPeers          bool
	UnknownPeerApplications     []Application
	UnknownPeerSupportedVendors []uint32
	ServedRealms                []string
	OriginStateID               uint32
	WatchdogInterval            time.Duration
	HandshakeTimeout            time.Duration
	ReconnectInterval           time.Duration
	MaxReconnectInterval        time.Duration
	RequestTimeout              time.Duration
	HandlerTimeout              time.Duration
	MaxConcurrentRequests       int
	MaxPendingConnections       int
	MaxDuplicateEntries         int
	OnPeerStateChange           func(PeerStatus)
	OnReroute                   func(Reroute)
	Resolver                    Resolver
	Logger                      *slog.Logger

	watchdogJitter    time.Duration
	allowFastWatchdog bool
	noWatchdogJitter  bool
}

type Peer struct {
	ID               string
	Host             string
	Addresses        []netip.Addr
	Transports       []Transport
	Applications     []Application
	SupportedVendors []uint32
	Routes           []Route
	Dial             *Dial
}

type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

type Dial struct {
	Port uint16
}

type Node struct {
	cfg        Config
	logger     *slog.Logger
	baseCtx    context.Context
	baseCancel context.CancelFunc

	mu           sync.Mutex
	closed       bool
	byID         map[string]*peer
	byHost       map[string]*peer
	routes       map[routeKey]*route
	cache        map[cacheKey]*cacheEntry
	pending      int
	conns        map[*Conn]struct{}
	listeners    map[Listener]struct{}
	peersChanged chan struct{}
	reports      []PeerStatus
	reporting    bool

	inflight   sync.WaitGroup
	goroutines sync.WaitGroup
	duplicates duplicateCache

	sessionHigh uint32
	sessionLow  atomic.Uint32
	endToEnd    atomic.Uint32
}

func New(cfg Config) (*Node, error) {
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	n := &Node{
		cfg:          cfg,
		logger:       cfg.Logger,
		baseCtx:      ctx,
		baseCancel:   cancel,
		byID:         make(map[string]*peer),
		byHost:       make(map[string]*peer),
		routes:       make(map[routeKey]*route),
		cache:        make(map[cacheKey]*cacheEntry),
		conns:        make(map[*Conn]struct{}),
		listeners:    make(map[Listener]struct{}),
		peersChanged: make(chan struct{}),
		duplicates:   duplicateCache{max: cfg.MaxDuplicateEntries},
		sessionHigh:  uint32(time.Now().Unix() + ntpUnixOffset),
	}

	n.endToEnd.Store(uint32(time.Now().Unix()&0xfff)<<20 | randomUint32()&0xfffff)

	return n, nil
}

func validateConfig(cfg *Config) error {
	id := &cfg.Identity

	switch {
	case id.OriginHost == "":
		return errors.New("diameter: Identity.OriginHost is required")
	case id.OriginRealm == "":
		return errors.New("diameter: Identity.OriginRealm is required")
	case len(id.HostIPAddresses) == 0:
		return errors.New("diameter: at least one Identity.HostIPAddresses entry is required")
	case id.ProductName == "":
		return errors.New("diameter: Identity.ProductName is required")
	case cfg.Handler == nil:
		return errors.New("diameter: Handler is required")
	case cfg.AcceptUnknownPeers && len(cfg.UnknownPeerApplications) == 0:
		return errors.New("diameter: AcceptUnknownPeers needs UnknownPeerApplications")
	}

	for _, a := range id.HostIPAddresses {
		if !a.IsValid() || a.IsUnspecified() {
			return fmt.Errorf("diameter: invalid Host-IP-Address %v", a)
		}
	}

	id.HostIPAddresses = slices.Clone(id.HostIPAddresses)
	cfg.UnknownPeerApplications = slices.Clone(cfg.UnknownPeerApplications)
	cfg.UnknownPeerSupportedVendors = slices.Clone(cfg.UnknownPeerSupportedVendors)
	cfg.ServedRealms = slices.Clone(cfg.ServedRealms)

	durations := []struct {
		name  string
		value *time.Duration
		def   time.Duration
	}{
		{"WatchdogInterval", &cfg.WatchdogInterval, DefaultWatchdogInterval},
		{"HandshakeTimeout", &cfg.HandshakeTimeout, DefaultHandshakeTimeout},
		{"ReconnectInterval", &cfg.ReconnectInterval, DefaultReconnectInterval},
		{"MaxReconnectInterval", &cfg.MaxReconnectInterval, DefaultMaxReconnectInterval},
		{"RequestTimeout", &cfg.RequestTimeout, DefaultRequestTimeout},
	}

	for _, d := range durations {
		switch {
		case *d.value < 0:
			return fmt.Errorf("diameter: %s must not be negative", d.name)
		case *d.value == 0:
			*d.value = d.def
		}
	}

	if cfg.HandlerTimeout < 0 {
		return errors.New("diameter: HandlerTimeout must not be negative")
	}

	if cfg.WatchdogInterval < minWatchdogInterval && !cfg.allowFastWatchdog {
		return fmt.Errorf("diameter: WatchdogInterval %s is below the %s minimum", cfg.WatchdogInterval, minWatchdogInterval)
	}

	if cfg.MaxReconnectInterval < cfg.ReconnectInterval {
		cfg.MaxReconnectInterval = cfg.ReconnectInterval
	}

	for _, v := range []*int{&cfg.MaxConcurrentRequests, &cfg.MaxPendingConnections, &cfg.MaxDuplicateEntries} {
		if *v < 0 {
			return errors.New("diameter: limits must not be negative")
		}
	}

	if cfg.MaxConcurrentRequests == 0 {
		cfg.MaxConcurrentRequests = DefaultMaxConcurrentRequests
	}

	if cfg.MaxPendingConnections == 0 {
		cfg.MaxPendingConnections = DefaultMaxPendingConnections
	}

	if cfg.MaxDuplicateEntries == 0 {
		cfg.MaxDuplicateEntries = DefaultMaxDuplicateEntries
	}

	if cfg.watchdogJitter == 0 && !cfg.noWatchdogJitter {
		cfg.watchdogJitter = 2 * time.Second
	}

	if cfg.OriginStateID == 0 {
		cfg.OriginStateID = uint32(time.Now().Unix())
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.Resolver == nil {
		cfg.Resolver = net.DefaultResolver
	}

	return nil
}

func (n *Node) Identity() Identity {
	id := n.cfg.Identity
	id.HostIPAddresses = slices.Clone(id.HostIPAddresses)

	return id
}

func (n *Node) NewSessionID() string {
	return n.cfg.Identity.OriginHost + ";" + strconv.FormatUint(uint64(n.sessionHigh), 10) + ";" +
		strconv.FormatUint(uint64(n.sessionLow.Add(1)), 10)
}

func (n *Node) nextEndToEnd() uint32 {
	return n.endToEnd.Add(1)
}

func (n *Node) Serve(ln Listener) error {
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()

		_ = ln.Close()

		return ErrClosed
	}

	n.listeners[ln] = struct{}{}
	n.goroutines.Add(1)
	n.mu.Unlock()

	defer n.goroutines.Done()

	defer func() {
		n.mu.Lock()
		delete(n.listeners, ln)
		n.mu.Unlock()
	}()

	n.logger.Info("Diameter listener started", slog.String("address", ln.Addr().String()))

	var backoff time.Duration

	for {
		t, err := ln.accept()
		if err != nil {
			if n.isClosed() {
				return ErrClosed
			}

			if errors.Is(err, net.ErrClosed) {
				return err
			}

			if errors.Is(err, errConnectionSetup) {
				n.logger.Debug("Diameter connection lost during setup", slog.Any("error", err))
				continue
			}

			backoff = min(max(2*backoff, 5*time.Millisecond), time.Second)
			n.logger.Warn("Diameter accept failed", slog.Any("error", err), slog.Duration("retry_in", backoff))

			select {
			case <-time.After(backoff):
			case <-n.baseCtx.Done():
				return ErrClosed
			}

			continue
		}

		backoff = 0

		c := newConn(n, t, nil)
		c.pendingSlot = true

		if !n.registerResponder(c) {
			n.logger.Debug("rejecting Diameter connection", slog.String("remote", t.remoteAddr().String()))

			_ = t.abort()

			continue
		}

		c.start()
	}
}

func (n *Node) registerResponder(c *Conn) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed || n.pending >= n.cfg.MaxPendingConnections {
		return false
	}

	n.pending++
	n.conns[c] = struct{}{}
	n.goroutines.Add(1)

	return true
}

func (n *Node) releasePending() {
	n.mu.Lock()
	n.pending--
	n.mu.Unlock()
}

func (n *Node) isClosed() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.closed
}

func (n *Node) SetPeers(peers []Peer) error {
	desired, err := normalizePeers(peers)
	if err != nil {
		return err
	}

	n.mu.Lock()

	if n.closed {
		n.mu.Unlock()
		return ErrClosed
	}

	var (
		removed   []*peer
		displaced []*Conn
		started   []*peer
	)

	for id, p := range n.byID {
		want, ok := desired[id]
		if !ok || !samePeer(*p.cfg, want) {
			removed = append(removed, p)
			n.unregisterLocked(p)

			continue
		}

		p.cfg.Routes = want.Routes
	}

	for id, want := range desired {
		if _, ok := n.byID[id]; ok {
			continue
		}

		p := newConfiguredPeer(want)
		n.byID[id] = p

		if p.host != "" {
			if other, ok := n.byHost[p.host]; ok && (other.cfg == nil || other.dynamic) {
				displaced = append(displaced, other.conns()...)
				n.dropUnknownLocked(other)
			}

			n.byHost[p.host] = p
		}

		for _, other := range n.byHost {
			if other.cfg == nil && other.open != nil && p.matches(other.open.t.remoteAddr(), other.open.t.kind(), other.host) {
				displaced = append(displaced, other.conns()...)
				n.dropUnknownLocked(other)
			}
		}

		if p.cfg.Dial != nil {
			started = append(started, p)
		}
	}

	for _, p := range started {
		n.goroutines.Add(1)

		go n.maintain(p)
	}

	n.rebuildRoutesLocked()
	n.notifyPeersLocked()
	n.mu.Unlock()

	for _, p := range removed {
		n.logger.Info("Diameter peer removed", slog.String("peer", p.id))

		for _, c := range p.conns() {
			go c.disconnect(DisconnectCauseDoNotWantToTalkToYou)
		}
	}

	for _, c := range displaced {
		c.abort("displaced by a configured peer")
	}

	return nil
}

func normalizePeers(peers []Peer) (map[string]Peer, error) {
	desired := make(map[string]Peer, len(peers))
	hosts := make(map[string]string)

	type endpoint struct {
		addr      netip.Addr
		transport Transport
	}

	hostless := make(map[endpoint]string)

	for _, p := range peers {
		if p.ID == "" {
			return nil, errors.New("diameter: every Peer needs an ID")
		}

		if _, dup := desired[p.ID]; dup {
			return nil, fmt.Errorf("diameter: duplicate peer ID %q", p.ID)
		}

		if len(p.Addresses) == 0 {
			return nil, fmt.Errorf("diameter: peer %q needs at least one address", p.ID)
		}

		if len(p.Applications) == 0 {
			return nil, fmt.Errorf("diameter: peer %q needs at least one application", p.ID)
		}

		if len(p.Transports) == 0 {
			return nil, fmt.Errorf("diameter: peer %q needs at least one transport", p.ID)
		}

		for i, kind := range p.Transports {
			if kind != TransportSCTP && kind != TransportTCP {
				return nil, fmt.Errorf("diameter: peer %q has an unknown transport %s", p.ID, kind)
			}

			if slices.Contains(p.Transports[:i], kind) {
				return nil, fmt.Errorf("diameter: peer %q lists the transport %s twice", p.ID, kind)
			}
		}

		if p.Dial != nil {
			d := *p.Dial
			if d.Port == 0 {
				d.Port = DefaultPort
			}

			p.Dial = &d
		}

		p.Transports = slices.Clone(p.Transports)
		p.Addresses = slices.Clone(p.Addresses)
		p.Applications = slices.Clone(p.Applications)
		p.SupportedVendors = slices.Clone(p.SupportedVendors)
		p.Routes = slices.Clone(p.Routes)

		if err := validateRoutes(p); err != nil {
			return nil, err
		}

		for i, a := range p.Addresses {
			if !a.IsValid() || a.IsUnspecified() {
				return nil, fmt.Errorf("diameter: peer %q has an invalid address %v", p.ID, a)
			}

			p.Addresses[i] = a.Unmap()
		}

		if p.Host != "" {
			key := strings.ToLower(p.Host)
			if other, dup := hosts[key]; dup {
				return nil, fmt.Errorf("diameter: peers %q and %q share the host %s", other, p.ID, p.Host)
			}

			hosts[key] = p.ID
		} else {
			for _, a := range p.Addresses {
				for _, kind := range p.Transports {
					ep := endpoint{a, kind}
					if other, dup := hostless[ep]; dup {
						return nil, fmt.Errorf("diameter: peers %q and %q without a host share the address %v over %s", other, p.ID, a, kind)
					}

					hostless[ep] = p.ID
				}
			}
		}

		desired[p.ID] = p
	}

	return desired, nil
}

func samePeer(a, b Peer) bool {
	return strings.EqualFold(a.Host, b.Host) && slices.Equal(a.Transports, b.Transports) && sameDial(a.Dial, b.Dial) &&
		slices.Equal(a.Addresses, b.Addresses) && slices.Equal(a.Applications, b.Applications) &&
		slices.Equal(a.SupportedVendors, b.SupportedVendors)
}

func sameDial(a, b *Dial) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

func (n *Node) Peers() []PeerStatus {
	n.mu.Lock()
	defer n.mu.Unlock()

	statuses := make([]PeerStatus, 0, len(n.byID)+len(n.byHost))

	for _, p := range n.byID {
		statuses = append(statuses, p.statusLocked())
	}

	for _, p := range n.byHost {
		if p.cfg == nil || p.dynamic {
			statuses = append(statuses, p.statusLocked())
		}
	}

	slices.SortFunc(statuses, func(a, b PeerStatus) int {
		if c := strings.Compare(a.ID, b.ID); c != 0 {
			return c
		}

		return strings.Compare(a.Host, b.Host)
	})

	return statuses
}

func (n *Node) Peer(id string) (PeerStatus, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	p, ok := n.byID[id]
	if !ok {
		return PeerStatus{}, false
	}

	return p.statusLocked(), true
}

type RequestOption func(*requestOptions)

type requestOptions struct {
	failFast bool
}

func FailFast() RequestOption {
	return func(o *requestOptions) { o.failFast = true }
}

func applyOptions(opts []RequestOption) requestOptions {
	var o requestOptions
	for _, opt := range opts {
		opt(&o)
	}

	return o
}

func (n *Node) newRequest(req *Message) Message {
	m := *req
	m.Flags = (m.Flags | FlagRequest) &^ (FlagRetransmit | FlagError)
	m.EndToEndID = n.nextEndToEnd()
	m.AVPs = slices.Clone(req.AVPs)

	return m
}

func (n *Node) Do(ctx context.Context, peerID string, req *Message, opts ...RequestOption) (*Message, error) {
	n.mu.Lock()
	p, ok := n.byID[peerID]
	n.mu.Unlock()

	if !ok {
		return nil, ErrUnknownPeer
	}

	o := applyOptions(opts)

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, n.cfg.RequestTimeout)
		defer cancel()
	}

	m := n.newRequest(req)

	for {
		c, err := n.waitAvailable(ctx, p, o.failFast)
		if err != nil {
			return nil, err
		}

		if !c.supports(m.ApplicationID) {
			return nil, ErrApplicationUnsupported
		}

		ans, err := c.exchange(ctx, &m, false)
		if !errors.Is(err, errConnClosed) {
			return ans, err
		}

		m.Flags |= FlagRetransmit
	}
}

func (n *Node) waitAvailable(ctx context.Context, p *peer, failFast bool) (*Conn, error) {
	for {
		n.mu.Lock()

		if n.closed {
			n.mu.Unlock()
			return nil, ErrClosed
		}

		if p.removed {
			n.mu.Unlock()
			return nil, ErrUnknownPeer
		}

		c := p.available()
		if c == nil && p.suppress {
			p.suppress = false
			signal(p.kick)
		}

		canWait := p.cfg != nil && !failFast
		changed := n.peersChanged
		n.mu.Unlock()

		switch {
		case c != nil:
			return c, nil
		case !canWait:
			return nil, ErrNotConnected
		}

		select {
		case <-changed:
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %w", ErrNotConnected, ctx.Err())
		}
	}
}

func (n *Node) notifyPeersLocked() {
	close(n.peersChanged)
	n.peersChanged = make(chan struct{})
}

func (n *Node) routingError(req *Message) uint32 {
	if host, ok := req.Find(AVPDestinationHost, 0); ok {
		if !strings.EqualFold(host.UTF8String(), n.cfg.Identity.OriginHost) {
			return ResultUnableToDeliver
		}

		return 0
	}

	if realm, ok := req.Find(AVPDestinationRealm, 0); ok && !n.servesRealm(realm.UTF8String()) {
		return ResultRealmNotServed
	}

	return 0
}

func (n *Node) servesRealm(realm string) bool {
	if strings.EqualFold(realm, n.cfg.Identity.OriginRealm) {
		return true
	}

	for _, r := range n.cfg.ServedRealms {
		if strings.EqualFold(realm, r) {
			return true
		}
	}

	return false
}

func (n *Node) admit(req *Message) uint32 {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.closed {
		n.inflight.Add(1)
		n.goroutines.Add(1)

		return 0
	}

	if _, ok := req.Find(AVPDestinationHost, 0); ok {
		return ResultTooBusy
	}

	return ResultUnableToDeliver
}

func (n *Node) Shutdown(ctx context.Context) error {
	return n.ShutdownWithCause(ctx, DisconnectCauseRebooting)
}

func (n *Node) ShutdownWithCause(ctx context.Context, cause uint32) error {
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return ErrClosed
	}

	n.closed = true

	for _, e := range n.cache {
		e.timer.Stop()
	}

	listeners := make([]Listener, 0, len(n.listeners))
	for ln := range n.listeners {
		listeners = append(listeners, ln)
	}

	n.notifyPeersLocked()
	n.mu.Unlock()

	for _, ln := range listeners {
		_ = ln.Close()
	}

	drained := make(chan struct{})

	go func() {
		n.inflight.Wait()
		close(drained)
	}()

	select {
	case <-drained:
	case <-ctx.Done():
	}

	var wg sync.WaitGroup

	for _, c := range n.allConns() {
		wg.Go(func() {
			c.disconnect(cause)
		})
	}

	disconnected := make(chan struct{})

	go func() {
		wg.Wait()
		close(disconnected)
	}()

	select {
	case <-disconnected:
	case <-ctx.Done():
	}

	n.baseCancel()

	for _, c := range n.allConns() {
		c.abort("shutdown")
	}

	finished := make(chan struct{})

	go func() {
		n.goroutines.Wait()
		close(finished)
	}()

	select {
	case <-finished:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (n *Node) allConns() []*Conn {
	n.mu.Lock()
	defer n.mu.Unlock()

	conns := make([]*Conn, 0, len(n.conns))
	for c := range n.conns {
		conns = append(conns, c)
	}

	return conns
}
