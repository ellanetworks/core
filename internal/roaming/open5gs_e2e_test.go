//go:build open5gs

// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/pki"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sepp"
	"go.uber.org/zap"
)

func composeDir() string {
	return filepath.Join("..", "..", "integration", "compose", "roaming")
}

func readCert(t *testing.T, name string) string {
	t.Helper()

	dir := os.Getenv("ROAMING_CERTS")
	if dir == "" {
		dir = filepath.Join(composeDir(), "certs")
	}

	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s (start the stack with integration/compose/roaming/run.sh up): %v", name, err)
	}

	return string(b)
}

func open5GSSEPPAddr() string {
	if addr := os.Getenv("OPEN5GS_SEPP_ADDR"); addr != "" {
		return addr
	}

	return "127.0.0.1:7777"
}

var (
	e2eOperator = wire.PlmnID{Mcc: "999", Mnc: "70"}
	e2eHome     = wire.PlmnID{Mcc: "001", Mnc: "01"}
)

func open5GSPartner(t *testing.T) sepp.PartnerConfig {
	t.Helper()

	return sepp.PartnerConfig{
		ID:             "open5gs",
		Name:           "open5gs",
		PLMNs:          []wire.PlmnID{e2eHome},
		FQDN:           "sepp1.5gc.mnc001.mcc001.3gppnetwork.org",
		N32cURI:        "https://sepp1.5gc.mnc001.mcc001.3gppnetwork.org:7777",
		TrustAnchorPEM: readCert(t, "open5gs-ca.crt"),
		NRFAPIRoot:     "http://nrf.5gc.mnc001.mcc001.3gppnetwork.org",
	}
}

func ellaIdentity(t *testing.T, seppID int) sepp.IdentityConfig {
	t.Helper()

	fqdn := pki.PLMN{MCC: e2eOperator.Mcc, MNC: e2eOperator.Mnc}.SEPPFQDN(seppID)

	if seppID == 1 {
		return sepp.IdentityConfig{SEPPID: 1, FQDN: fqdn, CertChainPEM: readCert(t, "ella-sepp.crt") + readCert(t, "ella-ca.crt"), KeyPEM: readCert(t, "ella-sepp.key")}
	}

	caCert, caKey, err := pki.ValidateSEPPCA([]byte(readCert(t, "ella-ca.crt")), []byte(readCert(t, "ella-ca.key")))
	if err != nil {
		t.Fatalf("Ella e2e CA: %v", err)
	}

	leaf, key, err := pki.IssueSEPPCert(fqdn, pki.PLMN{MCC: e2eOperator.Mcc, MNC: e2eOperator.Mnc}, caCert, caKey)
	if err != nil {
		t.Fatal(err)
	}

	keyPEM, err := pki.EncodePrivateKeyPEM(key)
	if err != nil {
		t.Fatal(err)
	}

	return sepp.IdentityConfig{SEPPID: seppID, FQDN: fqdn, CertChainPEM: string(pki.EncodeCertPEM(leaf)) + readCert(t, "ella-ca.crt"), KeyPEM: string(keyPEM)}
}

func startEllaSEPP(t *testing.T, partner sepp.PartnerConfig, identity sepp.IdentityConfig, port int) *sepp.Manager {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	m := sepp.New(sepp.Config{
		Partners: func(context.Context) ([]sepp.PartnerConfig, error) { return []sepp.PartnerConfig{partner}, nil },
		Identity: func(context.Context) (*sepp.IdentityConfig, error) { return &identity, nil },
		Operator: func(context.Context) (wire.PlmnID, error) { return e2eOperator, nil },
		N32:      config.N32Interface{Enabled: true, Address: "0.0.0.0", Port: port},
	}, zap.NewExample().Named(identity.FQDN), sepp.WithDialer(func(ctx context.Context, network, addr string) (net.Conn, error) {
		if strings.HasPrefix(addr, partner.FQDN+":") {
			addr = open5GSSEPPAddr()
		}

		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, addr)
	}))

	done := make(chan struct{})

	go func() {
		defer close(done)

		m.Run(ctx, make(chan struct{}))
	}()

	t.Cleanup(func() {
		cancel()
		<-done
	})

	return m
}

func waitEstablished(t *testing.T, m *sepp.Manager, partnerID string) {
	t.Helper()

	deadline := time.Now().Add(20 * time.Second)
	for m.Context(partnerID).State != sepp.StateEstablished {
		if time.Now().After(deadline) {
			t.Fatalf("%s: no N32-c context with Open5GS: %+v", m.LocalFQDN(), m.Context(partnerID))
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("%s: N32-c context: %+v", m.LocalFQDN(), m.Context(partnerID))
}

func authenticateThroughOpen5GS(t *testing.T, ctx context.Context, m *sepp.Manager, partnerID string) {
	t.Helper()

	discovery := NewDiscovery(m)

	ausf, err := discovery.Discover(ctx, DiscoveryRequest{PartnerID: partnerID, HomePLMN: e2eHome, ServingPLMN: e2eOperator, Target: wire.NFTypeAUSF, Requester: wire.NFTypeAMF, Service: wire.ServiceNausfAuth})
	if err != nil {
		t.Fatalf("%s: H-NRF discovery: %v", m.LocalFQDN(), err)
	}

	t.Logf("%s: AUSF: %s (%s)", m.LocalFQDN(), ausf.APIRoot, ausf.NfInstanceID)

	body, err := json.Marshal(map[string]string{"supiOrSuci": "imsi-001010000000001", "servingNetworkName": "5G:mnc070.mcc999.3gppnetwork.org"})
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ausf.APIRoot+wire.UEAuthenticationsPath(), bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", wire.ContentTypeJSON)
	req.Header.Set("User-Agent", "AMF")

	resp, err := m.Client(partnerID).Do(req)
	if err != nil {
		t.Fatalf("%s: AUSF POST over N32-f: %v", m.LocalFQDN(), err)
	}

	defer func() { _ = resp.Body.Close() }()

	rsp, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("%s: AUSF POST = %d %s", m.LocalFQDN(), resp.StatusCode, rsp)
	}

	var authCtx wire.UEAuthenticationCtx
	if err := json.Unmarshal(rsp, &authCtx); err != nil || authCtx.AuthType != wire.AuthType5GAKA {
		t.Fatalf("%s: UEAuthenticationCtx = %s, %v", m.LocalFQDN(), rsp, err)
	}

	href, ok := authCtx.Href("5g-aka")
	if !ok || !strings.HasPrefix(href, "http://ausf.5gc.mnc001.mcc001.3gppnetwork.org/") {
		t.Fatalf("%s: 5g-aka link = %q", m.LocalFQDN(), href)
	}

	t.Logf("%s: AUSF answered 201 (Location %s, 5g-aka %s)", m.LocalFQDN(), resp.Header.Get("Location"), href)
}

func TestOpen5GSHomeNetwork(t *testing.T) {
	ctx := context.Background()
	partner := open5GSPartner(t)

	m := startEllaSEPP(t, partner, ellaIdentity(t, 1), 7443)

	waitEstablished(t, m, partner.ID)
	authenticateThroughOpen5GS(t, ctx, m, partner.ID)

	deadline := time.Now().Add(30 * time.Second)
	for m.Context(partner.ID).Initiator != sepp.InitiatorPeer {
		if time.Now().After(deadline) {
			t.Logf("Open5GS did not initiate its own handshake within 30 s (state %+v)", m.Context(partner.ID))
			return
		}

		time.Sleep(200 * time.Millisecond)
	}

	t.Logf("Open5GS-initiated handshake accepted: %+v", m.Context(partner.ID))
}

func TestOpen5GSHomeNetworkWithOneSEPPPerNode(t *testing.T) {
	ctx := context.Background()
	partner := open5GSPartner(t)

	nodes := make([]*sepp.Manager, 0, 3)

	for seppID := 1; seppID <= 3; seppID++ {
		nodes = append(nodes, startEllaSEPP(t, partner, ellaIdentity(t, seppID), 7442+seppID))
	}

	for _, m := range nodes {
		waitEstablished(t, m, partner.ID)
	}

	for _, m := range nodes {
		authenticateThroughOpen5GS(t, ctx, m, partner.ID)
	}

	before := nodes[0].Context(partner.ID)

	if _, err := nodes[1].Rehandshake(partner.ID); err != nil {
		t.Fatal(err)
	}

	waitEstablished(t, nodes[1], partner.ID)
	authenticateThroughOpen5GS(t, ctx, nodes[1], partner.ID)

	if st := nodes[0].Context(partner.ID); st.State != sepp.StateEstablished || !st.Since.Equal(before.Since) {
		t.Fatalf("a handshake of SEPP 2 changed SEPP 1's context: %+v, was %+v", st, before)
	}

	authenticateThroughOpen5GS(t, ctx, nodes[0], partner.ID)

	restarted := time.Now()

	restart := exec.CommandContext(ctx, "docker", "compose", "-f", filepath.Join(composeDir(), "docker-compose.yaml"), "restart", "sepp")
	if out, err := restart.CombinedOutput(); err != nil {
		t.Fatalf("restart the Open5GS SEPP: %v: %s", err, out)
	}

	deadline := time.Now().Add(60 * time.Second)

	for _, m := range nodes {
		for {
			st := m.Context(partner.ID)
			if st.State == sepp.StateEstablished && st.Initiator == sepp.InitiatorPeer && st.Since.After(restarted) {
				t.Logf("%s: Open5GS-initiated handshake accepted: %+v", m.LocalFQDN(), st)
				break
			}

			if time.Now().After(deadline) {
				t.Fatalf("%s: the restarted Open5GS SEPP did not handshake (state %+v); is every Ella SEPP in its n32.client.sepp?", m.LocalFQDN(), st)
			}

			time.Sleep(200 * time.Millisecond)
		}
	}

	for _, m := range nodes {
		authenticateThroughOpen5GS(t, ctx, m, partner.ID)
	}
}
