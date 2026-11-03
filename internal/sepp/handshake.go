// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/netutil"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"go.uber.org/zap"
)

type State string

const (
	StateIdle         State = "idle"
	StateEstablishing State = "establishing"
	StateEstablished  State = "established"
	StateFailed       State = "failed"
	StateTerminated   State = "terminated"
)

func (s State) Gauge() float64 {
	switch s {
	case StateEstablishing:
		return 1
	case StateEstablished:
		return 2
	case StateFailed:
		return 3
	case StateTerminated:
		return 4
	}

	return 0
}

type Initiator string

const (
	InitiatorLocal Initiator = "local"
	InitiatorPeer  Initiator = "peer"
)

type ContextStatus struct {
	State      State         `json:"state"`
	Since      time.Time     `json:"since"`
	Initiator  Initiator     `json:"initiator,omitempty"`
	NFTLST     bool          `json:"nftlst"`
	Peer       string        `json:"peer,omitempty"`
	LastError  string        `json:"lastError,omitempty"`
	OtherPeers []PeerContext `json:"otherPeers,omitempty"`
}

type PeerContext struct {
	Peer   string    `json:"peer"`
	Since  time.Time `json:"since"`
	NFTLST bool      `json:"nftlst"`
}

const (
	roleInitiator = "initiator"
	roleResponder = "responder"

	resultEstablished = "established"
	resultRejected    = "rejected"
	resultFailed      = "failed"
	resultConflict    = "conflict"
	resultTeardown    = "teardown"
)

var (
	ErrNotReady = errors.New("sepp: the SEPP is not running on this node")
	errOngoing  = errors.New("the partner SEPP is running its own exchange-capability (N32C_EXCHANGE_CAPABILITY_ONGOING)")
)

type n32cContext struct {
	status   ContextStatus
	partner  *partner
	identity *localIdentity
	gen      uint64
	inflight bool
	cancel   context.CancelFunc
	timer    *time.Timer
	backoff  time.Duration
	lostAt   time.Time
	peer     *seppPeer
	others   map[string]PeerContext
}

func (c *n32cContext) snapshot() ContextStatus {
	st := c.status
	st.OtherPeers = nil

	for _, pc := range c.others {
		st.OtherPeers = append(st.OtherPeers, pc)
	}

	slices.SortFunc(st.OtherPeers, func(a, b PeerContext) int { return strings.Compare(a.Peer, b.Peer) })

	return st
}

func (c *n32cContext) correlates(dnsNames []string) bool {
	has := func(fqdn string) bool {
		return slices.ContainsFunc(dnsNames, func(n string) bool { return sameFQDN(n, fqdn) })
	}

	if c.status.State == StateEstablished && has(c.peerOf().fqdn) {
		return true
	}

	for fqdn := range c.others {
		if has(fqdn) {
			return true
		}
	}

	return false
}

func (m *Manager) dropOthersLocked(c *n32cContext) {
	for fqdn := range c.others {
		go m.closeInboundFrom(c.partner.id, fqdn)
	}

	c.others = nil
}

func (c *n32cContext) stop() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}

	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}

	c.inflight = false
}

func (m *Manager) setState(p *partner, c *n32cContext, state State, lastError string) {
	if c.status.State != state {
		m.logger.Info("N32-c state changed", zap.String("partner", p.name), zap.String("from", string(c.status.State)), zap.String("to", string(state)), zap.String("reason", lastError))

		if c.status.State == StateEstablished {
			go m.closeN32f(p.id)
		}

		c.status.State = state
		c.status.Since = m.now()
	}

	c.status.LastError = lastError
}

func (m *Manager) syncContextsLocked(prev, next *snapshot) {
	all := prev == nil || prev.identity != next.identity || prev.op != next.op

	for id, c := range m.contexts {
		if _, ok := next.partners[id]; ok {
			continue
		}

		if c.status.State == StateEstablished && c.status.NFTLST && c.identity != nil {
			go m.teardown(c.partner, c.peerOf(), c.identity)
		}

		c.stop()
		c.others = nil

		delete(m.contexts, id)
		go m.closeN32f(id)

		m.logger.Info("N32-c context removed with the partner's 5G agreement", zap.String("partner", c.partner.name))
	}

	for id, p := range next.partners {
		c, ok := m.contexts[id]
		if !ok {
			c = &n32cContext{status: ContextStatus{State: StateIdle, Since: m.now()}}
			m.contexts[id] = c
		}

		changed := !ok || c.partner.key != p.key
		c.partner = p

		switch {
		case !next.usable():
			c.stop()
			m.dropOthersLocked(c)
			m.setState(p, c, StateIdle, next.problem)
		case !p.usable():
			c.stop()
			m.dropOthersLocked(c)
			m.setState(p, c, StateIdle, p.problem)
		case changed || all:
			m.dropOthersLocked(c)
			m.startLocked(id, c, next)
		case c.status.State == StateIdle:
			m.startLocked(id, c, next)
		}
	}
}

func (m *Manager) startLocked(id string, c *n32cContext, snap *snapshot) {
	c.stop()
	c.gen++

	m.setState(c.partner, c, StateEstablishing, c.status.LastError)
	c.inflight = true

	ctx, cancel := context.WithTimeout(m.root, m.tuning.HandshakeTimeout)
	c.cancel = cancel

	go m.initiate(ctx, id, c.gen, c.partner, snap.identity, snap.op)
}

func (m *Manager) current(id string, gen uint64) *n32cContext {
	if m.root == nil {
		return nil
	}

	c, ok := m.contexts[id]
	if !ok || c.gen != gen {
		return nil
	}

	return c
}

func (m *Manager) Context(partnerID string) ContextStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.contexts[partnerID]; ok {
		return c.snapshot()
	}

	reason := m.snap.Load().problem
	if reason == "" {
		reason = ErrUnknownPartner.Error()
	}

	return ContextStatus{State: StateIdle, Since: m.now(), LastError: reason}
}

func (c *n32cContext) peerOf() seppPeer {
	if c.peer != nil {
		return *c.peer
	}

	return c.partner.defaultPeer()
}

func (m *Manager) contextPeer(p *partner) (ContextStatus, seppPeer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.contexts[p.id]
	if !ok {
		return ContextStatus{State: StateIdle}, p.defaultPeer()
	}

	if c.peer != nil {
		return c.status, *c.peer
	}

	return c.status, p.defaultPeer()
}

func (m *Manager) Contexts() map[string]ContextStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make(map[string]ContextStatus, len(m.contexts))
	for id, c := range m.contexts {
		out[id] = c.snapshot()
	}

	return out
}

func (m *Manager) Rehandshake(partnerID string) (ContextStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.root == nil {
		return ContextStatus{}, ErrNotReady
	}

	c, ok := m.contexts[partnerID]
	if !ok {
		return ContextStatus{}, ErrUnknownPartner
	}

	snap := m.snap.Load()

	switch {
	case !snap.usable():
		m.setState(c.partner, c, StateIdle, snap.problem)
	case !c.partner.usable():
		m.setState(c.partner, c, StateIdle, c.partner.problem)
	default:
		m.startLocked(partnerID, c, snap)
	}

	return c.snapshot(), nil
}

func (m *Manager) negotiation(p *partner, id *localIdentity, op wire.PlmnID, dnsNames []string) Negotiation {
	return Negotiation{
		LocalFQDN:         id.fqdn,
		LocalPlmnID:       op,
		PeerFQDN:          p.fqdn,
		PeerPlmnID:        p.main,
		PeerPlmnIDs:       p.plmns,
		PeerCertDNSNames:  dnsNames,
		SupportedFeatures: LocalFeatures,
	}
}

func (m *Manager) initiate(ctx context.Context, id string, gen uint64, p *partner, ident *localIdentity, op wire.PlmnID) {
	neg := m.negotiation(p, ident, op, nil)
	peer := p.defaultPeer()

	var (
		rsp   SecNegotiateRspData
		names []string
		err   error
		est   Established
	)

	for redirects := 0; ; redirects++ {
		rsp, names, err = m.postExchange(ctx, p, peer, neg.Request())

		var re *redirectError
		if !errors.As(err, &re) {
			break
		}

		if redirects == maxN32cRedirects {
			err = fmt.Errorf("%w: the partner SEPP redirected the exchange-capability more than once", ErrRedirect)
			break
		}

		if re.discovery {
			err = fmt.Errorf("%w: %s needs DNS-based SEPP discovery (TS 29.573 clause 6.1.8.2)", ErrRedirect, causeSEPPRedirectionWithDiscovery)
			break
		}

		next, rerr := p.redirectPeer(re.location)
		if rerr != nil {
			err = rerr
			break
		}

		m.logger.Info("The partner SEPP redirected the N32-c handshake to another of its SEPPs", zap.String("partner", p.name), zap.String("sepp", next.fqdn))

		peer = next
		neg.PeerFQDN = peer.fqdn
	}

	if err == nil {
		neg.PeerCertDNSNames = names
		est, err = neg.CheckResponse(rsp)

		var ne *NegotiationError
		if errors.As(err, &ne) && ne.TeardownAllowed {
			tdCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.tuning.HandshakeTimeout)
			m.sendTeardown(tdCtx, p, peer, ident)
			cancel()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	c := m.current(id, gen)
	if c == nil {
		m.logger.Debug("Discarded the answer to a superseded N32-c request", zap.String("partner", p.name), zap.Error(err))
		return
	}

	c.inflight = false
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}

	switch {
	case err == nil:
		handshakes.WithLabelValues(p.name, roleInitiator, resultEstablished).Inc()

		var redirected *seppPeer
		if peer.fqdn != p.fqdn {
			redirected = &peer
		}

		m.establishedLocked(c, InitiatorLocal, est.NFTLST, ident, redirected)
	case errors.Is(err, errOngoing):
		handshakes.WithLabelValues(p.name, roleInitiator, resultConflict).Inc()

		c.status.LastError = err.Error()
		c.timer = time.AfterFunc(m.tuning.OngoingWait, func() {
			m.mu.Lock()
			defer m.mu.Unlock()

			if c := m.current(id, gen); c != nil && c.status.State == StateEstablishing && !c.inflight {
				m.failLocked(id, c, errors.New("the partner SEPP answered N32C_EXCHANGE_CAPABILITY_ONGOING but sent no exchange-capability"))
			}
		})
	default:
		result := resultFailed
		if errors.Is(err, ErrNegotiation) {
			result = resultRejected
		}

		handshakes.WithLabelValues(p.name, roleInitiator, result).Inc()
		m.failLocked(id, c, err)
	}
}

func (m *Manager) establishedLocked(c *n32cContext, initiator Initiator, nftlst bool, ident *localIdentity, peer *seppPeer) {
	if c.status.State == StateEstablished {
		go m.closeN32f(c.partner.id)
	}

	c.stop()
	c.backoff = 0
	c.identity = ident
	c.peer = peer
	c.status.Peer = ""

	if peer != nil {
		c.status.Peer = peer.fqdn
	}

	delete(c.others, c.peerOf().fqdn)

	m.setState(c.partner, c, StateEstablished, "")
	c.status.Initiator = initiator
	c.status.NFTLST = nftlst
	c.status.Since = m.now()
}

func (m *Manager) failLocked(id string, c *n32cContext, err error) {
	m.logger.Warn("N32-c handshake failed", zap.String("partner", c.partner.name), zap.Error(err))
	m.setState(c.partner, c, StateFailed, err.Error())
	m.retryLocked(id, c)
}

func (m *Manager) retryLocked(id string, c *n32cContext) {
	c.stop()

	if c.backoff == 0 {
		c.backoff = m.tuning.BackoffMin
	} else {
		c.backoff = min(2*c.backoff, m.tuning.BackoffMax)
	}

	delay := time.Duration(float64(c.backoff) * (0.8 + 0.4*rand.Float64())) // #nosec G404 -- jitter, not a secret

	gen := c.gen
	c.timer = time.AfterFunc(delay, func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		c := m.current(id, gen)
		if c == nil || (c.status.State != StateFailed && c.status.State != StateTerminated) {
			return
		}

		snap := m.snap.Load()
		if !snap.usable() || !c.partner.usable() {
			return
		}

		m.startLocked(id, c, snap)
	})
}

type redirectError struct {
	location  string
	discovery bool
}

func (e *redirectError) Error() string {
	return "the partner SEPP redirected the exchange-capability to " + e.location
}

type redirectResponse struct {
	Cause string `json:"cause"`
}

func (m *Manager) postExchange(ctx context.Context, p *partner, peer seppPeer, req SecNegotiateReqData) (SecNegotiateRspData, []string, error) {
	data, err := wire.Encode(req)
	if err != nil {
		return SecNegotiateRspData{}, nil, fmt.Errorf("encode exchange-capability: %w", err)
	}

	tr := m.transport(p, peer)
	defer tr.CloseIdleConnections()

	client := &http.Client{
		Transport:     tr,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, peer.exchangeCapabilityURL(), bytes.NewReader(data))
	if err != nil {
		return SecNegotiateRspData{}, nil, err
	}

	httpReq.Header.Set("Content-Type", wire.ContentTypeJSON)
	httpReq.Header.Set("Accept", wire.ContentTypeJSON+", "+wire.ContentTypeProblem)

	resp, err := client.Do(httpReq)
	if err != nil {
		return SecNegotiateRspData{}, nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return SecNegotiateRspData{}, nil, fmt.Errorf("%w: read response: %w", ErrUnreachable, err)
	}

	var names []string
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		names = resp.TLS.PeerCertificates[0].DNSNames
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		rsp, err := wire.Decode[SecNegotiateRspData](body)
		if err != nil {
			return SecNegotiateRspData{}, nil, &NegotiationError{Reason: "invalid SecNegotiateRspData: " + err.Error()}
		}

		return rsp, names, nil
	case resp.StatusCode == http.StatusTemporaryRedirect && !req.Teardown():
		var rr redirectResponse

		discovery := len(body) > 0 && json.Unmarshal(body, &rr) == nil && rr.Cause == causeSEPPRedirectionWithDiscovery

		return SecNegotiateRspData{}, nil, &redirectError{location: resp.Header.Get("Location"), discovery: discovery}
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		return SecNegotiateRspData{}, nil, fmt.Errorf("%w: the partner SEPP answered %d", ErrRedirect, resp.StatusCode)
	}

	pd := wire.ParseProblem(resp.StatusCode, body)
	if resp.StatusCode == http.StatusConflict && pd.Cause == CauseN32cExchangeCapabilityOngoing {
		return SecNegotiateRspData{}, nil, errOngoing
	}

	return SecNegotiateRspData{}, nil, fmt.Errorf("the partner SEPP refused the exchange-capability: %w", pd)
}

func (m *Manager) sendTeardown(ctx context.Context, p *partner, peer seppPeer, ident *localIdentity) {
	neg := m.negotiation(p, ident, wire.PlmnID{}, nil)
	neg.PeerFQDN = peer.fqdn

	rsp, names, err := m.postExchange(ctx, p, peer, neg.TeardownRequest())
	if err == nil {
		neg.PeerCertDNSNames = names
		err = neg.CheckTeardownResponse(rsp)
	}

	if err != nil {
		handshakes.WithLabelValues(p.name, roleInitiator, resultFailed).Inc()
		m.logger.Warn("N32-c teardown failed", zap.String("partner", p.name), zap.Error(err))

		return
	}

	handshakes.WithLabelValues(p.name, roleInitiator, resultTeardown).Inc()
	m.logger.Info("N32-c context torn down", zap.String("partner", p.name))
}

func (m *Manager) teardown(p *partner, peer seppPeer, ident *localIdentity) {
	ctx, cancel := context.WithTimeout(context.Background(), m.tuning.HandshakeTimeout)
	defer cancel()

	m.sendTeardown(ctx, p, peer, ident)
}

func (m *Manager) transport(p *partner, peer seppPeer) *http.Transport {
	protocols := new(http.Protocols)
	protocols.SetHTTP2(true)

	return &http.Transport{
		TLSClientConfig:        m.clientTLSConfig(p, peer.n32c.Hostname(), peer.fqdn),
		DialContext:            m.dialContext,
		TLSHandshakeTimeout:    m.tuning.HandshakeTimeout,
		MaxResponseHeaderBytes: maxHeaderBytes,
		Protocols:              protocols,
	}
}

func (m *Manager) ClientTLSConfig(partnerID, host string) (*tls.Config, error) {
	_, p, err := m.partner(partnerID)
	if err != nil {
		return nil, err
	}

	if !p.usable() {
		return nil, errors.New(p.problem)
	}

	return m.clientTLSConfig(p, host, p.fqdn), nil
}

func (m *Manager) clientTLSConfig(p *partner, host, peerFQDN string) *tls.Config {
	cfg := baseTLSConfig()
	cfg.ServerName = ServerName(host, peerFQDN)
	cfg.InsecureSkipVerify = true // #nosec G402 -- VerifyConnection verifies the chain against the partner's trust anchor
	cfg.Renegotiation = tls.RenegotiateNever
	cfg.GetClientCertificate = m.getClientCertificate
	cfg.ClientSessionCache = tls.NewLRUClientSessionCache(0)

	trust := p.trust
	cfg.VerifyConnection = func(st tls.ConnectionState) error {
		err := checkALPN(st)
		if err == nil {
			err = trust.checkAs(st.PeerCertificates, m.now(), x509.ExtKeyUsageServerAuth, peerFQDN)
		}

		if err != nil {
			tlsFailures.WithLabelValues(TLSFailureReason(err)).Inc()
		}

		return err
	}

	return cfg
}

func (m *Manager) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if m.dial != nil {
		return m.dial(ctx, network, addr)
	}

	d := &net.Dialer{Timeout: m.tuning.HandshakeTimeout}

	device := m.cfg.N32.Name

	if m.cfg.N32.Address != "" {
		if ip := net.ParseIP(m.cfg.N32.Address); ip != nil && !ip.IsUnspecified() {
			d.LocalAddr = &net.TCPAddr{IP: ip}
		}

		vrf, err := netutil.VRFDeviceForAddress(m.cfg.N32.Address)
		if err != nil {
			return nil, err
		}

		device = vrf
	}

	if device != "" {
		d.Control = netutil.BindToDeviceControl(device)
	}

	return d.DialContext(ctx, network, addr)
}
