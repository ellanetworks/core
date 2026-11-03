// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

func (p *n32fPeer) exchange(t *testing.T, x *node, req SecNegotiateReqData) (int, SecNegotiateRspData) {
	t.Helper()

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	got := p.send(t, x, http.MethodPost, N32cExchangeCapabilityPath, http.Header{"Content-Type": {wire.ContentTypeJSON}}, body)

	var rsp SecNegotiateRspData

	if got.status == http.StatusOK {
		if err := json.Unmarshal(got.body, &rsp); err != nil {
			t.Fatal(err)
		}
	}

	return got.status, rsp
}

func teardownReq(sender string) SecNegotiateReqData {
	return SecNegotiateReqData{Sender: sender, SupportedSecCapabilityList: []SecurityCapability{SecurityNone}}
}

func (p *n32fPeer) callback(t *testing.T, x *node) inboundResult {
	t.Helper()

	return p.send(t, x, http.MethodPost, deregURI, target(ownAMF), []byte(`{}`))
}

func otherPeer(st ContextStatus, fqdn string) (PeerContext, bool) {
	for _, pc := range st.OtherPeers {
		if pc.Peer == fqdn {
			return pc, true
		}
	}

	return PeerContext{}, false
}

func TestOtherSEPPsOfThePartner(t *testing.T) {
	x, p := n32fFixture(t, fastTuning())

	sepp3 := newN32fPeer(t, "sepp3.5gc.mnc001.mcc001.3gppnetwork.org", p.ca)

	if got := p.callback(t, x); got.status != http.StatusNotFound {
		t.Fatalf("N32-f from the configured SEPP: %d %s", got.status, got.body)
	}

	got := sepp3.callback(t, x)
	if got.status != http.StatusForbidden || wire.ParseProblem(got.status, got.body).Cause != wire.CauseContextNotFound {
		t.Fatalf("N32-f from a SEPP of the partner without a context: %d %s", got.status, got.body)
	}

	primary := x.state("p")

	status, rsp := sepp3.exchange(t, x, baseReq(sepp3.fqdn))
	if status != http.StatusOK || rsp.Sender != xFQDN || rsp.SelectedSecCapability != SecurityTLS {
		t.Fatalf("handshake from another SEPP of the partner: %d %+v", status, rsp)
	}

	st := x.state("p")

	pc, ok := otherPeer(st, sepp3.fqdn)
	if !ok || !pc.NFTLST || pc.Since.IsZero() {
		t.Fatalf("other peers = %+v", st.OtherPeers)
	}

	if st.State != StateEstablished || !st.Since.Equal(primary.Since) || st.Peer != "" {
		t.Fatalf("a handshake of another partner SEPP changed the context with the configured SEPP: %+v, was %+v", st, primary)
	}

	if got := sepp3.callback(t, x); got.status != http.StatusNotFound || wire.ParseProblem(got.status, got.body).Cause != wire.CauseContextNotFound {
		t.Fatalf("N32-f from the other SEPP after its handshake: %d %s", got.status, got.body)
	}

	if status, _ := sepp3.exchange(t, x, baseReq(p.fqdn)); status != http.StatusForbidden {
		t.Fatalf("a sender that is not in the client certificate: %d", status)
	}

	if status, _ := sepp3.exchange(t, x, baseReq("sepp3.5gc.mnc099.mcc099.3gppnetwork.org")); status != http.StatusForbidden {
		t.Fatalf("a sender of another network: %d", status)
	}

	if status, _ := sepp3.exchange(t, x, baseReq(sepp3.fqdn)); status != http.StatusOK {
		t.Fatalf("a repeated handshake: %d", status)
	}

	if again, ok := otherPeer(x.state("p"), sepp3.fqdn); !ok || again.Since.Before(pc.Since) {
		t.Fatalf("replaced context = %+v", again)
	}

	if status, rsp := sepp3.exchange(t, x, teardownReq(sepp3.fqdn)); status != http.StatusOK || rsp.SelectedSecCapability != SecurityNone {
		t.Fatalf("teardown from the other SEPP: %d %+v", status, rsp)
	}

	if st := x.state("p"); len(st.OtherPeers) != 0 || st.State != StateEstablished || !st.Since.Equal(primary.Since) {
		t.Fatalf("after the other SEPP's teardown: %+v", st)
	}

	if got := sepp3.callback(t, x); got.status != http.StatusForbidden {
		t.Fatalf("N32-f from the other SEPP after its teardown: %d %s", got.status, got.body)
	}

	if status, _ := sepp3.exchange(t, x, baseReq(sepp3.fqdn)); status != http.StatusOK {
		t.Fatalf("handshake again: %d", status)
	}

	cfg := p.partnerConfig()
	cfg.NRFAPIRoot = "http://nrf2.5gc.mnc001.mcc001.3gppnetwork.org"
	x.setPartners(cfg)

	if st := x.waitState("p", StateEstablished); len(st.OtherPeers) != 0 {
		t.Fatalf("a partner change kept the contexts of other SEPPs: %+v", st)
	}
}

func TestRedirectedSEPPIsAcceptedInbound(t *testing.T) {
	x, p, p2, ports := redirectingPeers(t)

	p.redirect.Store(&[2]string{"https://" + p2.fqdn + ":" + ports[p2.fqdn] + N32cExchangeCapabilityPath, ""})

	x.setPartners(p.partnerConfig())
	x.start()

	st := x.waitState("p", StateEstablished)
	if st.Peer != p2.fqdn {
		t.Fatalf("context = %+v", st)
	}

	if got := p2.callback(t, x); got.status != http.StatusNotFound || wire.ParseProblem(got.status, got.body).Cause != wire.CauseContextNotFound {
		t.Fatalf("N32-f from the SEPP the handshake was redirected to: %d %s", got.status, got.body)
	}

	if got := p.callback(t, x); got.status != http.StatusForbidden {
		t.Fatalf("N32-f from the configured SEPP, which has no context: %d %s", got.status, got.body)
	}

	time.Sleep(10 * time.Millisecond)

	status, rsp := p2.exchange(t, x, baseReq(p2.fqdn))
	if status != http.StatusOK || rsp.SelectedSecCapability != SecurityTLS {
		t.Fatalf("handshake from the redirect target: %d %+v", status, rsp)
	}

	after := x.state("p")
	if after.State != StateEstablished || after.Initiator != InitiatorPeer || after.Peer != p2.fqdn || len(after.OtherPeers) != 0 || !after.Since.After(st.Since) {
		t.Fatalf("context after the redirect target's handshake = %+v", after)
	}

	if _, _, err := get(t, x.m.Client("p"), "http://udm.5gc.mnc001.mcc001.3gppnetwork.org/nudm-sdm/v2/imsi-001010000000001", nil); err != nil {
		t.Fatal(err)
	}

	if r, _ := p2.last(); r.URL.Path != "/nudm-sdm/v2/imsi-001010000000001" {
		t.Fatalf("N32-f went to %s", r.URL.Path)
	}

	if status, _ := p2.exchange(t, x, teardownReq(p2.fqdn)); status != http.StatusOK {
		t.Fatalf("teardown from the redirect target: %d", status)
	}

	if st := x.state("p"); st.State != StateTerminated && st.State != StateEstablishing && st.State != StateEstablished {
		t.Fatalf("after the redirect target's teardown: %+v", st)
	}
}
