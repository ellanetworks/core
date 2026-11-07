// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepptest

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/pki"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sepp"
	"go.uber.org/zap"
)

type CA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func NewCA(t testing.TB, name string) CA {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	return CA{cert: cert, key: key}
}

func (ca CA) PEM() string {
	return string(pki.EncodeCertPEM(ca.cert))
}

func (ca CA) Issue(t testing.TB, fqdn string) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: fqdn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(12 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     []string{fqdn},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}

	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	return tls.Certificate{Certificate: [][]byte{leaf.Raw, ca.cert.Raw}, PrivateKey: key, Leaf: leaf}
}

func (ca CA) Identity(t testing.TB, seppID int, fqdn string) sepp.IdentityConfig {
	t.Helper()

	cert := ca.Issue(t, fqdn)

	keyPEM, err := pki.EncodePrivateKeyPEM(cert.PrivateKey.(*ecdsa.PrivateKey))
	if err != nil {
		t.Fatal(err)
	}

	return sepp.IdentityConfig{SEPPID: seppID, FQDN: fqdn, CertChainPEM: string(pki.EncodeCertPEM(cert.Leaf)) + ca.PEM(), KeyPEM: string(keyPEM)}
}

type Partner struct {
	FQDN string
	PLMN wire.PlmnID
	CA   CA

	cert tls.Certificate
	srv  *httptest.Server
	home http.Handler
}

func NewPartner(t testing.TB, plmn wire.PlmnID, home http.Handler) *Partner {
	t.Helper()

	p := &Partner{FQDN: plmn.NFFQDN("sepp1"), PLMN: plmn, CA: NewCA(t, "partner"), home: home}
	p.cert = p.CA.Issue(t, p.FQDN)

	p.srv = httptest.NewUnstartedServer(http.HandlerFunc(p.serve))
	p.srv.EnableHTTP2 = true
	p.srv.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{p.cert}, ClientAuth: tls.RequireAnyClientCert}
	p.srv.StartTLS()
	t.Cleanup(p.srv.Close)

	return p
}

func (p *Partner) Config(id string) sepp.PartnerConfig {
	return sepp.PartnerConfig{
		ID:             id,
		Name:           id,
		PLMNs:          []wire.PlmnID{p.PLMN},
		FQDN:           p.FQDN,
		N32cURI:        p.srv.URL,
		TrustAnchorPEM: p.CA.PEM(),
		NRFAPIRoots:    map[wire.PlmnID]string{p.PLMN: "http://" + p.PLMN.NFFQDN("nrf")},
	}
}

func (p *Partner) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	if r.URL.Path == sepp.N32cExchangeCapabilityPath {
		if _, err := wire.Decode[sepp.SecNegotiateReqData](body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		rsp := sepp.SecNegotiateRspData{Sender: p.FQDN, SelectedSecCapability: sepp.SecurityTLS, TargetAPIRootSupported: true, PlmnIDList: []wire.PlmnID{p.PLMN}}

		w.Header().Set("Content-Type", wire.ContentTypeJSON)
		_ = json.NewEncoder(w).Encode(rsp)

		return
	}

	root, err := url.Parse(r.Header.Get(sepp.HeaderTargetAPIRoot))
	if err != nil || root.Host == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	in := r.Clone(r.Context())
	in.Host = root.Host
	in.URL.Scheme = root.Scheme
	in.URL.Host = root.Host
	in.Body = io.NopCloser(bytes.NewReader(body))
	in.Header.Del(sepp.HeaderTargetAPIRoot)

	p.home.ServeHTTP(w, in)
}

func (p *Partner) Notify(ctx context.Context, ellaN32 string, target string, body any) (*http.Response, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ellaN32+u.EscapedPath(), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", wire.ContentTypeJSON)
	req.Header.Set(sepp.HeaderTargetAPIRoot, u.Scheme+"://"+u.Host)

	client := &http.Client{Transport: &http.Transport{ForceAttemptHTTP2: true, TLSClientConfig: &tls.Config{
		MinVersion:         tls.VersionTLS12,
		NextProtos:         []string{"h2"},
		Certificates:       []tls.Certificate{p.cert},
		InsecureSkipVerify: true, // #nosec G402 -- test client for a self-signed test SEPP
	}}}

	return client.Do(req)
}

type Ella struct {
	Manager *sepp.Manager
	FQDN    string
	SEPPID  int
}

func NewElla(t testing.TB, op wire.PlmnID, seppID int, partners ...sepp.PartnerConfig) *Ella {
	t.Helper()

	fqdn := op.NFFQDN("sepp1")
	identity := NewCA(t, "ella").Identity(t, seppID, fqdn)

	m := sepp.New(sepp.Config{
		Partners: func(context.Context) ([]sepp.PartnerConfig, error) { return partners, nil },
		Identity: func(context.Context) (*sepp.IdentityConfig, error) { return &identity, nil },
		Operator: func(context.Context) (wire.PlmnID, error) { return op, nil },
		N32:      config.N32Interface{Enabled: true, Address: "127.0.0.1"},
	}, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	wakeup := make(chan struct{})

	go func() {
		defer close(done)

		m.Run(ctx, wakeup)
	}()

	t.Cleanup(func() {
		cancel()
		<-done
	})

	for _, p := range partners {
		if err := m.WaitEstablished(ctx, p.ID, 10*time.Second); err != nil {
			t.Fatalf("N32 context with %s: %v", p.ID, err)
		}
	}

	return &Ella{Manager: m, FQDN: fqdn, SEPPID: seppID}
}

func (e *Ella) URL() string {
	return "https://" + e.Manager.ListenAddr().String()
}
