// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

type n32fPeer struct {
	t          *testing.T
	fqdn       string
	ca         testCA
	cert       tls.Certificate
	srv        *httptest.Server
	handshakes atomic.Int32
	closed     atomic.Int32
	accept     atomic.Bool
	redirect   atomic.Pointer[[2]string]

	mu      sync.Mutex
	got     []*http.Request
	bodies  [][]byte
	handler http.HandlerFunc
}

func newN32fPeer(t *testing.T, fqdn string, ca testCA) *n32fPeer {
	t.Helper()

	p := &n32fPeer{t: t, fqdn: fqdn, ca: ca}
	p.accept.Store(true)

	leaf, key := ca.issue(t, []string{fqdn})
	p.cert = tlsCert(t, ca, leaf, key)

	p.srv = httptest.NewUnstartedServer(http.HandlerFunc(p.serve))
	p.srv.EnableHTTP2 = true
	p.srv.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{p.cert}, ClientAuth: tls.RequireAnyClientCert}
	p.srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateClosed {
			p.closed.Add(1)
		}
	}
	p.srv.StartTLS()
	t.Cleanup(p.srv.Close)

	return p
}

func (p *n32fPeer) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	if r.URL.Path == N32cExchangeCapabilityPath {
		p.handshakes.Add(1)

		if r := p.redirect.Load(); r != nil {
			if r[0] != "" {
				w.Header().Set("Location", r[0])
			}

			w.WriteHeader(http.StatusTemporaryRedirect)
			_, _ = w.Write([]byte(r[1]))

			return
		}

		req, err := wire.Decode[SecNegotiateReqData](body)
		if err != nil || !p.accept.Load() {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		rsp := SecNegotiateRspData{Sender: p.fqdn, SelectedSecCapability: SecurityTLS, TargetAPIRootSupported: true, PlmnIDList: []wire.PlmnID{yPLMN}, SupportedFeatures: "1"}
		if req.Teardown() {
			rsp = SecNegotiateRspData{Sender: p.fqdn, SelectedSecCapability: SecurityNone}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rsp)

		return
	}

	p.mu.Lock()
	p.got = append(p.got, r)
	p.bodies = append(p.bodies, body)
	h := p.handler
	p.mu.Unlock()

	if h == nil {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))

		return
	}

	h(w, r)
}

func (p *n32fPeer) handle(h http.HandlerFunc) {
	p.mu.Lock()
	p.handler = h
	p.mu.Unlock()
}

func (p *n32fPeer) last() (*http.Request, []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.got) == 0 {
		p.t.Fatal("the peer received no N32-f request")
	}

	return p.got[len(p.got)-1], p.bodies[len(p.bodies)-1]
}

func (p *n32fPeer) partnerConfig() PartnerConfig {
	return PartnerConfig{ID: "p", Name: "p", PLMNs: []wire.PlmnID{yPLMN}, FQDN: p.fqdn, N32cURI: p.srv.URL, TrustAnchorPEM: p.ca.pem(), NRFAPIRoots: map[wire.PlmnID]string{yPLMN: "http://nrf.5gc.mnc001.mcc001.3gppnetwork.org"}}
}

func n32fFixture(t *testing.T, tuning Tuning) (*node, *n32fPeer) {
	t.Helper()

	p := newN32fPeer(t, yFQDN, newTestCA(t, "partner"))
	x := newNode(t, xFQDN, xPLMN, tuning)
	x.setPartners(p.partnerConfig())
	x.start()
	x.waitState("p", StateEstablished)

	return x, p
}

func get(t *testing.T, c *http.Client, target string, header http.Header) (int, []byte, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}

	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)

	return resp.StatusCode, body, err
}

func TestForwarderRewritesAndSetsHeaders(t *testing.T) {
	x, p := n32fFixture(t, fastTuning())
	client := x.m.Client("p")

	header := http.Header{
		"Traceparent":              {"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"},
		"Tracestate":               {"x=1"},
		HeaderTargetAPIRoot:        {"https://evil.example.org"},
		HeaderOriginatingNetworkID: {"001-01"},
		"User-Agent":               {"AMF"},
	}

	status, body, err := get(t, client, "http://nrf.5gc.mnc001.mcc001.3gppnetwork.org/nnrf-disc/v1/nf-instances?target-nf-type=AUSF&requester-nf-type=AMF", header)
	if err != nil || status != http.StatusOK || string(body) != `{"ok":true}` {
		t.Fatalf("forward = %d, %v, %s", status, err, body)
	}

	r, _ := p.last()

	checks := map[string][2]string{
		"proto":                    {r.Proto, "HTTP/2.0"},
		"path":                     {r.URL.Path, "/nnrf-disc/v1/nf-instances"},
		"query":                    {r.URL.RawQuery, "target-nf-type=AUSF&requester-nf-type=AMF"},
		"authority":                {r.Host, strings.TrimPrefix(p.srv.URL, "https://")},
		HeaderTargetAPIRoot:        {r.Header.Get(HeaderTargetAPIRoot), "http://nrf.5gc.mnc001.mcc001.3gppnetwork.org"},
		HeaderOriginatingNetworkID: {r.Header.Get(HeaderOriginatingNetworkID), "999-70"},
		HeaderVia:                  {r.Header.Get(HeaderVia), "2.0 SEPP-" + xFQDN},
		"traceparent":              {r.Header.Get("Traceparent"), ""},
		"tracestate":               {r.Header.Get("Tracestate"), ""},
		"user-agent":               {r.Header.Get("User-Agent"), "AMF"},
	}

	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}

	if len(r.Header.Values(HeaderTargetAPIRoot)) != 1 {
		t.Errorf("Target-apiRoot values = %v", r.Header.Values(HeaderTargetAPIRoot))
	}

	if _, _, err := get(t, client, "http://ausf.5gc.mnc001.mcc001.3gppnetwork.org:8080/pfx/nausf-auth/v1/ue-authentications/1", nil); err != nil {
		t.Fatal(err)
	}

	r, _ = p.last()
	if r.URL.Path != "/pfx/nausf-auth/v1/ue-authentications/1" || r.Header.Get(HeaderTargetAPIRoot) != "http://ausf.5gc.mnc001.mcc001.3gppnetwork.org:8080" {
		t.Fatalf("absolute href: path %q, Target-apiRoot %q", r.URL.Path, r.Header.Get(HeaderTargetAPIRoot))
	}

	payload := []byte(`{"supiOrSuci":"imsi-001010000000001"}`)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://ausf.5gc.mnc001.mcc001.3gppnetwork.org/nausf-auth/v1/ue-authentications", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	_ = resp.Body.Close()

	if _, got := p.last(); !bytes.Equal(got, payload) {
		t.Fatalf("body = %s", got)
	}
}

func TestForwarderRefusals(t *testing.T) {
	x, _ := n32fFixture(t, fastTuning())
	client := x.m.Client("p")

	for _, target := range []string{
		"http://udm.example.org/nudm-sdm/v2/imsi-001010000000001",
		"http://udm.5gc.mnc002.mcc001.3gppnetwork.org/nudm-sdm/v2/x",
		"http://udm.5gc.mnc070.mcc999.3gppnetwork.org/nudm-sdm/v2/x",
		"http://10.0.0.1/nudm-sdm/v2/x",
	} {
		if _, _, err := get(t, client, target, nil); !errors.Is(err, ErrTargetNotAllowed) {
			t.Errorf("%s: err = %v, want ErrTargetNotAllowed", target, err)
		}
	}

	if _, _, err := get(t, x.m.Client("nope"), "http://udm.5gc.mnc001.mcc001.3gppnetwork.org/x", nil); !errors.Is(err, ErrUnknownPartner) {
		t.Errorf("unknown partner: %v", err)
	}

	cfg := x.partners[0]
	cfg.N32fURI = "https://127.0.0.1:1"
	x.setPartners(cfg)
	x.waitState("p", StateEstablished)

	if _, _, err := get(t, client, "http://udm.5gc.mnc001.mcc001.3gppnetwork.org/x", nil); !errors.Is(err, ErrUnreachable) {
		t.Errorf("unreachable N32-f endpoint: %v", err)
	}
}

func TestForwarderNeedsAnEstablishedContext(t *testing.T) {
	p := newN32fPeer(t, yFQDN, newTestCA(t, "partner"))
	p.accept.Store(false)

	x := newNode(t, xFQDN, xPLMN, fastTuning())
	x.setPartners(p.partnerConfig())
	x.start()
	x.waitState("p", StateFailed)

	if _, _, err := get(t, x.m.Client("p"), "http://udm.5gc.mnc001.mcc001.3gppnetwork.org/x", nil); !errors.Is(err, ErrNoContext) {
		t.Fatalf("err = %v, want ErrNoContext", err)
	}

	off := newNode(t, xFQDN, xPLMN, fastTuning())
	off.m.cfg.N32.Enabled = false
	off.setPartners(p.partnerConfig())

	if _, _, err := get(t, off.m.Client("p"), "http://udm.5gc.mnc001.mcc001.3gppnetwork.org/x", nil); !errors.Is(err, ErrNoContext) {
		t.Fatalf("node without interfaces.n32: err = %v, want ErrNoContext", err)
	}
}

func TestContextNotFoundTriggersRehandshake(t *testing.T) {
	x, p := n32fFixture(t, fastTuning())

	p.handle(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", wire.ContentTypeProblem)
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(wire.NewProblem(http.StatusForbidden, wire.CauseContextNotFound, ""))
	})

	forward := func() {
		t.Helper()

		status, _, err := get(t, x.m.Client("p"), "http://udm.5gc.mnc001.mcc001.3gppnetwork.org/x", nil)
		if err != nil || status != http.StatusForbidden {
			t.Fatalf("forward = %d, %v", status, err)
		}
	}

	before := p.handshakes.Load()

	forward()
	eventually(t, "re-handshake after CONTEXT_NOT_FOUND", func() bool { return p.handshakes.Load() == before+1 })
	x.waitState("p", StateEstablished)

	forward()
	time.Sleep(100 * time.Millisecond)

	if got := p.handshakes.Load(); got != before+1 {
		t.Fatalf("%d re-handshakes within the debounce delay, want 1", got-before)
	}

	time.Sleep(200 * time.Millisecond)

	forward()
	eventually(t, "re-handshake after the debounce delay", func() bool { return p.handshakes.Load() == before+2 })
	x.waitState("p", StateEstablished)
}

func TestStateChangeClosesN32fConnections(t *testing.T) {
	x, p := n32fFixture(t, fastTuning())

	if _, _, err := get(t, x.m.Client("p"), "http://udm.5gc.mnc001.mcc001.3gppnetwork.org/x", nil); err != nil {
		t.Fatal(err)
	}

	closedBefore := p.closed.Load()

	if _, err := x.m.Rehandshake("p"); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the pooled N32-f connection closed", func() bool { return p.closed.Load() > closedBefore })

	x.waitState("p", StateEstablished)

	if _, _, err := get(t, x.m.Client("p"), "http://udm.5gc.mnc001.mcc001.3gppnetwork.org/x", nil); err != nil {
		t.Fatalf("forward after re-handshake: %v", err)
	}
}

func TestForwarderDoesNotFollowN32fRedirects(t *testing.T) {
	x, p := n32fFixture(t, fastTuning())

	p.handle(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://sepp2.5gc.mnc001.mcc001.3gppnetwork.org/x")
		w.WriteHeader(http.StatusTemporaryRedirect)
	})

	status, _, err := get(t, x.m.Client("p"), "http://udm.5gc.mnc001.mcc001.3gppnetwork.org/x", nil)
	if err != nil || status != http.StatusTemporaryRedirect {
		t.Fatalf("forward = %d, %v; want the 307 passed through", status, err)
	}
}

func redirectingPeers(t *testing.T) (*node, *n32fPeer, *n32fPeer, map[string]string) {
	t.Helper()

	ca := newTestCA(t, "partner")
	p := newN32fPeer(t, yFQDN, ca)
	p2 := newN32fPeer(t, "sepp2.5gc.mnc001.mcc001.3gppnetwork.org", ca)
	outsider := newN32fPeer(t, "sepp3.5gc.mnc001.mcc001.3gppnetwork.org", newTestCA(t, "other"))

	ports := map[string]string{}

	for _, peer := range []*n32fPeer{p2, outsider} {
		_, port, _ := net.SplitHostPort(strings.TrimPrefix(peer.srv.URL, "https://"))
		ports[peer.fqdn] = port
	}

	x := newNode(t, xFQDN, xPLMN, fastTuning())
	x.m.dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		if strings.HasSuffix(host, ".3gppnetwork.org") {
			host = "127.0.0.1"
		}

		return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(host, port))
	}

	return x, p, p2, ports
}

func TestHandshakeFollowsPartnerRedirect(t *testing.T) {
	x, p, p2, ports := redirectingPeers(t)

	p.redirect.Store(&[2]string{"https://" + p2.fqdn + ":" + ports[p2.fqdn] + N32cExchangeCapabilityPath, ""})

	x.setPartners(p.partnerConfig())
	x.start()

	st := x.waitState("p", StateEstablished)
	if st.Peer != p2.fqdn || p2.handshakes.Load() != 1 {
		t.Fatalf("context = %+v, %d handshakes on the target", st, p2.handshakes.Load())
	}

	if _, _, err := get(t, x.m.Client("p"), "http://udm.5gc.mnc001.mcc001.3gppnetwork.org/nudm-sdm/v2/imsi-001010000000001", nil); err != nil {
		t.Fatal(err)
	}

	if r, _ := p2.last(); r.URL.Path != "/nudm-sdm/v2/imsi-001010000000001" {
		t.Fatalf("N32-f after the redirect went to %s", r.URL.Path)
	}

	p.redirect.Store(nil)

	before := p.handshakes.Load()

	if _, err := x.m.Rehandshake("p"); err != nil {
		t.Fatal(err)
	}

	eventually(t, "re-handshake with the configured SEPP", func() bool {
		st := x.state("p")
		return st.State == StateEstablished && st.Peer == "" && p.handshakes.Load() > before
	})
}

func TestHandshakeRedirectRefusals(t *testing.T) {
	for name, tc := range map[string]func(ports map[string]string, p2 *n32fPeer) [2]string{
		"other CA": func(ports map[string]string, _ *n32fPeer) [2]string {
			return [2]string{"https://sepp3.5gc.mnc001.mcc001.3gppnetwork.org:" + ports["sepp3.5gc.mnc001.mcc001.3gppnetwork.org"], ""}
		},
		"other network": func(ports map[string]string, p2 *n32fPeer) [2]string {
			return [2]string{"https://sepp2.5gc.mnc070.mcc999.3gppnetwork.org:" + ports[p2.fqdn], ""}
		},
		"http": func(ports map[string]string, p2 *n32fPeer) [2]string {
			return [2]string{"http://" + p2.fqdn + ":" + ports[p2.fqdn], ""}
		},
		"no Location": func(map[string]string, *n32fPeer) [2]string { return [2]string{"", ""} },
		"with discovery": func(ports map[string]string, p2 *n32fPeer) [2]string {
			return [2]string{"https://" + p2.fqdn + ":" + ports[p2.fqdn], `{"cause":"SEPP_REDIRECTION_WITH_DISCOVERY","seppFqdnForDiscovery":"sepp.5gc.mnc001.mcc001.3gppnetwork.org"}`}
		},
	} {
		t.Run(name, func(t *testing.T) {
			x, p, p2, ports := redirectingPeers(t)

			r := tc(ports, p2)
			p.redirect.Store(&r)

			x.setPartners(p.partnerConfig())
			x.start()

			if st := x.waitState("p", StateFailed); st.LastError == "" {
				t.Fatalf("context = %+v", st)
			}
		})
	}
}
