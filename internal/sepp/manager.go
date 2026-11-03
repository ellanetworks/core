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

type Config struct {
	Partners  PartnersSource
	Identity  IdentitySource
	Operator  OperatorSource
	Provision func(ctx context.Context) error
	N32       config.N32Interface
}

type Tuning struct {
	Reconcile        time.Duration
	HandshakeTimeout time.Duration
	OngoingWait      time.Duration
	BackoffMin       time.Duration
	BackoffMax       time.Duration
	ExpiryWarning    time.Duration
	RequestTimeout   time.Duration
	PingIdle         time.Duration
	PingTimeout      time.Duration
	ContextLostDelay time.Duration

	N32cRequestsPerMinute int
	N32fRequestsPerSecond int
}

func DefaultTuning() Tuning {
	return Tuning{
		Reconcile:        30 * time.Second,
		HandshakeTimeout: 5 * time.Second,
		OngoingWait:      10 * time.Second,
		BackoffMin:       2 * time.Second,
		BackoffMax:       60 * time.Second,
		ExpiryWarning:    30 * 24 * time.Hour,
		RequestTimeout:   4 * time.Second,
		PingIdle:         30 * time.Second,
		PingTimeout:      10 * time.Second,
		ContextLostDelay: 10 * time.Second,

		N32cRequestsPerMinute: 5,
		N32fRequestsPerSecond: 200,
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
	op       wire.PlmnID
	identity *localIdentity
	problem  string
	partners map[string]*partner
	trusted  []*trustedPartner
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

	n32cLimit rateLimiter
	n32fLimit rateLimiter
	expiryLog time.Time
}

type Option func(*Manager)

func WithTuning(t Tuning) Option {
	return func(m *Manager) { m.tuning = t }
}

func WithDialer(dial func(ctx context.Context, network, addr string) (net.Conn, error)) Option {
	return func(m *Manager) { m.dial = dial }
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
	}

	for _, o := range opts {
		o(m)
	}

	m.snap.Store(&snapshot{problem: "not reconciled yet", partners: map[string]*partner{}})
	m.n32cLimit.init(time.Minute)
	m.n32fLimit.init(time.Second)

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

	prev := m.snap.Swap(next)

	if prev.problem != next.problem && next.problem != "" {
		m.logger.Warn("SEPP is not operational on this node", zap.String("reason", next.problem))
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

const errNoIdentity = "no SEPP certificate for this node"

func (m *Manager) load(ctx context.Context) (*snapshot, error) {
	configs, err := m.cfg.Partners(ctx)
	if err != nil {
		return nil, fmt.Errorf("read roaming partners: %w", err)
	}

	snap := &snapshot{partners: make(map[string]*partner, len(configs))}

	for _, cfg := range configs {
		p := newPartner(cfg)
		snap.partners[p.id] = p

		if p.trust != nil {
			snap.trusted = append(snap.trusted, p.trust)
		}

		if p.problem != "" {
			m.logger.Warn("Roaming partner SEPP configuration is unusable", zap.String("partner", p.name), zap.String("reason", p.problem))
		}
	}

	if !m.cfg.N32.Enabled {
		snap.problem = "interfaces.n32 is not configured on this node"
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
	if prev != nil && prev.identity != nil && prev.op == op && prev.identity.key == identityKey(*idCfg) {
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

func (m *Manager) checkExpiry(snap *snapshot) {
	if snap.identity == nil {
		return
	}

	now := m.now()
	notAfter := snap.identity.leaf.NotAfter

	if notAfter.Sub(now) > m.tuning.ExpiryWarning || now.Sub(m.expiryLog) < 24*time.Hour {
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
		tlsFailures.WithLabelValues(TLSReasonNoLocalIdentity).Inc()
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
