// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"go.uber.org/zap"
)

type PartnersSource func(ctx context.Context) ([]PartnerConfig, error)

type IdentitySource func(ctx context.Context) (*IdentityConfig, error)

type OperatorSource func(ctx context.Context) (wire.PlmnID, error)

type ClusterSource func(ctx context.Context) ([]int, error)

type Config struct {
	Partners  PartnersSource
	Identity  IdentitySource
	Operator  OperatorSource
	Provision func(ctx context.Context) error
	Renew     func(ctx context.Context, seppID int) error
	Cluster   ClusterSource
	N32       config.N32Interface
}

type Tuning struct {
	Reconcile        time.Duration
	HandshakeTimeout time.Duration
	RenewBefore      time.Duration
	RequestTimeout   time.Duration
}

const (
	expiryWarning = 30 * 24 * time.Hour
	renewRetry    = time.Hour
	pingIdle      = 30 * time.Second
	pingTimeout   = 10 * time.Second
)

func DefaultTuning() Tuning {
	return Tuning{
		Reconcile:        10 * time.Second,
		HandshakeTimeout: 5 * time.Second,
		RenewBefore:      90 * 24 * time.Hour,
		RequestTimeout:   4 * time.Second,
	}
}

type ListenerStatus struct {
	Configured bool   `json:"n32Configured"`
	Address    string `json:"address,omitempty"`
	Port       int    `json:"port,omitempty"`
	Listening  bool   `json:"listening"`
	Error      string `json:"error,omitempty"`
}

type NodeStatus struct {
	SEPPID   int                      `json:"seppID,omitempty"`
	FQDN     string                   `json:"fqdn,omitempty"`
	Listener ListenerStatus           `json:"listener"`
	Contexts map[string]ContextStatus `json:"contexts"`
}

type snapshot struct {
	op             wire.PlmnID
	identity       *localIdentity
	problem        string
	partners       map[string]*partner
	trusted        []*trustedPartner
	clusterSEPPIDs map[int]struct{}
}

func (s *snapshot) usable() bool {
	return s != nil && s.problem == "" && s.identity != nil
}

type Manager struct {
	cfg    Config
	logger *zap.Logger
	tuning Tuning
	now    func() time.Time
	dial   func(ctx context.Context, network, addr string) (net.Conn, error)

	snap atomic.Pointer[snapshot]

	listenMu  sync.Mutex
	server    *http.Server
	listener  net.Listener
	listenErr string
	conns     sync.Map

	mu       sync.Mutex
	root     context.Context
	contexts map[string]*n32cContext

	transportsMu sync.Mutex
	transports   map[string]*n32fTransport

	callbacks atomic.Pointer[http.Handler]

	reconcileMu  sync.Mutex
	rejected     map[string]string
	expiryLog    time.Time
	renewAttempt time.Time
}

type Option func(*Manager)

func WithTuning(t Tuning) Option {
	return func(m *Manager) { m.tuning = t }
}

func WithClock(now func() time.Time) Option {
	return func(m *Manager) { m.now = now }
}

func New(cfg Config, logger *zap.Logger, opts ...Option) *Manager {
	m := &Manager{
		cfg:        cfg,
		logger:     logger,
		tuning:     DefaultTuning(),
		now:        time.Now,
		contexts:   map[string]*n32cContext{},
		transports: map[string]*n32fTransport{},
		rejected:   map[string]string{},
	}

	for _, o := range opts {
		o(m)
	}

	m.snap.Store(&snapshot{problem: "not reconciled yet", partners: map[string]*partner{}})

	return m
}

func (m *Manager) Run(ctx context.Context, wakeup <-chan struct{}) {
	m.mu.Lock()
	m.root = ctx
	m.mu.Unlock()

	ticker := time.NewTicker(m.tuning.Reconcile)
	defer ticker.Stop()

	for {
		m.reconcile(ctx)

		select {
		case <-ctx.Done():
			m.shutdown()
			return
		case <-wakeup:
		case <-ticker.C:
		}
	}
}

func (m *Manager) Reconcile(ctx context.Context) {
	m.reconcile(ctx)
}

func (m *Manager) shutdown() {
	m.stopListener()

	m.mu.Lock()
	for _, c := range m.contexts {
		c.stop()
	}

	m.root = nil
	m.mu.Unlock()

	m.transportsMu.Lock()
	for id, t := range m.transports {
		t.close()
		delete(m.transports, id)
	}
	m.transportsMu.Unlock()
}

func (m *Manager) reconcile(ctx context.Context) {
	m.reconcileMu.Lock()
	defer m.reconcileMu.Unlock()

	next, err := m.load(ctx)
	if err != nil {
		m.logger.Warn("SEPP reconcile failed", zap.Error(err))
		return
	}

	if next.identity == nil && next.problem == errNoIdentity && m.cfg.Provision != nil {
		if err := m.cfg.Provision(ctx); err != nil {
			m.logger.Warn("Could not generate this node's SEPP certificate", zap.Error(err))
		} else if again, err := m.load(ctx); err == nil {
			next = again
		}
	}

	if m.renew(ctx, next) {
		if again, err := m.load(ctx); err == nil {
			next = again
		}
	}

	prev := m.snap.Swap(next)

	if prev.problem != next.problem && next.problem != "" {
		if next.problem == errNoN32 && len(next.partners) == 0 && len(m.rejected) == 0 {
			m.logger.Debug("SEPP is not operational on this node", zap.String("reason", next.problem))
		} else {
			m.logger.Warn("SEPP is not operational on this node", zap.String("reason", next.problem))
		}
	}

	m.checkExpiry(next)
	m.syncTransports(prev, next)
	m.syncListener(ctx)

	m.mu.Lock()
	if m.root != nil {
		m.syncContextsLocked(prev, next)
	}
	m.mu.Unlock()
}

const (
	errNoIdentity = "no SEPP certificate for this node"
	errNoN32      = "interfaces.n32 is not configured on this node"
)

func (m *Manager) load(ctx context.Context) (*snapshot, error) {
	configs, err := m.cfg.Partners(ctx)
	if err != nil {
		return nil, fmt.Errorf("read roaming partners: %w", err)
	}

	snap := &snapshot{partners: make(map[string]*partner, len(configs)), clusterSEPPIDs: m.loadCluster(ctx)}
	rejected := map[string]string{}

	for _, cfg := range configs {
		p, err := newPartner(cfg)
		if err != nil {
			rejected[cfg.ID] = err.Error()

			if m.rejected[cfg.ID] != err.Error() {
				m.logger.Warn("Ignoring a roaming partner with an unusable SEPP configuration", zap.String("partner", cfg.Name), zap.Error(err))
			}

			continue
		}

		snap.partners[p.id] = p
		snap.trusted = append(snap.trusted, p.trust)
	}

	m.rejected = rejected

	if !m.cfg.N32.Enabled {
		snap.problem = errNoN32
		return snap, nil
	}

	op, err := m.cfg.Operator(ctx)
	if err != nil {
		snap.problem = "the operator PLMN is not configured: " + err.Error()
		return snap, nil
	}

	snap.op = op

	idCfg, err := m.cfg.Identity(ctx)
	if err != nil {
		return nil, fmt.Errorf("read SEPP identity: %w", err)
	}

	if idCfg == nil {
		snap.problem = errNoIdentity
		return snap, nil
	}

	prev := m.snap.Load()
	if prev != nil && prev.identity != nil && prev.op == op && prev.identity.cfg == *idCfg {
		snap.identity = prev.identity
		return snap, nil
	}

	identity, err := newLocalIdentity(*idCfg, op)
	if err != nil {
		snap.problem = "the SEPP certificate is unusable: " + err.Error()
		return snap, nil
	}

	snap.identity = identity

	return snap, nil
}

func (m *Manager) loadCluster(ctx context.Context) map[int]struct{} {
	if m.cfg.Cluster == nil {
		return nil
	}

	ids, err := m.cfg.Cluster(ctx)
	if err != nil {
		m.logger.Warn("Could not read the SEPP IDs of the cluster nodes", zap.Error(err))

		if prev := m.snap.Load(); prev != nil {
			return prev.clusterSEPPIDs
		}

		return nil
	}

	set := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}

	return set
}

func (m *Manager) renew(ctx context.Context, snap *snapshot) bool {
	id := snap.identity
	if id == nil || !id.generated || m.cfg.Renew == nil {
		return false
	}

	now := m.now()

	if id.leaf.NotAfter.Sub(now) > m.tuning.RenewBefore || now.Sub(m.renewAttempt) < renewRetry {
		return false
	}

	if id.anchor.NotAfter.Sub(now) <= m.tuning.RenewBefore {
		return false
	}

	m.renewAttempt = now

	if err := m.cfg.Renew(ctx, id.seppID); err != nil {
		m.logger.Warn("Could not renew this node's generated SEPP certificate", zap.Int("sepp_id", id.seppID), zap.Time("not_after", id.leaf.NotAfter), zap.Error(err))
		return false
	}

	m.logger.Info("Renewed this node's generated SEPP certificate with the cluster SEPP CA", zap.Int("sepp_id", id.seppID), zap.Time("previous_not_after", id.leaf.NotAfter))

	return true
}

func (m *Manager) checkExpiry(snap *snapshot) {
	if snap.identity == nil {
		return
	}

	now := m.now()
	notAfter := snap.identity.leaf.NotAfter

	if notAfter.Sub(now) > expiryWarning || now.Sub(m.expiryLog) < 24*time.Hour {
		return
	}

	m.expiryLog = now

	if now.After(notAfter) {
		m.logger.Error("The SEPP certificate has expired: partners reject N32 until it is replaced", zap.Time("not_after", notAfter))
		return
	}

	m.logger.Error("The SEPP certificate expires soon: replace it before partners reject N32", zap.Time("not_after", notAfter))
}

func (m *Manager) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return m.localCertificate()
}

func (m *Manager) getClientCertificate(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	return m.localCertificate()
}

func (m *Manager) localCertificate() (*tls.Certificate, error) {
	snap := m.snap.Load()
	if snap.identity == nil {
		return nil, ErrNoIdentity
	}

	return snap.identity.cert, nil
}

func (m *Manager) LocalFQDN() string {
	if id := m.snap.Load().identity; id != nil {
		return id.fqdn
	}

	return ""
}

func (m *Manager) SEPPID() int {
	if id := m.snap.Load().identity; id != nil {
		return id.seppID
	}

	return 0
}

func (m *Manager) Problem() string {
	return m.snap.Load().problem
}

func (m *Manager) Status() NodeStatus {
	return NodeStatus{
		SEPPID:   m.SEPPID(),
		FQDN:     m.LocalFQDN(),
		Listener: m.ListenerStatus(),
		Contexts: m.Contexts(),
	}
}

func (m *Manager) SetCallbacks(h http.Handler) {
	m.callbacks.Store(&h)
}

var ErrUnknownPartner = errors.New("sepp: partner has no 5G agreement")

func (m *Manager) partner(id string) (*snapshot, *partner, error) {
	snap := m.snap.Load()

	p, ok := snap.partners[id]
	if !ok {
		return snap, nil, ErrUnknownPartner
	}

	return snap, p, nil
}
