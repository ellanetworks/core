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
	"maps"
	"net"
	"net/http"
	"slices"
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
)

type Initiator string

const (
	InitiatorLocal Initiator = "local"
	InitiatorPeer  Initiator = "peer"
)

type ContextStatus struct {
	State      State
	Since      time.Time
	Initiator  Initiator
	Peer       string
	LastError  string
	OtherPeers []PeerContext
}

type PeerContext struct {
	Peer  string
	Since time.Time
}

var (
	ErrNotReady = errors.New("sepp: the SEPP is not running on this node")
	errOngoing  = errors.New("the partner SEPP is running its own exchange-capability (N32C_EXCHANGE_CAPABILITY_ONGOING)")
)

type n32cContext struct {
	status   ContextStatus
	partner  *partner
	gen      uint64
	inflight bool
	cancel   context.CancelFunc
	peer     *seppPeer
	others   map[string]time.Time
}

func (c *n32cContext) snapshot() ContextStatus {
	st := c.status
	st.OtherPeers = nil

	for _, fqdn := range slices.Sorted(maps.Keys(c.others)) {
		st.OtherPeers = append(st.OtherPeers, PeerContext{Peer: fqdn, Since: c.others[fqdn]})
	}

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
	all := prev == nil || !prev.identity.sameSession(next.identity) || prev.op != next.op

	for id, c := range m.contexts {
		if _, ok := next.partners[id]; ok {
			continue
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

		changed := !ok || !c.partner.sameN32(p)
		c.partner = p

		switch {
		case !next.usable():
			c.stop()
			m.dropOthersLocked(c)
			m.setState(p, c, StateIdle, next.problem)
		case changed || all:
			m.dropOthersLocked(c)
			m.startLocked(id, c, next)
		case !c.inflight && c.status.State != StateEstablished:
			m.startLocked(id, c, next)
		}
	}
}

func (m *Manager) startLocked(id string, c *n32cContext, snap *snapshot) {
	c.stop()
	c.gen++

	if c.status.State != StateFailed {
		m.setState(c.partner, c, StateEstablishing, c.status.LastError)
	}

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

func (m *Manager) ConnectedPartners() int {
	snap := m.snap.Load()
	contexts := m.Contexts()

	n := 0

	for id := range snap.partners {
		if contexts[id].State == StateEstablished {
			n++
		}
	}

	return n
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

	if snap.usable() {
		m.setState(c.partner, c, StateEstablishing, c.status.LastError)
		m.startLocked(partnerID, c, snap)
	} else {
		m.setState(c.partner, c, StateIdle, snap.problem)
	}

	return c.snapshot(), nil
}

func (m *Manager) negotiation(p *partner, id *localIdentity, op wire.PlmnID, dnsNames []string) Negotiation {
	return Negotiation{
		LocalFQDN:        id.fqdn,
		LocalPlmnID:      op,
		PeerFQDN:         p.fqdn,
		PeerPlmnID:       p.main,
		PeerPlmnIDs:      p.plmns,
		PeerCertDNSNames: dnsNames,
	}
}

func (m *Manager) initiate(ctx context.Context, id string, gen uint64, p *partner, ident *localIdentity, op wire.PlmnID) {
	neg := m.negotiation(p, ident, op, nil)
	peer := p.defaultPeer()

	var (
		rsp   SecNegotiateRspData
		names []string
		err   error
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
		err = neg.CheckResponse(rsp)
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
		var redirected *seppPeer
		if peer.fqdn != p.fqdn {
			redirected = &peer
		}

		m.establishedLocked(c, InitiatorLocal, redirected)
	case errors.Is(err, errOngoing):
		c.status.LastError = err.Error()
	default:
		if c.status.State != StateFailed || c.status.LastError != err.Error() {
			m.logger.Warn("N32-c handshake failed", zap.String("partner", p.name), zap.Error(err))
		}

		m.setState(p, c, StateFailed, err.Error())
	}
}

func (m *Manager) establishedLocked(c *n32cContext, initiator Initiator, peer *seppPeer) {
	if c.status.State == StateEstablished {
		go m.closeN32f(c.partner.id)
	}

	c.stop()
	c.peer = peer
	c.status.Peer = ""

	if peer != nil {
		c.status.Peer = peer.fqdn
	}

	delete(c.others, c.peerOf().fqdn)

	m.setState(c.partner, c, StateEstablished, "")
	c.status.Initiator = initiator
	c.status.Since = m.now()
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
	case resp.StatusCode == http.StatusTemporaryRedirect:
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
