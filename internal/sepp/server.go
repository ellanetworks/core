// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/netutil"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"go.uber.org/zap"
)

const (
	maxBodyBytes          = 256 << 10
	maxHeaderBytes        = 16 << 10
	maxConcurrentStreams  = 100
	readHeaderTimeout     = 5 * time.Second
	idleTimeout           = 5 * time.Minute
	retryAfterSeconds     = 5
	listenerShutdownGrace = 2 * time.Second
)

type connClass int

const (
	classUnset connClass = iota
	classN32c
	classN32f
)

type connState struct {
	conn     net.Conn
	mu       sync.Mutex
	partner  string
	dnsNames []string
	class    connClass
}

func (c *connState) identity() (string, []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.partner, c.dnsNames
}

func (c *connState) is(partnerID string, class connClass) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.partner == partnerID && c.class == class
}

func (c *connState) fix(class connClass) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.class == classUnset {
		c.class = class
	}

	return c.class == class
}

type connKey struct{}

func connFrom(ctx context.Context) *connState {
	cs, _ := ctx.Value(connKey{}).(*connState)
	return cs
}

func (m *Manager) serverTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		GetConfigForClient: m.configForClient,
	}
}

func (m *Manager) configForClient(hello *tls.ClientHelloInfo) (*tls.Config, error) {
	v, ok := m.conns.Load(hello.Conn)
	if !ok {
		return nil, errors.New("sepp: unknown connection")
	}

	cs, _ := v.(*connState)

	cfg := baseTLSConfig()
	cfg.ClientAuth = tls.RequestClientCert
	cfg.GetCertificate = m.getCertificate
	cfg.VerifyConnection = func(st tls.ConnectionState) error {
		p, err := m.verifyClient(st)
		if err != nil {
			tlsFailures.WithLabelValues(TLSFailureReason(err)).Inc()
			m.logger.Warn("Rejected an N32 TLS connection", zap.String("remote", hello.Conn.RemoteAddr().String()), zap.String("reason", TLSFailureReason(err)), zap.Error(err))

			return err
		}

		names := append([]string(nil), st.PeerCertificates[0].DNSNames...)

		cs.mu.Lock()
		cs.partner, cs.dnsNames = p.id, names
		cs.mu.Unlock()

		return nil
	}

	return cfg, nil
}

func (m *Manager) verifyClient(st tls.ConnectionState) (*trustedPartner, error) {
	if err := checkALPN(st); err != nil {
		return nil, err
	}

	return identifyPeer(m.snap.Load().trusted, st.PeerCertificates, m.now(), x509.ExtKeyUsageClientAuth)
}

func (m *Manager) ListenerStatus() ListenerStatus {
	m.listenMu.Lock()
	defer m.listenMu.Unlock()

	st := ListenerStatus{Configured: m.cfg.N32.Enabled}
	if !m.cfg.N32.Enabled {
		return st
	}

	st.Address = m.cfg.N32.Address
	if st.Address == "" {
		st.Address = m.cfg.N32.Name
	}

	st.Port = m.cfg.N32.Port
	st.Listening = m.listener != nil
	st.Error = m.listenErr

	if st.Listening && st.Error == "" {
		st.Error = m.snap.Load().problem
	}

	return st
}

func (m *Manager) ListenAddr() net.Addr {
	m.listenMu.Lock()
	defer m.listenMu.Unlock()

	if m.listener == nil {
		return nil
	}

	return m.listener.Addr()
}

func (m *Manager) syncListener(ctx context.Context) {
	m.listenMu.Lock()
	defer m.listenMu.Unlock()

	if !m.cfg.N32.Enabled || m.server != nil {
		return
	}

	ln, err := listenTCP(ctx, m.cfg.N32)
	if err != nil {
		m.listenErr = "listen on TCP port " + strconv.Itoa(m.cfg.N32.Port) + ": " + err.Error()
		m.logger.Error("Could not start the N32 listener", zap.Error(err))

		return
	}

	m.listenErr = ""
	m.listener = ln

	srv := &http.Server{
		Handler:           m,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          logger.StdLogger(m.logger),
		HTTP2:             &http.HTTP2Config{MaxConcurrentStreams: maxConcurrentStreams},
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			cs := &connState{conn: c}

			if tc, ok := c.(*tls.Conn); ok {
				m.conns.Store(tc.NetConn(), cs)
			}

			return context.WithValue(ctx, connKey{}, cs)
		},
		ConnState: func(c net.Conn, s http.ConnState) {
			if s != http.StateClosed && s != http.StateHijacked {
				return
			}

			if tc, ok := c.(*tls.Conn); ok {
				m.conns.Delete(tc.NetConn())
			}
		},
	}

	protocols := new(http.Protocols)
	protocols.SetHTTP2(true)
	srv.Protocols = protocols

	m.server = srv

	tlsLn := tls.NewListener(ln, m.serverTLSConfig())

	go func() {
		if err := srv.Serve(tlsLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
			m.logger.Error("N32 listener stopped", zap.Error(err))

			m.listenMu.Lock()
			m.listenErr = err.Error()
			m.listenMu.Unlock()
		}
	}()

	m.logger.Info("N32 listener started", zap.String("address", ln.Addr().String()))
}

func (m *Manager) stopListener() {
	m.listenMu.Lock()
	srv := m.server
	m.server, m.listener = nil, nil
	m.listenMu.Unlock()

	if srv == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), listenerShutdownGrace)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		_ = srv.Close()
	}
}

func listenTCP(ctx context.Context, n config.N32Interface) (net.Listener, error) {
	addr := net.JoinHostPort(n.Address, strconv.Itoa(n.Port))

	device := n.Name
	if device == "" {
		d, err := netutil.VRFDeviceForAddress(n.Address)
		if err != nil {
			return nil, err
		}

		device = d
	}

	var ln net.Listener

	err := netutil.Retry(ctx, netutil.BindTimeout, netutil.BindInterval, netutil.IsAddrNotAvailable, func() error {
		lc := net.ListenConfig{}
		if device != "" {
			lc.Control = netutil.BindToDeviceControl(device)
		}

		var err error

		ln, err = lc.Listen(ctx, "tcp", addr)

		return err
	})

	return ln, err
}

func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cs := connFrom(r.Context())
	if cs == nil {
		m.writeProblem(w, wire.NewProblem(http.StatusForbidden, wire.CauseUnspecifiedMsgFailure, "no N32 TLS identity"))
		return
	}

	partnerID, dnsNames := cs.identity()

	class := classN32f
	if r.URL.Path == N32cHandshakePath || strings.HasPrefix(r.URL.Path, N32cHandshakePath+"/") {
		class = classN32c
	}

	if !cs.fix(class) {
		w.Header().Set("Connection", "close")
		m.writeProblem(w, wire.NewProblem(http.StatusBadRequest, wire.CauseUnspecifiedMsgFailure, "N32-c and N32-f must use separate TLS connections"))

		return
	}

	if class == classN32f {
		m.serveN32f(w, r, partnerID, dnsNames)
		return
	}

	if r.URL.Path != N32cExchangeCapabilityPath {
		m.writeProblem(w, wire.NewProblem(http.StatusNotFound, wire.CauseResourceURIStructureNotFound, "only exchange-capability is supported (TLS security)"))
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		m.writeProblem(w, wire.NewProblem(http.StatusMethodNotAllowed, "", "exchange-capability accepts POST only"))

		return
	}

	w.Header().Set("Connection", "close")

	if !m.n32cLimit.allow(partnerID, m.now(), m.tuning.N32cRequestsPerMinute) {
		w.Header().Set("Retry-After", strconv.Itoa(n32cRateRetryAfterS))
		m.writeProblem(w, wire.NewProblem(http.StatusTooManyRequests, wire.CauseNFCongestionRisk, "too many N32-c requests"))

		return
	}

	body, ok := m.readBody(w, r)
	if !ok {
		return
	}

	status, rsp := m.respond(partnerID, dnsNames, body)

	if status == http.StatusServiceUnavailable {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	}

	m.writeBody(w, status, rsp)
}

func (m *Manager) readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err == nil {
		return body, true
	}

	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		m.writeProblem(w, wire.NewProblem(http.StatusRequestEntityTooLarge, wire.CauseMaxJSONSizeExceeded, "request body too large"))
		return nil, false
	}

	m.writeProblem(w, wire.NewProblem(http.StatusBadRequest, wire.CauseInvalidMsgFormat, "could not read the request body"))

	return nil, false
}

func (m *Manager) closeInbound(partnerID string, class connClass) {
	m.conns.Range(func(_, v any) bool {
		if cs, ok := v.(*connState); ok && cs.is(partnerID, class) && cs.conn != nil {
			_ = cs.conn.Close()
		}

		return true
	})
}

func (m *Manager) closeInboundFrom(partnerID, fqdn string) {
	m.conns.Range(func(_, v any) bool {
		cs, ok := v.(*connState)
		if !ok || cs.conn == nil || !cs.is(partnerID, classN32f) {
			return true
		}

		if _, names := cs.identity(); slices.ContainsFunc(names, func(n string) bool { return sameFQDN(n, fqdn) }) {
			_ = cs.conn.Close()
		}

		return true
	})
}

func (m *Manager) writeProblem(w http.ResponseWriter, pd wire.ProblemDetails) {
	body, err := json.Marshal(pd)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	m.writeBody(w, pd.Status, body)
}

func (m *Manager) writeBody(w http.ResponseWriter, status int, body []byte) {
	if fqdn := m.LocalFQDN(); fqdn != "" {
		w.Header().Set(HeaderServer, SEPPServer(fqdn))
	}

	ct := wire.ContentTypeJSON
	if status >= http.StatusBadRequest {
		ct = wire.ContentTypeProblem
	}

	w.Header().Set("Content-Type", ct)
	w.WriteHeader(status)

	if _, err := w.Write(body); err != nil {
		m.logger.Debug("Could not write an N32 response", zap.Error(err))
	}
}
