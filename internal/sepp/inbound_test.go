// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

const (
	ownAMF   = "https://amf1.5gc.mnc070.mcc999.3gppnetwork.org"
	deregURI = "/roaming-callbacks/v1/imsi-001010000000001/uecm-dereg"
)

type inboundResult struct {
	status int
	header http.Header
	body   []byte
}

func (p *n32fPeer) send(t *testing.T, x *node, method, path string, header http.Header, body []byte) inboundResult {
	t.Helper()

	pool := x509.NewCertPool()
	pool.AddCert(x.ca.cert)

	protocols := new(http.Protocols)
	protocols.SetHTTP2(true)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool, ServerName: x.fqdn, Certificates: []tls.Certificate{p.cert}},
		Protocols:       protocols,
	}
	defer tr.CloseIdleConnections()

	req, err := http.NewRequestWithContext(context.Background(), method, x.url()+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	req.Header = header.Clone()
	if req.Header == nil {
		req.Header = http.Header{}
	}

	client := &http.Client{Transport: tr, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = resp.Body.Close() }()

	rb, _ := io.ReadAll(resp.Body)

	return inboundResult{status: resp.StatusCode, header: resp.Header, body: rb}
}

func target(root string) http.Header {
	return http.Header{HeaderTargetAPIRoot: {root}}
}

func TestInboundChecks(t *testing.T) {
	x, p := n32fFixture(t, fastTuning())
	x.setCluster(1, 2)

	withONI := func(h http.Header, v string) http.Header {
		h.Set(HeaderOriginatingNetworkID, v)
		return h
	}

	for name, tc := range map[string]struct {
		method, path string
		header       http.Header
		status       int
		cause        wire.Cause
	}{
		"no Target-apiRoot":          {http.MethodPost, deregURI, nil, 400, wire.CauseMandatoryIEMissing},
		"two Target-apiRoots":        {http.MethodPost, deregURI, http.Header{HeaderTargetAPIRoot: {ownAMF, ownAMF}}, 400, wire.CauseMandatoryIEIncorrect},
		"http Target-apiRoot":        {http.MethodPost, deregURI, target("http://amf1.5gc.mnc070.mcc999.3gppnetwork.org"), 400, wire.CauseMandatoryIEIncorrect},
		"Target-apiRoot with prefix": {http.MethodPost, deregURI, target(ownAMF + "/pfx"), 400, wire.CauseMandatoryIEIncorrect},
		"not a callback host":        {http.MethodPost, deregURI, target("https://udm.5gc.mnc070.mcc999.3gppnetwork.org"), 400, wire.CauseMandatoryIEIncorrect},
		"host of another PLMN":       {http.MethodPost, deregURI, target("https://amf1.5gc.mnc001.mcc001.3gppnetwork.org"), 400, wire.CauseMandatoryIEIncorrect},
		"wrong originating network":  {http.MethodPost, deregURI, withONI(target(ownAMF), "999-70"), 403, wire.CauseOriginatingNetworkIDMismatch},
		"malformed originating id":   {http.MethodPost, deregURI, withONI(target(ownAMF), "x"), 400, wire.CauseMandatoryIEIncorrect},
		"path not allowed":           {http.MethodPost, "/nudm-sdm/v2/imsi-001010000000001", target(ownAMF), 404, wire.CauseResourceURIStructureNotFound},
		"SMF dereg, no handler":      {http.MethodPost, deregURI, target("https://smf1.5gc.mnc070.mcc999.3gppnetwork.org"), 404, wire.CauseContextNotFound},
		"method not allowed":         {http.MethodPut, deregURI, target(ownAMF), 404, wire.CauseResourceURIStructureNotFound},
		"SUPI of another PLMN":       {http.MethodPost, "/roaming-callbacks/v1/imsi-999700000000001/uecm-dereg", target(ownAMF), 404, wire.CauseContextNotFound},
		"own node, no handler":       {http.MethodPost, deregURI, withONI(target(ownAMF), "001-01"), 404, wire.CauseContextNotFound},
		"own node, SDM change":       {http.MethodPost, "/roaming-callbacks/v1/imsi-001010000000001/sdm-change", target("https://smf1.5gc.mnc070.mcc999.3gppnetwork.org"), 404, wire.CauseSubscriptionNotFound},
		"unknown node's AMF":         {http.MethodPost, deregURI, target("https://amf3.5gc.mnc070.mcc999.3gppnetwork.org"), 404, wire.CauseContextNotFound},
		"unknown node's SMF":         {http.MethodPost, "/roaming-callbacks/v1/imsi-001010000000001/sdm-change", target("https://smf3.5gc.mnc070.mcc999.3gppnetwork.org"), 404, wire.CauseSubscriptionNotFound},
		"another node's AMF":         {http.MethodPost, deregURI, target("https://amf2.5gc.mnc070.mcc999.3gppnetwork.org"), 503, wire.CauseNFCongestion},
		"another node's SMF":         {http.MethodPost, "/roaming-callbacks/v1/imsi-001010000000001/sdm-change", target("https://smf2.5gc.mnc070.mcc999.3gppnetwork.org"), 503, wire.CauseNFCongestion},
	} {
		got := p.send(t, x, tc.method, tc.path, tc.header, []byte(`{}`))

		if got.status != tc.status || wire.ParseProblem(got.status, got.body).Cause != tc.cause {
			t.Errorf("%s: %d %s, want %d %s", name, got.status, got.body, tc.status, tc.cause)
		}

		if wantRetry := tc.status == http.StatusServiceUnavailable; (got.header.Get("Retry-After") == "2") != wantRetry {
			t.Errorf("%s: Retry-After = %q", name, got.header.Get("Retry-After"))
		}

		if got.header.Get(HeaderServer) != SEPPServer(xFQDN) {
			t.Errorf("%s: Server = %q", name, got.header.Get(HeaderServer))
		}
	}

	got := p.send(t, x, http.MethodPost, deregURI, target(ownAMF), bytes.Repeat([]byte("x"), maxBodyBytes+1))
	if got.status != http.StatusRequestEntityTooLarge {
		t.Fatalf("large body: %d", got.status)
	}
}

func TestInboundDispatchToCallbacks(t *testing.T) {
	x, p := n32fFixture(t, fastTuning())

	type seen struct {
		peer   Peer
		ok     bool
		host   string
		target string
		body   string
	}

	calls := make(chan seen, 1)

	x.m.SetCallbacks(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, ok := PeerFrom(r.Context())

		body, _ := io.ReadAll(r.Body)
		calls <- seen{peer: peer, ok: ok, host: r.Host, target: r.Header.Get(HeaderTargetAPIRoot), body: string(body)}

		w.WriteHeader(http.StatusNoContent)
	}))

	got := p.send(t, x, http.MethodPost, deregURI, target(ownAMF), []byte(`{"deregReason":"UE_INITIAL_REGISTRATION"}`))
	if got.status != http.StatusNoContent {
		t.Fatalf("callback: %d %s", got.status, got.body)
	}

	c := <-calls
	if !c.ok || c.peer.PartnerID != "p" || c.peer.Callback != (Callback{NF: CallbackAMF, SEPPID: 1, Supi: "imsi-001010000000001", Kind: CallbackUECMDereg}) {
		t.Fatalf("peer = %+v, %v", c.peer, c.ok)
	}

	if c.target != "" || c.host != strings.TrimPrefix(ownAMF, "https://") || c.body != `{"deregReason":"UE_INITIAL_REGISTRATION"}` {
		t.Fatalf("dispatched request: %+v", c)
	}

	got = p.send(t, x, http.MethodPost, deregURI, target("https://smf1.5gc.mnc070.mcc999.3gppnetwork.org"), []byte(`{"deregReason":"DUPLICATE_PDU_SESSION","pduSessionId":5}`))
	if got.status != http.StatusNoContent {
		t.Fatalf("SMF callback: %d %s", got.status, got.body)
	}

	c = <-calls
	if !c.ok || c.peer.Callback != (Callback{NF: CallbackSMF, SEPPID: 1, Supi: "imsi-001010000000001", Kind: CallbackUECMDereg}) || c.host != "smf1.5gc.mnc070.mcc999.3gppnetwork.org" {
		t.Fatalf("SMF callback dispatch: %+v", c)
	}
}

func TestInboundRateLimit(t *testing.T) {
	tuning := fastTuning()
	tuning.N32fRequestsPerSecond = 2

	x, p := n32fFixture(t, tuning)

	statuses := []int{}
	for range 3 {
		statuses = append(statuses, p.send(t, x, http.MethodPost, deregURI, target(ownAMF), nil).status)
	}

	if statuses[0] != 404 || statuses[1] != 404 || statuses[2] != http.StatusTooManyRequests {
		t.Fatalf("statuses = %v", statuses)
	}
}
