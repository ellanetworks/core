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
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func fastTuning() Tuning {
	t := DefaultTuning()
	t.HandshakeTimeout = 2 * time.Second
	t.OngoingWait = 300 * time.Millisecond
	t.BackoffMin = 100 * time.Millisecond
	t.BackoffMax = 400 * time.Millisecond
	t.ContextLostDelay = 200 * time.Millisecond
	t.N32cRequestsPerMinute = 1000

	return t
}

func eventually(t *testing.T, msg string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", msg)
}

type node struct {
	t        *testing.T
	ca       testCA
	fqdn     string
	op       wire.PlmnID
	m        *Manager
	ctx      context.Context
	mu       sync.Mutex
	identity IdentityConfig
	partners []PartnerConfig
	cluster  []int
	run      context.CancelFunc
}

func newNode(t *testing.T, fqdn string, op wire.PlmnID, tuning Tuning) *node {
	t.Helper()

	n := &node{t: t, ca: newTestCA(t, fqdn), fqdn: fqdn, op: op}
	n.identity = identityFor(t, n.ca, fqdn)

	ctx, cancel := context.WithCancel(context.Background())
	n.ctx = ctx

	n.m = New(
		Config{
			Partners: func(context.Context) ([]PartnerConfig, error) {
				n.mu.Lock()
				defer n.mu.Unlock()

				return append([]PartnerConfig(nil), n.partners...), nil
			},
			Identity: func(context.Context) (*IdentityConfig, error) {
				n.mu.Lock()
				defer n.mu.Unlock()

				id := n.identity

				return &id, nil
			},
			Operator: func(context.Context) (wire.PlmnID, error) { return op, nil },
			Cluster: func(context.Context) ([]int, error) {
				n.mu.Lock()
				defer n.mu.Unlock()

				return slices.Clone(n.cluster), nil
			},
			N32: config.N32Interface{Enabled: true, Address: "127.0.0.1"},
		},
		zap.NewNop(),
		WithTuning(tuning),
	)

	n.m.Reconcile(ctx)

	t.Cleanup(func() {
		if n.run != nil {
			n.run()
		}

		cancel()
		n.m.stopListener()
	})

	return n
}

func (n *node) url() string {
	return "https://" + n.m.ListenAddr().String()
}

func (n *node) partnerConfig(id string) PartnerConfig {
	return PartnerConfig{ID: id, Name: id, PLMNs: []wire.PlmnID{n.op}, FQDN: n.fqdn, N32cURI: n.url(), TrustAnchorPEM: n.ca.pem()}
}

func (n *node) setPartners(ps ...PartnerConfig) {
	n.mu.Lock()
	n.partners = ps
	n.mu.Unlock()

	n.m.Reconcile(n.ctx)
}

func (n *node) setCluster(ids ...int) {
	n.mu.Lock()
	n.cluster = ids
	n.mu.Unlock()

	n.m.Reconcile(n.ctx)
}

func (n *node) start() {
	ctx, cancel := context.WithCancel(n.ctx)
	done := make(chan struct{})
	wakeup := make(chan struct{})

	go func() {
		defer close(done)

		n.m.Run(ctx, wakeup)
	}()

	n.run = func() {
		cancel()
		<-done
	}

	eventually(n.t, "SEPP running", func() bool {
		n.m.mu.Lock()
		defer n.m.mu.Unlock()

		return n.m.root != nil
	})
}

func (n *node) state(id string) ContextStatus {
	return n.m.Context(id)
}

func (n *node) waitState(id string, s State) ContextStatus {
	n.t.Helper()

	var st ContextStatus

	eventually(n.t, n.fqdn+" context "+id+" "+string(s), func() bool {
		st = n.state(id)
		return st.State == s
	})

	return st
}

const (
	xFQDN = "sepp1.5gc.mnc070.mcc999.3gppnetwork.org"
	yFQDN = "sepp1.5gc.mnc001.mcc001.3gppnetwork.org"
)

var (
	xPLMN = wire.PlmnID{Mcc: "999", Mnc: "70"}
	yPLMN = wire.PlmnID{Mcc: "001", Mnc: "01"}
)

func pair(t *testing.T, tx, ty Tuning) (*node, *node) {
	t.Helper()

	x := newNode(t, xFQDN, xPLMN, tx)
	y := newNode(t, yFQDN, yPLMN, ty)

	x.setPartners(y.partnerConfig("y"))
	y.setPartners(x.partnerConfig("x"))

	return x, y
}

func TestHandshakeBetweenTwoSEPPs(t *testing.T) {
	x, y := pair(t, fastTuning(), fastTuning())

	x.start()
	y.start()

	xs := x.waitState("y", StateEstablished)
	ys := y.waitState("x", StateEstablished)

	if !xs.NFTLST || !ys.NFTLST {
		t.Fatalf("NFTLST not agreed: %+v %+v", xs, ys)
	}

	if xs.Initiator == ys.Initiator {
		t.Fatalf("both sides report initiator %s", xs.Initiator)
	}

	for _, dir := range []struct{ from, to *node }{{x, y}, {y, x}} {
		fromID, toID := "y", "x"
		if dir.from == y {
			fromID, toID = "x", "y"
		}

		before := dir.to.state(toID).Since

		st, err := dir.from.m.Rehandshake(fromID)
		if err != nil || st.State != StateEstablishing {
			t.Fatalf("rehandshake %s: %+v, %v", dir.from.fqdn, st, err)
		}

		if got := dir.from.waitState(fromID, StateEstablished); got.Initiator != InitiatorLocal {
			t.Fatalf("%s initiator = %s", dir.from.fqdn, got.Initiator)
		}

		eventually(t, "responder replaced its context", func() bool {
			st := dir.to.state(toID)
			return st.State == StateEstablished && st.Initiator == InitiatorPeer && st.Since.After(before)
		})
	}

	if _, err := x.m.Rehandshake("nope"); !errors.Is(err, ErrUnknownPartner) {
		t.Fatalf("unknown partner: %v", err)
	}
}

func TestTeardownOnPartnerRemoval(t *testing.T) {
	slow := fastTuning()
	slow.BackoffMin, slow.BackoffMax = time.Minute, time.Minute

	x, y := pair(t, fastTuning(), slow)

	x.start()
	y.start()

	x.waitState("y", StateEstablished)
	y.waitState("x", StateEstablished)

	x.setPartners()

	st := y.waitState("x", StateTerminated)
	if !strings.Contains(st.LastError, "terminated") {
		t.Fatalf("lastError = %q", st.LastError)
	}

	if _, ok := x.m.Contexts()["y"]; ok {
		t.Fatal("removed partner still has a context")
	}
}

func TestNodesOfOneClusterHaveIndependentContexts(t *testing.T) {
	ca := newTestCA(t, "ella")

	var nodes []*node

	for _, fqdn := range []string{xFQDN, "sepp2.5gc.mnc070.mcc999.3gppnetwork.org", "sepp3.5gc.mnc070.mcc999.3gppnetwork.org"} {
		x := newNode(t, fqdn, xPLMN, fastTuning())

		x.mu.Lock()
		x.ca = ca
		x.identity = identityFor(t, ca, x.fqdn)
		x.mu.Unlock()
		x.m.Reconcile(x.ctx)

		nodes = append(nodes, x)
	}

	x1, x2, x3 := nodes[0], nodes[1], nodes[2]

	y := newNode(t, yFQDN, yPLMN, fastTuning())

	for _, x := range nodes {
		x.setPartners(y.partnerConfig("y"))
	}

	y.setPartners(x1.partnerConfig("x"))

	y.start()

	for _, x := range nodes {
		x.start()
		x.waitState("y", StateEstablished)
	}

	var ys ContextStatus

	eventually(t, "the partner has a context with every Ella SEPP", func() bool {
		ys = y.state("x")
		_, ok2 := otherPeer(ys, x2.fqdn)
		_, ok3 := otherPeer(ys, x3.fqdn)

		return ys.State == StateEstablished && ok2 && ok3
	})

	before2, _ := otherPeer(ys, x2.fqdn)
	before3, _ := otherPeer(ys, x3.fqdn)

	if _, err := x1.m.Rehandshake("y"); err != nil {
		t.Fatal(err)
	}

	x1.waitState("y", StateEstablished)

	time.Sleep(100 * time.Millisecond)

	after := y.state("x")
	after2, _ := otherPeer(after, x2.fqdn)
	after3, _ := otherPeer(after, x3.fqdn)

	if !after2.Since.Equal(before2.Since) || !after3.Since.Equal(before3.Since) {
		t.Fatalf("a handshake of node 1 changed the contexts of nodes 2 and 3: %+v, was %+v", after.OtherPeers, ys.OtherPeers)
	}

	x2.run()
	x2.run = nil

	time.Sleep(100 * time.Millisecond)

	if st := x1.state("y"); st.State != StateEstablished {
		t.Fatalf("stopping node 2 changed node 1's context: %+v", st)
	}

	x3.setPartners()

	eventually(t, "node 3's teardown", func() bool {
		_, ok := otherPeer(y.state("x"), x3.fqdn)
		return !ok
	})

	if st := y.state("x"); st.State != StateEstablished {
		t.Fatalf("node 3's teardown ended the context with node 1: %+v", st)
	}

	if x1.m.LocalFQDN() != xFQDN || x2.m.LocalFQDN() != x2.fqdn || x3.m.LocalFQDN() != x3.fqdn {
		t.Fatalf("local identities: %q %q %q", x1.m.LocalFQDN(), x2.m.LocalFQDN(), x3.m.LocalFQDN())
	}
}

func TestStopDropsContexts(t *testing.T) {
	x, y := pair(t, fastTuning(), fastTuning())

	x.start()
	y.start()

	x.waitState("y", StateEstablished)
	y.waitState("x", StateEstablished)

	x.run()
	x.run = nil

	if _, err := x.m.Rehandshake("y"); !errors.Is(err, ErrNotReady) {
		t.Fatalf("rehandshake after stop: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if st := y.state("x"); st.State != StateEstablished {
		t.Fatalf("stopping one SEPP tore the peer's context down: %+v", st)
	}
}

func TestRotationRehandshakes(t *testing.T) {
	x, y := pair(t, fastTuning(), fastTuning())

	newCA := newTestCA(t, "x-2")

	cfg := x.partnerConfig("x")
	cfg.TrustAnchorPEM += newCA.pem()
	y.setPartners(cfg)

	x.start()
	y.start()

	x.waitState("y", StateEstablished)
	before := y.waitState("x", StateEstablished).Since

	x.mu.Lock()
	x.identity = identityFor(t, newCA, xFQDN)
	x.mu.Unlock()
	x.m.Reconcile(x.ctx)

	eventually(t, "rotation re-handshake", func() bool {
		ys, xs := y.state("x"), x.state("y")
		return ys.State == StateEstablished && ys.Since.After(before) && xs.State == StateEstablished
	})
}

func TestIdentityProblemsMakeContextsIdle(t *testing.T) {
	x, y := pair(t, fastTuning(), fastTuning())

	x.mu.Lock()
	x.identity.FQDN = "sepp1.5gc.mnc001.mcc001.3gppnetwork.org"
	x.mu.Unlock()
	x.m.Reconcile(x.ctx)

	x.start()

	st := x.state("y")
	if st.State != StateIdle || st.LastError == "" {
		t.Fatalf("mismatched identity: %+v", st)
	}

	if x.m.Problem() == "" || x.m.ListenerStatus().Error == "" {
		t.Fatalf("problem not reported: %q %+v", x.m.Problem(), x.m.ListenerStatus())
	}

	x.mu.Lock()
	x.identity.FQDN = xFQDN
	x.mu.Unlock()
	x.m.Reconcile(x.ctx)

	y.start()
	x.waitState("y", StateEstablished)
}

type testPeer struct {
	t       *testing.T
	fqdn    string
	ca      testCA
	cert    tls.Certificate
	srv     *httptest.Server
	handler atomic.Pointer[func(http.ResponseWriter, *http.Request, SecNegotiateReqData)]
	got     chan SecNegotiateReqData
}

func newTestPeer(t *testing.T, fqdn string) *testPeer {
	t.Helper()

	p := &testPeer{t: t, fqdn: fqdn, ca: newTestCA(t, fqdn), got: make(chan SecNegotiateReqData, 16)}
	leaf, key := p.ca.issue(t, []string{fqdn})
	p.cert = tlsCert(t, p.ca, leaf, key)

	p.srv = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		req, err := wire.Decode[SecNegotiateReqData](body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		p.got <- req

		(*p.handler.Load())(w, r, req)
	}))
	p.srv.EnableHTTP2 = true
	p.srv.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{p.cert}, ClientAuth: tls.RequireAnyClientCert}
	p.srv.StartTLS()
	t.Cleanup(p.srv.Close)

	p.answer(func(w http.ResponseWriter, _ *http.Request, req SecNegotiateReqData) {
		p.reply(w, http.StatusOK, p.accept(req, "1"))
	})

	return p
}

func (p *testPeer) answer(h func(http.ResponseWriter, *http.Request, SecNegotiateReqData)) {
	p.handler.Store(&h)
}

func (p *testPeer) accept(req SecNegotiateReqData, features wire.SupportedFeatures) SecNegotiateRspData {
	if req.Teardown() {
		return SecNegotiateRspData{Sender: p.fqdn, SelectedSecCapability: SecurityNone}
	}

	return SecNegotiateRspData{Sender: p.fqdn, SelectedSecCapability: SecurityTLS, TargetAPIRootSupported: true, PlmnIDList: []wire.PlmnID{yPLMN}, SupportedFeatures: features}
}

func (p *testPeer) reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (p *testPeer) partnerConfig() PartnerConfig {
	return PartnerConfig{ID: "p", Name: "p", PLMNs: []wire.PlmnID{yPLMN}, FQDN: p.fqdn, N32cURI: p.srv.URL, TrustAnchorPEM: p.ca.pem()}
}

func (p *testPeer) post(x *node, body any) (int, []byte) {
	p.t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		p.t.Fatal(err)
	}

	return p.do(x, http.MethodPost, N32cExchangeCapabilityPath, data, nil)
}

func (p *testPeer) do(x *node, method, path string, data []byte, tr *http.Transport) (int, []byte) {
	p.t.Helper()

	if tr == nil {
		tr = p.transport(x)
		defer tr.CloseIdleConnections()
	}

	req, err := http.NewRequestWithContext(context.Background(), method, x.url()+path, bytes.NewReader(data))
	if err != nil {
		p.t.Fatal(err)
	}

	resp, err := (&http.Client{Transport: tr, Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		p.t.Fatal(err)
	}

	defer func() { _ = resp.Body.Close() }()

	rb, _ := io.ReadAll(resp.Body)

	if resp.Header.Get("Server") != "SEPP-"+x.fqdn {
		p.t.Errorf("Server header = %q", resp.Header.Get("Server"))
	}

	return resp.StatusCode, rb
}

func (p *testPeer) transport(x *node) *http.Transport {
	pool := x509.NewCertPool()
	pool.AddCert(x.ca.cert)

	protocols := new(http.Protocols)
	protocols.SetHTTP2(true)

	return &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool, ServerName: x.fqdn, Certificates: []tls.Certificate{p.cert}},
		Protocols:       protocols,
	}
}

func baseReq(sender string) SecNegotiateReqData {
	target := xPLMN

	return SecNegotiateReqData{
		Sender:                     sender,
		SupportedSecCapabilityList: []SecurityCapability{SecurityTLS},
		TargetAPIRootSupported:     true,
		PlmnIDList:                 []wire.PlmnID{yPLMN},
		TargetPlmnID:               &target,
		SupportedFeatures:          "1",
	}
}

func peerFixture(t *testing.T, fqdn string, tuning Tuning) (*node, *testPeer) {
	t.Helper()

	p := newTestPeer(t, fqdn)
	x := newNode(t, xFQDN, xPLMN, tuning)
	x.setPartners(p.partnerConfig())

	return x, p
}

func TestInitiatorOutcomes(t *testing.T) {
	for name, tc := range map[string]struct {
		handler   func(p *testPeer) func(http.ResponseWriter, *http.Request, SecNegotiateReqData)
		state     State
		lastError string
		nftlst    bool
		teardown  bool
	}{
		"established with NFTLST": {
			handler: func(p *testPeer) func(http.ResponseWriter, *http.Request, SecNegotiateReqData) {
				return func(w http.ResponseWriter, _ *http.Request, req SecNegotiateReqData) {
					p.reply(w, 200, p.accept(req, "1"))
				}
			},
			state: StateEstablished, nftlst: true,
		},
		"open5gs on-the-fly node answers features 0": {
			handler: func(p *testPeer) func(http.ResponseWriter, *http.Request, SecNegotiateReqData) {
				return func(w http.ResponseWriter, _ *http.Request, req SecNegotiateReqData) {
					p.reply(w, 200, p.accept(req, "0"))
				}
			},
			state: StateEstablished,
		},
		"403 negotiation not allowed": {
			handler: func(p *testPeer) func(http.ResponseWriter, *http.Request, SecNegotiateReqData) {
				return func(w http.ResponseWriter, _ *http.Request, _ SecNegotiateReqData) {
					p.reply(w, 403, wire.NewProblem(403, CauseNegotiationNotAllowed, "no"))
				}
			},
			state: StateFailed, lastError: "NEGOTIATION_NOT_ALLOWED",
		},
		"redirect outside the partner's network": {
			handler: func(*testPeer) func(http.ResponseWriter, *http.Request, SecNegotiateReqData) {
				return func(w http.ResponseWriter, _ *http.Request, _ SecNegotiateReqData) {
					w.Header().Set("Location", "https://sepp2.example.org/n32c-handshake/v1/exchange-capability")
					w.WriteHeader(http.StatusTemporaryRedirect)
				}
			},
			state: StateFailed, lastError: "outside the network",
		},
		"permanent redirect": {
			handler: func(*testPeer) func(http.ResponseWriter, *http.Request, SecNegotiateReqData) {
				return func(w http.ResponseWriter, _ *http.Request, _ SecNegotiateReqData) {
					w.Header().Set("Location", "https://sepp2.5gc.mnc001.mcc001.3gppnetwork.org")
					w.WriteHeader(http.StatusPermanentRedirect)
				}
			},
			state: StateFailed, lastError: "answered 308",
		},
		"wrong sender with NFTLST sends NONE first": {
			handler: func(p *testPeer) func(http.ResponseWriter, *http.Request, SecNegotiateReqData) {
				return func(w http.ResponseWriter, _ *http.Request, req SecNegotiateReqData) {
					rsp := p.accept(req, "1")
					if !req.Teardown() {
						rsp.Sender = "sepp7.5gc.mnc001.mcc001.3gppnetwork.org"
					}

					p.reply(w, 200, rsp)
				}
			},
			state: StateFailed, lastError: "sender", teardown: true,
		},
		"PRINS selected": {
			handler: func(p *testPeer) func(http.ResponseWriter, *http.Request, SecNegotiateReqData) {
				return func(w http.ResponseWriter, _ *http.Request, req SecNegotiateReqData) {
					rsp := p.accept(req, "1")
					rsp.SelectedSecCapability = SecurityPRINS
					p.reply(w, 200, rsp)
				}
			},
			state: StateFailed, lastError: "PRINS",
		},
		"open5gs sender mismatch answers nothing": {
			handler: func(*testPeer) func(http.ResponseWriter, *http.Request, SecNegotiateReqData) {
				return func(_ http.ResponseWriter, r *http.Request, _ SecNegotiateReqData) { <-r.Context().Done() }
			},
			state: StateFailed, lastError: "deadline",
		},
		"409 ongoing without a peer request": {
			handler: func(p *testPeer) func(http.ResponseWriter, *http.Request, SecNegotiateReqData) {
				return func(w http.ResponseWriter, _ *http.Request, _ SecNegotiateReqData) {
					p.reply(w, 409, wire.NewProblem(409, CauseN32cExchangeCapabilityOngoing, ""))
				}
			},
			state: StateFailed, lastError: "sent no exchange-capability",
		},
	} {
		t.Run(name, func(t *testing.T) {
			tuning := fastTuning()
			tuning.HandshakeTimeout = 500 * time.Millisecond
			tuning.BackoffMin, tuning.BackoffMax = time.Minute, time.Minute

			p := newTestPeer(t, yFQDN)
			p.answer(tc.handler(p))

			x := newNode(t, xFQDN, xPLMN, tuning)
			x.setPartners(p.partnerConfig())
			x.start()

			req := <-p.got
			if b, _ := wire.Encode(req); !strings.Contains(string(b), `"supportedSecCapabilityList":["TLS"],"3GppSbiTargetApiRootSupported":true,"plmnIdList":[{"mcc":"999","mnc":"70"}],"targetPlmnId":{"mcc":"001","mnc":"01"},"supportedFeatures":"1"`) {
				t.Fatalf("request = %s", b)
			}

			st := x.waitState("p", tc.state)
			if !strings.Contains(st.LastError, tc.lastError) || st.NFTLST != tc.nftlst {
				t.Fatalf("status = %+v", st)
			}

			if tc.state == StateEstablished && st.Initiator != InitiatorLocal {
				t.Fatalf("initiator = %s", st.Initiator)
			}

			if tc.teardown {
				select {
				case td := <-p.got:
					if !td.Teardown() {
						t.Fatalf("second request is not a teardown: %+v", td)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("no teardown sent")
				}
			}
		})
	}
}

func TestCollision(t *testing.T) {
	for name, tc := range map[string]struct {
		peerFQDN  string
		initiator Initiator
		inbound   int
	}{
		"peer precedes: Ella yields":   {yFQDN, InitiatorPeer, http.StatusOK},
		"Ella precedes: peer gets 409": {"sepp9.5gc.mnc001.mcc001.3gppnetwork.org", InitiatorLocal, http.StatusConflict},
	} {
		t.Run(name, func(t *testing.T) {
			x, p := peerFixture(t, tc.peerFQDN, fastTuning())

			var inbound atomic.Int32

			release := make(chan struct{})

			p.answer(func(w http.ResponseWriter, _ *http.Request, req SecNegotiateReqData) {
				if req.Teardown() {
					p.reply(w, 200, p.accept(req, "1"))
					return
				}

				body := baseReq(strings.ToUpper(p.fqdn))
				status, _ := p.post(x, body)
				inbound.Store(int32(status))

				if status == http.StatusOK {
					p.reply(w, 409, wire.NewProblem(409, CauseN32cExchangeCapabilityOngoing, ""))
				} else {
					rsp := p.accept(req, "1")
					rsp.Sender = p.fqdn
					p.reply(w, 200, rsp)
				}

				close(release)
			})

			x.start()
			<-release

			if got := int(inbound.Load()); got != tc.inbound {
				t.Fatalf("inbound answered %d, want %d", got, tc.inbound)
			}

			x.waitState("p", StateEstablished)
			time.Sleep(100 * time.Millisecond)

			if st := x.state("p"); st.State != StateEstablished || st.Initiator != tc.initiator {
				t.Fatalf("after collision: %+v", st)
			}
		})
	}
}

func TestResponderChecks(t *testing.T) {
	tuning := fastTuning()
	tuning.BackoffMin, tuning.BackoffMax = time.Minute, time.Minute

	x, p := peerFixture(t, yFQDN, tuning)
	p.answer(func(w http.ResponseWriter, _ *http.Request, _ SecNegotiateReqData) {
		p.reply(w, 403, wire.NewProblem(403, CauseNegotiationNotAllowed, "responder only"))
	})
	x.start()
	x.waitState("p", StateFailed)

	none := baseReq(yFQDN)
	none.SupportedSecCapabilityList = []SecurityCapability{SecurityNone}

	if status, body := p.post(x, none); status != 200 || !strings.Contains(string(body), `"selectedSecCapability":"NONE"`) {
		t.Fatalf("NONE while failed: %d %s", status, body)
	}

	if st := x.state("p"); st.State != StateFailed {
		t.Fatalf("NONE changed a failed context: %+v", st)
	}

	for name, tc := range map[string]struct {
		mutate func(*SecNegotiateReqData)
		raw    string
		status int
		cause  wire.Cause
	}{
		"malformed JSON":       {raw: `{"sender":`, status: 400, cause: wire.CauseInvalidMsgFormat},
		"sender missing":       {raw: `{"supportedSecCapabilityList":["TLS"]}`, status: 400, cause: wire.CauseMandatoryIEMissing},
		"sender not partner":   {mutate: func(r *SecNegotiateReqData) { r.Sender = "sepp2.5gc.mnc001.mcc001.3gppnetwork.org" }, status: 403, cause: CauseNegotiationNotAllowed},
		"PRINS only":           {mutate: func(r *SecNegotiateReqData) { r.SupportedSecCapabilityList = []SecurityCapability{SecurityPRINS} }, status: 403, cause: CauseNegotiationNotAllowed},
		"no target apiRoot":    {mutate: func(r *SecNegotiateReqData) { r.TargetAPIRootSupported = false }, status: 403, cause: CauseNegotiationNotAllowed},
		"another PLMN in list": {mutate: func(r *SecNegotiateReqData) { r.PlmnIDList = append(r.PlmnIDList, wire.PlmnID{Mcc: "002", Mnc: "10"}) }, status: 403, cause: CauseNegotiationNotAllowed},
		"target not home":      {mutate: func(r *SecNegotiateReqData) { r.TargetPlmnID = &yPLMN }, status: 403, cause: CauseNegotiationNotAllowed},
		"SMS purpose only": {mutate: func(r *SecNegotiateReqData) {
			r.IntendedUsagePurpose = []IntendedN32Purpose{{UsagePurpose: PurposeSMSInterconnect}}
		}, status: 403, cause: wire.CauseRequestedPurposeNotAllowed},
	} {
		var (
			status int
			body   []byte
		)

		if tc.raw != "" {
			status, body = p.do(x, http.MethodPost, N32cExchangeCapabilityPath, []byte(tc.raw), nil)
		} else {
			req := baseReq(yFQDN)
			tc.mutate(&req)
			status, body = p.post(x, req)
		}

		pd := wire.ParseProblem(status, body)
		if status != tc.status || pd.Cause != tc.cause {
			t.Errorf("%s: %d %s", name, status, body)
		}
	}

	if st := x.state("p"); st.State != StateFailed {
		t.Fatalf("rejected requests changed the context: %+v", st)
	}

	req := baseReq(yFQDN)
	req.IntendedUsagePurpose = []IntendedN32Purpose{{UsagePurpose: PurposeRoaming}, {UsagePurpose: PurposeSMSInterconnect}}
	req.N32HandshakeID = "0123456789abcdef"

	status, body := p.post(x, req)

	rsp, err := wire.Decode[SecNegotiateRspData](body)
	if status != 200 || err != nil || rsp.Sender != xFQDN || rsp.SelectedSecCapability != SecurityTLS || !rsp.TargetAPIRootSupported ||
		rsp.SupportedFeatures != "1" || rsp.N32HandshakeID != "" || len(rsp.PlmnIDList) != 1 || rsp.PlmnIDList[0] != xPLMN ||
		len(rsp.AllowedUsagePurpose) != 1 || len(rsp.RejectedUsagePurpose) != 1 {
		t.Fatalf("accept: %d %s", status, body)
	}

	first := x.waitState("p", StateEstablished)
	if first.Initiator != InitiatorPeer || !first.NFTLST {
		t.Fatalf("established by peer: %+v", first)
	}

	time.Sleep(10 * time.Millisecond)

	req = baseReq(yFQDN)
	req.SupportedFeatures = ""

	if status, _ := p.post(x, req); status != 200 {
		t.Fatalf("replacement: %d", status)
	}

	if st := x.state("p"); st.State != StateEstablished || st.NFTLST || !st.Since.After(first.Since) {
		t.Fatalf("replacement while established: %+v", st)
	}

	if status, body := p.post(x, none); status != 200 || !strings.Contains(string(body), `"NONE"`) {
		t.Fatalf("NONE while established: %d %s", status, body)
	}

	if st := x.state("p"); st.State != StateTerminated {
		t.Fatalf("after NONE: %+v", st)
	}
}

func TestListenerRouting(t *testing.T) {
	tuning := fastTuning()
	tuning.N32cRequestsPerMinute = 1

	x, p := peerFixture(t, yFQDN, tuning)

	tr := p.transport(x)
	defer tr.CloseIdleConnections()

	if status, body := p.do(x, http.MethodGet, "/nausf-auth/v1/ue-authentications", nil, tr); status != 403 || wire.ParseProblem(status, body).Cause != wire.CauseContextNotFound {
		t.Fatalf("N32-f without a context: %d %s", status, body)
	}

	if status, _ := p.do(x, http.MethodPost, N32cExchangeCapabilityPath, []byte(`{}`), tr); status != http.StatusBadRequest {
		t.Fatalf("N32-c on an N32-f connection: %d", status)
	}

	if status, _ := p.do(x, http.MethodGet, N32cExchangeCapabilityPath, nil, nil); status != http.StatusMethodNotAllowed {
		t.Fatalf("GET exchange-capability: %d", status)
	}

	if status, body := p.do(x, http.MethodPost, N32cHandshakePath+"/n32f-terminate", []byte(`{}`), nil); status != 404 || wire.ParseProblem(status, body).Cause != wire.CauseResourceURIStructureNotFound {
		t.Fatalf("n32f-terminate: %d %s", status, body)
	}

	if status, _ := p.do(x, http.MethodPost, N32cExchangeCapabilityPath, bytes.Repeat([]byte("x"), maxBodyBytes+1), nil); status != http.StatusRequestEntityTooLarge {
		t.Fatalf("large body: %d", status)
	}

	status, body := p.post(x, baseReq(yFQDN))
	if status != http.StatusTooManyRequests || wire.ParseProblem(status, body).Cause != wire.CauseNFCongestionRisk {
		t.Fatalf("rate limit: %d %s", status, body)
	}
}

func TestMetrics(t *testing.T) {
	x, y := pair(t, fastTuning(), fastTuning())

	x.start()
	y.start()

	x.waitState("y", StateEstablished)

	reg := prometheus.NewRegistry()
	RegisterMetrics(reg, x.m)

	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]float64{}

	for _, f := range families {
		for _, m := range f.GetMetric() {
			key := f.GetName()
			for _, l := range m.GetLabel() {
				key += "," + l.GetName() + "=" + l.GetValue()
			}

			switch {
			case m.GetGauge() != nil:
				got[key] = m.GetGauge().GetValue()
			case m.GetCounter() != nil:
				got[key] = m.GetCounter().GetValue()
			}
		}
	}

	if got["app_sepp_n32c_state,partner=y"] != 2 {
		t.Fatalf("state gauge: %v", got)
	}

	if got["app_sepp_certificate_expiry_timestamp_seconds"] < float64(time.Now().Unix()) {
		t.Fatalf("expiry gauge: %v", got)
	}

	if got["app_sepp_n32c_handshakes_total,partner=y,result=established,role=initiator"]+got["app_sepp_n32c_handshakes_total,partner=y,result=established,role=responder"] == 0 {
		t.Fatalf("handshake counters: %v", got)
	}
}
