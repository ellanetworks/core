// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/diameter"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
)

const (
	productName       = "Ella Core"
	reconcileInterval = 30 * time.Second
	shutdownTimeout   = 5 * time.Second
)

type Identity struct {
	Host  string
	Realm string
}

type NodeSettings struct {
	MCC        string
	MNC        string
	MMEGroupID uint16
	MMECode    uint8
}

type PeerConfig struct {
	Role         string
	Address      netip.AddrPort
	Applications []diameter.Application
}

type PeerStatus struct {
	Role    string
	Host    string
	Realm   string
	Address netip.AddrPort
	State   diameter.PeerState
	Since   time.Time
}

type NodeSource func(ctx context.Context) (NodeSettings, error)

type PeersSource func(ctx context.Context) ([]PeerConfig, error)

type Manager struct {
	nodeSource  NodeSource
	peersSource PeersSource
	listen      ListenConfig
	mux         *diameter.Mux
	logger      *zap.Logger
	slog        *slog.Logger
	localAddr   func(remote netip.Addr) (netip.Addr, error)

	mu        sync.Mutex
	node      *diameter.Node
	identity  diameter.Identity
	peers     []PeerConfig
	lastError string
	since     time.Time
	stopping  atomic.Bool
}

func New(nodeSource NodeSource, peersSource PeersSource, listen ListenConfig, logger *zap.Logger) *Manager {
	return &Manager{
		nodeSource:  nodeSource,
		peersSource: peersSource,
		listen:      listen,
		mux:         diameter.NewMux(),
		logger:      logger,
		slog:        slog.New(zapslog.NewHandler(logger.Core(), zapslog.WithName("Diameter"), zapslog.WithCaller(true))),
		localAddr:   routeSource,
		since:       time.Now(),
	}
}

func (m *Manager) Handle(applicationID, commandCode uint32, h diameter.Handler) {
	m.mux.Handle(applicationID, commandCode, h)
}

func (m *Manager) Run(ctx context.Context, wakeup <-chan struct{}) {
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	for {
		m.reconcile(ctx)

		select {
		case <-ctx.Done():
			m.stop()
			return
		case <-wakeup:
		case <-ticker.C:
		}
	}
}

func (m *Manager) Node() *diameter.Node {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.node
}

func (m *Manager) Identity(ctx context.Context) (Identity, error) {
	settings, err := m.nodeSource(ctx)
	if err != nil {
		return Identity{}, err
	}

	return IdentityOf(settings)
}

func IdentityOf(s NodeSettings) (Identity, error) {
	realm, err := DiameterRealm(s.MCC, s.MNC)
	if err != nil {
		return Identity{}, err
	}

	return Identity{Host: MMEHost(realm, s.MMEGroupID, s.MMECode), Realm: realm}, nil
}

func (m *Manager) Peers() []PeerStatus {
	m.mu.Lock()
	node, peers, since := m.node, m.peers, m.since
	m.mu.Unlock()

	statuses := make([]PeerStatus, 0, len(peers))

	for _, p := range peers {
		status := PeerStatus{Role: p.Role, Address: p.Address, State: diameter.PeerDown, Since: since}

		if node != nil {
			if peer, ok := node.Peer(p.Role); ok {
				status.Host = peer.Host
				status.Realm = peer.Realm
				status.State = peer.State
				status.Since = peer.Since
			}
		}

		statuses = append(statuses, status)
	}

	return statuses
}

func (m *Manager) reconcile(ctx context.Context) {
	peers, err := m.peersSource(ctx)
	if err != nil {
		m.fail(fmt.Errorf("read Diameter peers: %w", err))
		return
	}

	m.setPeers(peers)

	if len(peers) == 0 && !m.listen.enabled() {
		m.stopWithCause(diameter.DisconnectCauseDoNotWantToTalkToYou)
		return
	}

	settings, err := m.nodeSource(ctx)
	if err != nil {
		m.fail(fmt.Errorf("read node identity: %w", err))
		return
	}

	identity, err := m.desiredIdentity(settings, peers)
	if err != nil {
		m.fail(err)
		return
	}

	diameterPeers := toDiameterPeers(peers)

	m.mu.Lock()
	node := m.node
	running := node != nil && sameIdentity(m.identity, identity)
	m.mu.Unlock()

	if running {
		if err := node.SetPeers(diameterPeers); err != nil {
			m.fail(fmt.Errorf("configure Diameter peers: %w", err))
		}

		return
	}

	m.stop()

	if err := m.start(ctx, identity, diameterPeers); err != nil {
		m.fail(err)
	}
}

func (m *Manager) desiredIdentity(settings NodeSettings, peers []PeerConfig) (diameter.Identity, error) {
	identity, err := IdentityOf(settings)
	if err != nil {
		return diameter.Identity{}, err
	}

	var addrs []netip.Addr

	for _, p := range peers {
		remote := p.Address.Addr().Unmap()

		local, err := m.localAddr(remote)
		if err != nil {
			return diameter.Identity{}, fmt.Errorf("find a local address towards the %s peer %s: %w", p.Role, remote, err)
		}

		if !slices.Contains(addrs, local) {
			addrs = append(addrs, local)
		}
	}

	if len(addrs) == 0 && m.listen.enabled() {
		listenAddrs, err := m.listenAddrs()
		if err != nil {
			return diameter.Identity{}, err
		}

		if len(listenAddrs) == 0 {
			return diameter.Identity{}, errors.New("the Diameter listen address is unspecified; set a specific address or an interface name to listen without an SMSC")
		}

		addrs = slices.Clone(listenAddrs)
	}

	slices.SortFunc(addrs, func(a, b netip.Addr) int { return a.Compare(b) })

	return diameter.Identity{
		OriginHost:      identity.Host,
		OriginRealm:     identity.Realm,
		HostIPAddresses: addrs,
		ProductName:     productName,
	}, nil
}

func toDiameterPeers(peers []PeerConfig) []diameter.Peer {
	out := make([]diameter.Peer, 0, len(peers))

	for _, p := range peers {
		out = append(out, diameter.Peer{
			ID:           p.Role,
			Addresses:    []netip.Addr{p.Address.Addr().Unmap()},
			Port:         p.Address.Port(),
			Transport:    diameter.TransportSCTP,
			Applications: p.Applications,
		})
	}

	return out
}

func sameIdentity(a, b diameter.Identity) bool {
	return a.OriginHost == b.OriginHost && a.OriginRealm == b.OriginRealm && slices.Equal(a.HostIPAddresses, b.HostIPAddresses)
}

func samePeers(a, b []PeerConfig) bool {
	return slices.EqualFunc(a, b, func(x, y PeerConfig) bool {
		return x.Role == y.Role && x.Address == y.Address && slices.Equal(x.Applications, y.Applications)
	})
}

func (m *Manager) start(ctx context.Context, identity diameter.Identity, peers []diameter.Peer) error {
	node, err := diameter.New(diameter.Config{
		Identity:          identity,
		Handler:           m.mux,
		OnPeerStateChange: m.peerStateChanged,
		Logger:            m.slog,
	})
	if err != nil {
		return fmt.Errorf("create Diameter node: %w", err)
	}

	if err := node.SetPeers(peers); err != nil {
		_ = node.Shutdown(context.Background())
		return fmt.Errorf("configure Diameter peers: %w", err)
	}

	if m.listen.enabled() {
		listeners, err := m.openListeners(ctx)
		if err != nil {
			_ = node.Shutdown(context.Background())
			return err
		}

		for _, ln := range listeners {
			go m.serve(node, ln)
		}
	}

	m.mu.Lock()
	m.node = node
	m.identity = identity
	m.lastError = ""
	m.mu.Unlock()

	m.logger.Info("Diameter node started",
		zap.String("origin_host", identity.OriginHost),
		zap.String("origin_realm", identity.OriginRealm),
		zap.Stringers("host_ip_addresses", identity.HostIPAddresses),
	)

	return nil
}

func (m *Manager) serve(node *diameter.Node, ln diameter.Listener) {
	err := node.Serve(ln)

	_ = ln.Close()

	if err != nil && !errors.Is(err, diameter.ErrClosed) {
		m.logger.Warn("Diameter listener stopped", zap.Stringer("address", ln.Addr()), zap.Error(err))
	}
}

func (m *Manager) peerStateChanged(s diameter.PeerStatus) {
	fields := []zap.Field{
		zap.String("peer", s.ID),
		zap.Stringer("state", s.State),
		zap.String("peer_host", s.Host),
		zap.String("peer_realm", s.Realm),
	}

	switch {
	case s.State == diameter.PeerOpen:
		m.logger.Info("Diameter peer connected", fields...)
		return
	case s.State == diameter.PeerConnecting, s.State == diameter.PeerReopen:
		m.logger.Debug("Diameter peer connecting", fields...)
		return
	case s.State == diameter.PeerClosing:
		m.logger.Debug("Diameter peer closing", fields...)
		return
	case m.stopping.Load():
		m.logger.Info("Diameter peer disconnected", fields...)
		return
	}

	if s.LastError != "" {
		fields = append(fields, zap.String("error", s.LastError))
	}

	m.logger.Warn("Diameter peer not connected", fields...)
}

func (m *Manager) setPeers(peers []PeerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !samePeers(m.peers, peers) {
		m.peers = slices.Clone(peers)
		m.since = time.Now()
	}
}

func (m *Manager) fail(err error) {
	m.stop()

	m.mu.Lock()
	changed := m.lastError != err.Error()
	m.lastError = err.Error()

	if changed {
		m.since = time.Now()
	}
	m.mu.Unlock()

	if changed {
		m.logger.Warn("Diameter node unavailable", zap.Error(err))
	}
}

func (m *Manager) stop() {
	m.stopWithCause(diameter.DisconnectCauseRebooting)
}

func (m *Manager) stopWithCause(cause uint32) {
	m.mu.Lock()
	node := m.node
	m.node = nil
	m.identity = diameter.Identity{}
	m.mu.Unlock()

	if node == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	m.stopping.Store(true)
	defer m.stopping.Store(false)

	if err := node.ShutdownWithCause(ctx, cause); err != nil && !errors.Is(err, diameter.ErrClosed) {
		m.logger.Warn("Diameter node did not shut down cleanly", zap.Error(err))
	}

	m.logger.Info("Diameter node stopped")
}

func routeSource(remote netip.Addr) (netip.Addr, error) {
	conn, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(netip.AddrPortFrom(remote, diameter.DefaultPort)))
	if err != nil {
		return netip.Addr{}, err
	}

	defer func() { _ = conn.Close() }()

	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return netip.Addr{}, fmt.Errorf("unexpected local address %v", conn.LocalAddr())
	}

	return local.AddrPort().Addr().Unmap(), nil
}
