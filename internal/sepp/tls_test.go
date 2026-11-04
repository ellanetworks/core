// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/pki"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"go.uber.org/zap"
)

const (
	partnerAFQDN = "sepp1.5gc.mnc001.mcc001.3gppnetwork.org"
	partnerBFQDN = "sepp1.5gc.mnc010.mcc002.3gppnetwork.org"
	ownFQDN      = "sepp1.5gc.mnc070.mcc999.3gppnetwork.org"
)

var ownPLMN = wire.PlmnID{Mcc: "999", Mnc: "70"}

func mustTrusted(t *testing.T, id, fqdn string, anchors string, plmns ...pki.PLMN) *trustedPartner {
	t.Helper()

	p, err := newTrustedPartner(id, fqdn, plmns, anchors)
	if err != nil {
		t.Fatal(err)
	}

	return p
}

func TestIdentifyPeer(t *testing.T) {
	caA, caB, shared, rogue := newTestCA(t, "A"), newTestCA(t, "B"), newTestCA(t, "IPX"), newTestCA(t, "rogue")
	plmnA, plmnA2, plmnB := pki.PLMN{MCC: "001", MNC: "01"}, pki.PLMN{MCC: "001", MNC: "02"}, pki.PLMN{MCC: "002", MNC: "10"}

	a := mustTrusted(t, "a", partnerAFQDN, caA.pem()+shared.pem(), plmnA, plmnA2)
	b := mustTrusted(t, "b", partnerBFQDN, caB.pem()+shared.pem(), plmnB)
	partners := []*trustedPartner{a, b}

	leaf := func(ca testCA, names ...string) []*x509.Certificate {
		c, _ := ca.issue(t, names)
		return []*x509.Certificate{c, ca.cert}
	}

	expired, _ := caA.issue(t, []string{partnerAFQDN}, leafOpts{notBefore: time.Now().Add(-2 * time.Hour), notAfter: time.Now().Add(-time.Hour), eku: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
	serverOnly, _ := caA.issue(t, []string{partnerAFQDN}, leafOpts{notBefore: time.Now().Add(-time.Hour), notAfter: time.Now().Add(time.Hour), eku: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})

	for name, tc := range map[string]struct {
		partners []*trustedPartner
		certs    []*x509.Certificate
		want     string
		reason   string
	}{
		"partner A":                      {partners, leaf(caA, partnerAFQDN), "a", ""},
		"partner A, second PLMN SAN":     {partners, leaf(caA, partnerAFQDN, "sepp2.5gc.mnc002.mcc001.3gppnetwork.org"), "a", ""},
		"partner A, mixed case SAN":      {partners, leaf(caA, strings.ToUpper(partnerAFQDN)), "a", ""},
		"partner A, non-PLMN SAN too":    {partners, leaf(caA, partnerAFQDN, "sepp.example.org"), "a", ""},
		"partner B via shared IPX CA":    {partners, leaf(shared, partnerBFQDN), "b", ""},
		"no certificate":                 {partners, nil, "", TLSReasonNoCertificate},
		"wrong anchor":                   {partners, leaf(rogue, partnerAFQDN), "", TLSReasonUnknownAnchor},
		"other partner's anchor":         {partners, leaf(caB, partnerAFQDN), "", TLSReasonPLMNMismatch},
		"another SEPP of partner A":      {partners, leaf(caA, "sepp9.5gc.mnc001.mcc001.3gppnetwork.org"), "a", ""},
		"SEPP of A's second PLMN":        {partners, leaf(caA, "sepp3.5gc.mnc002.mcc001.3gppnetwork.org"), "a", ""},
		"no PLMN in SAN":                 {partners, leaf(caA, "sepp.example.org"), "", TLSReasonPLMNMismatch},
		"no SAN at all":                  {partners, leaf(caA), "", TLSReasonPLMNMismatch},
		"other partner's PLMN":           {partners, leaf(caA, partnerAFQDN, "sepp1.5gc.mnc010.mcc002.3gppnetwork.org"), "", TLSReasonPLMNMismatch},
		"other partner's PLMN, IPX":      {partners, leaf(shared, partnerBFQDN, "sepp1.5gc.mnc001.mcc001.ipx1.ipxnetwork.org"), "", TLSReasonPLMNMismatch},
		"unknown PLMN":                   {partners, leaf(caA, partnerAFQDN, "sepp1.5gc.mnc099.mcc099.3gppnetwork.org"), "", TLSReasonPLMNMismatch},
		"expired":                        {partners, []*x509.Certificate{expired, caA.cert}, "", TLSReasonExpired},
		"server-only EKU as client":      {partners, []*x509.Certificate{serverOnly, caA.cert}, "", TLSReasonUnknownAnchor},
		"two partners matching":          {[]*trustedPartner{a, mustTrusted(t, "a2", partnerAFQDN, caA.pem(), plmnA)}, leaf(caA, partnerAFQDN), "", TLSReasonAmbiguous},
		"no partner with a 5G agreement": {nil, leaf(caA, partnerAFQDN), "", TLSReasonUnknownAnchor},
	} {
		got, err := identifyPeer(tc.partners, tc.certs, time.Now(), x509.ExtKeyUsageClientAuth)

		switch {
		case tc.want != "" && (err != nil || got.id != tc.want):
			t.Errorf("%s: got %v, %v, want %s", name, got, err, tc.want)
		case tc.want == "" && (err == nil || TLSFailureReason(err) != tc.reason || !errors.Is(err, ErrPeerCertificate)):
			t.Errorf("%s: got %v, want reason %s", name, err, tc.reason)
		}
	}
}

func TestServerName(t *testing.T) {
	for host, want := range map[string]string{
		"sepp.partner.example": "sepp.partner.example",
		"192.0.2.1":            partnerAFQDN,
		"[2001:db8::1]":        partnerAFQDN,
		"2001:db8::1":          partnerAFQDN,
		"":                     partnerAFQDN,
	} {
		if got := ServerName(host, partnerAFQDN); got != want {
			t.Errorf("ServerName(%q) = %q, want %q", host, got, want)
		}
	}
}

type tlsFixture struct {
	ellaCA, partnerCA testCA
	m                 *Manager
	addr              string
}

func newTLSFixture(t *testing.T) *tlsFixture {
	t.Helper()

	f := &tlsFixture{ellaCA: newTestCA(t, "ella"), partnerCA: newTestCA(t, "partner")}
	identity := identityFor(t, f.ellaCA, ownFQDN)

	f.m = New(
		Config{
			Partners: func(context.Context) ([]PartnerConfig, error) {
				return []PartnerConfig{{
					ID: "a", Name: "A", PLMNs: []wire.PlmnID{{Mcc: "001", Mnc: "01"}}, FQDN: partnerAFQDN,
					N32cURI: "https://127.0.0.1:1", TrustAnchorPEM: f.partnerCA.pem(),
				}}, nil
			},
			Identity: func(context.Context) (*IdentityConfig, error) { return &identity, nil },
			Operator: func(context.Context) (wire.PlmnID, error) { return ownPLMN, nil },
			N32:      config.N32Interface{Enabled: true, Address: "127.0.0.1"},
		},
		zap.NewNop(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	f.m.Reconcile(ctx)
	t.Cleanup(f.m.stopListener)

	addr := f.m.ListenAddr()
	if addr == nil {
		t.Fatalf("listener not started: %+v", f.m.ListenerStatus())
	}

	f.addr = addr.String()

	return f
}

func (f *tlsFixture) clientCert(t *testing.T, ca testCA, names ...string) []tls.Certificate {
	leaf, key := ca.issue(t, names)
	return []tls.Certificate{tlsCert(t, ca, leaf, key)}
}

func (f *tlsFixture) request(t *testing.T, cfg *tls.Config) (int, error) {
	t.Helper()

	if cfg.RootCAs == nil && !cfg.InsecureSkipVerify {
		pool := x509.NewCertPool()
		pool.AddCert(f.ellaCA.cert)
		cfg.RootCAs = pool
	}

	if cfg.ServerName == "" {
		cfg.ServerName = ownFQDN
	}

	protocols := new(http.Protocols)
	protocols.SetHTTP2(true)

	tr := &http.Transport{TLSClientConfig: cfg, Protocols: protocols}
	defer tr.CloseIdleConnections()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+f.addr+N32cExchangeCapabilityPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		return 0, err
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	return resp.StatusCode, nil
}

func TestListenerTLS(t *testing.T) {
	f := newTLSFixture(t)

	good := func() *tls.Config {
		return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: f.clientCert(t, f.partnerCA, partnerAFQDN), NextProtos: []string{ALPNH2}}
	}

	for _, v := range []uint16{tls.VersionTLS12, tls.VersionTLS13} {
		cfg := good()
		cfg.MinVersion, cfg.MaxVersion = v, v

		if status, err := f.request(t, cfg); err != nil || status != http.StatusMethodNotAllowed {
			t.Fatalf("TLS %x: status %d, %v", v, status, err)
		}
	}

	cfg := good()
	cfg.MaxVersion = tls.VersionTLS12
	cfg.CipherSuites = []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA}

	if _, err := f.request(t, cfg); err == nil {
		t.Fatal("CBC cipher suite accepted")
	}

	for name, mutate := range map[string]func(*tls.Config){
		"no client certificate": func(c *tls.Config) { c.Certificates = nil },
		"wrong anchor":          func(c *tls.Config) { c.Certificates = f.clientCert(t, newTestCA(t, "rogue"), partnerAFQDN) },
		"no PLMN in SAN": func(c *tls.Config) {
			c.Certificates = f.clientCert(t, f.partnerCA, "sepp7.partner.example")
		},
		"other PLMN in SAN": func(c *tls.Config) {
			c.Certificates = f.clientCert(t, f.partnerCA, partnerAFQDN, partnerBFQDN)
		},
		"expired client certificate": func(c *tls.Config) {
			leaf, key := f.partnerCA.issue(t, []string{partnerAFQDN}, leafOpts{notBefore: time.Now().Add(-3 * time.Hour), notAfter: time.Now().Add(-time.Hour), eku: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
			c.Certificates = []tls.Certificate{tlsCert(t, f.partnerCA, leaf, key)}
		},
		"TLS 1.1": func(c *tls.Config) { c.MinVersion, c.MaxVersion = tls.VersionTLS11, tls.VersionTLS11 },
	} {
		cfg := good()
		mutate(cfg)

		if status, err := f.request(t, cfg); err == nil {
			t.Errorf("%s: accepted with status %d", name, status)
		}
	}

	other := good()
	other.Certificates = f.clientCert(t, f.partnerCA, "sepp7.5gc.mnc001.mcc001.3gppnetwork.org")

	if status, err := f.request(t, other); err != nil || status != http.StatusMethodNotAllowed {
		t.Errorf("another SEPP of the partner: status %d, %v", status, err)
	}

	if err := rawHandshake(f.addr, good(), []string{"http/1.1"}); err == nil {
		t.Error("ALPN http/1.1 accepted")
	}

	if err := rawHandshake(f.addr, good(), nil); err == nil {
		t.Error("no ALPN accepted")
	}

	if err := rawHandshake(f.addr, good(), []string{ALPNH2}); err != nil {
		t.Errorf("raw h2 handshake: %v", err)
	}
}

func rawHandshake(addr string, cfg *tls.Config, alpn []string) error {
	cfg.NextProtos = alpn
	cfg.InsecureSkipVerify = true

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := (&tls.Dialer{Config: cfg}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	if _, err := conn.Write([]byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n\x00\x00\x00\x04\x00\x00\x00\x00\x00")); err != nil {
		return err
	}

	buf := make([]byte, 9)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return err
	}

	if buf[3] != 0x04 {
		return errors.New("no SETTINGS frame from the server")
	}

	return nil
}

func TestListenerHotReloadsIdentity(t *testing.T) {
	f := newTLSFixture(t)

	cfg := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: f.clientCert(t, f.partnerCA, partnerAFQDN)}
	if _, err := f.request(t, cfg); err != nil {
		t.Fatal(err)
	}

	newCA := newTestCA(t, "ella-2")
	next := identityFor(t, newCA, "sepp2.5gc.mnc070.mcc999.3gppnetwork.org")
	f.m.cfg.Identity = func(context.Context) (*IdentityConfig, error) { return &next, nil }
	f.m.Reconcile(context.Background())

	if _, err := f.request(t, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: f.clientCert(t, f.partnerCA, partnerAFQDN)}); err == nil {
		t.Fatal("old trust anchor still accepted after rotation")
	}

	pool := x509.NewCertPool()
	pool.AddCert(newCA.cert)

	if status, err := f.request(t, &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool, ServerName: "sepp2.5gc.mnc070.mcc999.3gppnetwork.org", Certificates: f.clientCert(t, f.partnerCA, partnerAFQDN)}); err != nil || status != http.StatusMethodNotAllowed {
		t.Fatalf("new identity: %d, %v", status, err)
	}

	if f.m.LocalFQDN() != "sepp2.5gc.mnc070.mcc999.3gppnetwork.org" {
		t.Fatalf("FQDN = %s", f.m.LocalFQDN())
	}
}

func TestOutboundTLSVerification(t *testing.T) {
	ellaCA, partnerCA := newTestCA(t, "ella"), newTestCA(t, "partner")
	identity := identityFor(t, ellaCA, ownFQDN)

	m := New(Config{
		Partners: func(context.Context) ([]PartnerConfig, error) {
			return []PartnerConfig{{ID: "a", Name: "A", PLMNs: []wire.PlmnID{{Mcc: "001", Mnc: "01"}}, FQDN: partnerAFQDN, N32cURI: "https://127.0.0.1:1", TrustAnchorPEM: partnerCA.pem()}}, nil
		},
		Identity: func(context.Context) (*IdentityConfig, error) { return &identity, nil },
		Operator: func(context.Context) (wire.PlmnID, error) { return ownPLMN, nil },
		N32:      config.N32Interface{Enabled: true, Address: "127.0.0.1"},
	}, zap.NewNop())
	m.Reconcile(context.Background())
	t.Cleanup(m.stopListener)

	serve := func(cert tls.Certificate, alpn []string) (string, chan tls.ConnectionState) {
		ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
			NextProtos:   alpn,
			ClientAuth:   tls.RequireAnyClientCert,
		})
		if err != nil {
			t.Fatal(err)
		}

		t.Cleanup(func() { _ = ln.Close() })

		states := make(chan tls.ConnectionState, 1)

		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			tc, _ := conn.(*tls.Conn)
			if tc.HandshakeContext(context.Background()) == nil {
				states <- tc.ConnectionState()
			}

			_ = conn.Close()
		}()

		return ln.Addr().String(), states
	}

	dial := func(addr, host string) error {
		cfg, err := m.ClientTLSConfig("a", host)
		if err != nil {
			t.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		conn, err := (&tls.Dialer{Config: cfg}).DialContext(ctx, "tcp", addr)
		if err != nil {
			return err
		}

		return conn.Close()
	}

	leaf, key := partnerCA.issue(t, []string{partnerAFQDN})
	addr, states := serve(tlsCert(t, partnerCA, leaf, key), []string{ALPNH2})

	if err := dial(addr, "127.0.0.1"); err != nil {
		t.Fatalf("valid partner server: %v", err)
	}

	st := <-states
	if st.ServerName != partnerAFQDN || len(st.PeerCertificates) == 0 || st.PeerCertificates[0].DNSNames[0] != ownFQDN {
		t.Fatalf("SNI %q, client cert %v", st.ServerName, st.PeerCertificates)
	}

	addr, states = serve(tlsCert(t, partnerCA, leaf, key), []string{ALPNH2})
	if err := dial(addr, "sepp-n32.partner.example"); err != nil {
		t.Fatal(err)
	}

	if st := <-states; st.ServerName != "sepp-n32.partner.example" {
		t.Fatalf("SNI for a DNS host = %q", st.ServerName)
	}

	rogue := newTestCA(t, "rogue")
	rLeaf, rKey := rogue.issue(t, []string{partnerAFQDN})
	wLeaf, wKey := partnerCA.issue(t, []string{"sepp9.5gc.mnc001.mcc001.3gppnetwork.org"})

	for name, srv := range map[string]struct {
		cert tls.Certificate
		alpn []string
	}{
		"wrong anchor": {tlsCert(t, rogue, rLeaf, rKey), []string{ALPNH2}},
		"wrong SAN":    {tlsCert(t, partnerCA, wLeaf, wKey), []string{ALPNH2}},
		"no ALPN":      {tlsCert(t, partnerCA, leaf, key), nil},
	} {
		addr, _ := serve(srv.cert, srv.alpn)
		if err := dial(addr, "127.0.0.1"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	if _, err := m.ClientTLSConfig("unknown", ""); !errors.Is(err, ErrUnknownPartner) {
		t.Fatalf("unknown partner: %v", err)
	}
}
