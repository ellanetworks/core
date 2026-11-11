// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
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

func (p *n32fPeer) callback(t *testing.T, x *node) inboundResult {
	t.Helper()

	return p.send(t, x, http.MethodPost, deregURI, target(ownAMF), []byte(`{}`))
}

func otherPeer(st ContextStatus, fqdn string) bool {
	return slices.ContainsFunc(st.OtherPeers, func(pc PeerContext) bool { return pc.Peer == fqdn })
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

	if !otherPeer(st, sepp3.fqdn) {
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

	if st := x.state("p"); !otherPeer(st, sepp3.fqdn) || len(st.OtherPeers) != 1 {
		t.Fatalf("replaced context = %+v", st)
	}

	if status, rsp := sepp3.exchange(t, x, SecNegotiateReqData{Sender: sepp3.fqdn, SupportedSecCapabilityList: []SecurityCapability{SecurityNone}}); status != http.StatusForbidden {
		t.Fatalf("NONE from the other SEPP: %d %+v", status, rsp)
	}

	cfg := p.partnerConfig()
	cfg.NRFAPIRoots = map[wire.PlmnID]string{cfg.PLMNs[0]: "http://nrf2.5gc.mnc001.mcc001.3gppnetwork.org"}
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
}

func TestHPLMNNRFIsPerPLMN(t *testing.T) {
	other := wire.PlmnID{Mcc: "001", Mnc: "02"}
	cfg := PartnerConfig{
		ID: "p", Name: "p", PLMNs: []wire.PlmnID{yPLMN, other}, FQDN: yFQDN, N32cURI: "https://" + yFQDN, TrustAnchorPEM: newTestCA(t, "partner").pem(),
		NRFAPIRoots: map[wire.PlmnID]string{other: "http://nrf.example.org:8080"},
	}

	p, err := newPartner(cfg)
	if err != nil {
		t.Fatalf("partner unusable: %s", err)
	}

	for home, want := range map[wire.PlmnID]string{yPLMN: "https://nrf.5gc.mnc001.mcc001.3gppnetwork.org", other: "http://nrf.example.org:8080"} {
		if root, err := p.hplmnNRF(home); err != nil || root.String() != want {
			t.Errorf("home NRF of %s = %v, %v, want %s", home, root, err, want)
		}
	}

	if _, err := p.hplmnNRF(wire.PlmnID{Mcc: "999", Mnc: "99"}); err == nil {
		t.Error("home NRF of a PLMN of no partner: no error")
	}

	if err := p.allowsTarget(&url.URL{Scheme: "http", Host: "nrf.example.org:8080"}); err != nil {
		t.Errorf("target at the overridden NRF: %s", err)
	}

	cfg.NRFAPIRoots = map[wire.PlmnID]string{{Mcc: "999", Mnc: "99"}: "http://nrf.example.org"}
	if _, err := newPartner(cfg); err == nil {
		t.Error("NRF apiRoot of a PLMN not of the partner accepted")
	}

	cfg.NRFAPIRoots = map[wire.PlmnID]string{other: "http://nrf2.example.org:8080"}
	if changed, err := newPartner(cfg); err != nil || p.sameN32(changed) {
		t.Errorf("a changed NRF apiRoot is the same N32 configuration: %v", err)
	}
}
