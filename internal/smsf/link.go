// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6c"
	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/tgpp"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
)

const (
	smscPeerID        = "smsc"
	productName       = "Ella Core"
	reconcileInterval = 30 * time.Second
	shutdownTimeout   = 5 * time.Second
)

var applications = []diameter.Application{
	{ID: sgd.ApplicationID, VendorID: tgpp.VendorID},
	{ID: s6c.ApplicationID, VendorID: tgpp.VendorID},
}

const PeerRoleSMSC = "smsc"

type Identity struct {
	Host  string
	Realm string
}

type PeerStatus struct {
	Role    string
	Host    string
	Realm   string
	Address netip.AddrPort
	State   diameter.PeerState
	Since   time.Time
}

type NodeSettings struct {
	MCC        string
	MNC        string
	MMEGroupID uint16
	MMECode    uint8
}

type SMSCSource func(ctx context.Context) (netip.AddrPort, error)

type NodeSource func(ctx context.Context) (NodeSettings, error)

type linkConfig struct {
	identity diameter.Identity
	peer     diameter.Peer
}

func (a linkConfig) equal(b linkConfig) bool {
	return a.identity.OriginHost == b.identity.OriginHost &&
		a.identity.OriginRealm == b.identity.OriginRealm &&
		sameAddrs(a.identity.HostIPAddresses, b.identity.HostIPAddresses) &&
		sameAddrs(a.peer.Addresses, b.peer.Addresses) &&
		a.peer.Port == b.peer.Port
}

func sameAddrs(a, b []netip.Addr) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

type Link struct {
	smscSource SMSCSource
	nodeSource NodeSource
	handler    diameter.Handler
	logger     *zap.Logger
	slog       *slog.Logger
	localAddr  func(remote netip.Addr) (netip.Addr, error)

	mu        sync.Mutex
	node      *diameter.Node
	cfg       linkConfig
	smsc      netip.AddrPort
	lastError string
	since     time.Time
}

func NewLink(smscSource SMSCSource, nodeSource NodeSource, handler diameter.Handler, logger *zap.Logger) *Link {
	if handler == nil {
		handler = NewStubHandler()
	}

	return &Link{
		smscSource: smscSource,
		nodeSource: nodeSource,
		handler:    handler,
		logger:     logger,
		slog:       slog.New(zapslog.NewHandler(logger.Core(), zapslog.WithName("diameter"))),
		localAddr:  routeSource,
		since:      time.Now(),
	}
}

func (l *Link) Run(ctx context.Context, wakeup <-chan struct{}) {
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	for {
		l.reconcile(ctx)

		select {
		case <-ctx.Done():
			l.stop()
			return
		case <-wakeup:
		case <-ticker.C:
		}
	}
}

func (l *Link) Node() *diameter.Node {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.node
}

func (l *Link) Identity(ctx context.Context) (Identity, error) {
	settings, err := l.nodeSource(ctx)
	if err != nil {
		return Identity{}, err
	}

	return identityOf(settings)
}

func identityOf(s NodeSettings) (Identity, error) {
	realm, err := DiameterRealm(s.MCC, s.MNC)
	if err != nil {
		return Identity{}, err
	}

	return Identity{Host: MMEHost(realm, s.MMEGroupID, s.MMECode), Realm: realm}, nil
}

func (l *Link) Peers() []PeerStatus {
	l.mu.Lock()
	node, smsc, since := l.node, l.smsc, l.since
	l.mu.Unlock()

	if !smsc.IsValid() {
		return nil
	}

	status := PeerStatus{Role: PeerRoleSMSC, Address: smsc, State: diameter.PeerDown, Since: since}

	if node != nil {
		if peer, ok := node.Peer(smscPeerID); ok {
			status.Host = peer.Host
			status.Realm = peer.Realm
			status.State = peer.State
			status.Since = peer.Since
		}
	}

	return []PeerStatus{status}
}

func (l *Link) reconcile(ctx context.Context) {
	smsc, err := l.smscSource(ctx)
	if err != nil {
		l.fail(fmt.Errorf("read SMS settings: %w", err))
		return
	}

	if !smsc.IsValid() {
		l.disable()
		return
	}

	l.setSMSC(smsc)

	node, err := l.nodeSource(ctx)
	if err != nil {
		l.fail(fmt.Errorf("read node identity: %w", err))
		return
	}

	cfg, err := l.desired(smsc, node)
	if err != nil {
		l.fail(err)
		return
	}

	l.mu.Lock()
	running := l.node != nil && l.cfg.equal(cfg)
	l.mu.Unlock()

	if running {
		return
	}

	l.stop()

	if err := l.start(cfg); err != nil {
		l.fail(err)
	}
}

func (l *Link) desired(smsc netip.AddrPort, node NodeSettings) (linkConfig, error) {
	identity, err := identityOf(node)
	if err != nil {
		return linkConfig{}, err
	}

	remote := smsc.Addr().Unmap()

	local, err := l.localAddr(remote)
	if err != nil {
		return linkConfig{}, fmt.Errorf("find a local address towards the SMSC %s: %w", remote, err)
	}

	return linkConfig{
		identity: diameter.Identity{
			OriginHost:      identity.Host,
			OriginRealm:     identity.Realm,
			HostIPAddresses: []netip.Addr{local},
			ProductName:     productName,
		},
		peer: diameter.Peer{
			ID:           smscPeerID,
			Addresses:    []netip.Addr{remote},
			Port:         smsc.Port(),
			Transport:    diameter.TransportSCTP,
			Applications: applications,
		},
	}, nil
}

func (l *Link) start(cfg linkConfig) error {
	node, err := diameter.New(diameter.Config{
		Identity:          cfg.identity,
		Handler:           l.handler,
		OnPeerStateChange: l.peerStateChanged,
		Logger:            l.slog,
	})
	if err != nil {
		return fmt.Errorf("create Diameter node: %w", err)
	}

	if err := node.SetPeers([]diameter.Peer{cfg.peer}); err != nil {
		_ = node.Shutdown(context.Background())
		return fmt.Errorf("configure SMSC peer: %w", err)
	}

	l.mu.Lock()
	l.node = node
	l.cfg = cfg
	l.lastError = ""
	l.since = time.Now()
	l.mu.Unlock()

	l.logger.Info("Connecting to SMSC",
		zap.String("smsc", netip.AddrPortFrom(cfg.peer.Addresses[0], cfg.peer.Port).String()),
		zap.String("origin_host", cfg.identity.OriginHost),
		zap.String("origin_realm", cfg.identity.OriginRealm),
		zap.Stringer("host_ip_address", cfg.identity.HostIPAddresses[0]),
	)

	return nil
}

func (l *Link) peerStateChanged(s diameter.PeerStatus) {
	fields := []zap.Field{
		zap.Stringer("state", s.State),
		zap.String("peer_host", s.Host),
		zap.String("peer_realm", s.Realm),
	}

	if s.State == diameter.PeerOpen {
		l.logger.Info("SMSC link up", fields...)
		return
	}

	if s.LastError != "" {
		fields = append(fields, zap.String("error", s.LastError))
	}

	l.logger.Warn("SMSC link not up", fields...)
}

func (l *Link) setSMSC(smsc netip.AddrPort) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.smsc != smsc {
		l.smsc = smsc
		l.since = time.Now()
	}
}

func (l *Link) disable() {
	l.mu.Lock()
	wasEnabled := l.smsc.IsValid() || l.node != nil
	l.mu.Unlock()

	l.stop()

	l.mu.Lock()
	l.smsc = netip.AddrPort{}
	l.cfg = linkConfig{}
	l.lastError = ""
	l.since = time.Now()
	l.mu.Unlock()

	if wasEnabled {
		l.logger.Info("SMS disabled; SMSC link closed")
	}
}

func (l *Link) fail(err error) {
	l.stop()

	l.mu.Lock()
	changed := l.lastError != err.Error()
	l.lastError = err.Error()

	if changed {
		l.since = time.Now()
	}
	l.mu.Unlock()

	if changed {
		l.logger.Warn("SMSC link unavailable", zap.Error(err))
	}
}

func (l *Link) stop() {
	l.mu.Lock()
	node := l.node
	l.node = nil
	l.mu.Unlock()

	if node == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := node.Shutdown(ctx); err != nil && !errors.Is(err, diameter.ErrClosed) {
		l.logger.Warn("SMSC link did not close cleanly", zap.Error(err))
	}
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
