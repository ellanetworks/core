// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/pki"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sepp"
	"go.uber.org/zap"
)

const (
	seppN32Port       = 7443
	seppPartnerName   = "hplmn"
	seppPartnerMCC    = "208"
	seppPartnerMNC    = "93"
	seppPartnerFQDN   = "sepp1.5gc.mnc093.mcc208.3gppnetwork.org"
	seppPartnerSUPI   = "imsi-208930000000001"
	seppEllaDomain    = "5gc.mnc001.mcc001.3gppnetwork.org"
	seppWaitHandshake = 90 * time.Second
)

var seppNodesWithN32 = map[int]bool{1: true, 2: false, 3: true}

func ellaSEPPFQDN(seppID int) string {
	return "sepp" + strconv.Itoa(seppID) + "." + seppEllaDomain
}

type partnerSEPP struct {
	m      *sepp.Manager
	ca     string
	url    string
	cancel context.CancelFunc
	done   chan struct{}

	mu   sync.Mutex
	ella sepp.PartnerConfig
}

func clusterGateway() string {
	return strings.TrimSuffix(ClusterAddress(1), "11") + "1"
}

func startPartnerSEPP(t *testing.T, ellaAnchorPEM string) *partnerSEPP {
	t.Helper()

	plmn := pki.PLMN{MCC: seppPartnerMCC, MNC: seppPartnerMNC}

	leaf, key, ca, err := pki.GenerateSEPPCert(seppPartnerFQDN, plmn)
	if err != nil {
		t.Fatalf("partner SEPP certificate: %v", err)
	}

	keyPEM, err := pki.EncodePrivateKeyPEM(key)
	if err != nil {
		t.Fatal(err)
	}

	identity := sepp.IdentityConfig{SEPPID: 1, FQDN: seppPartnerFQDN, CertChainPEM: string(pki.EncodeCertPEM(leaf)) + string(pki.EncodeCertPEM(ca)), KeyPEM: string(keyPEM)}

	p := &partnerSEPP{
		ca:   string(pki.EncodeCertPEM(ca)),
		done: make(chan struct{}),
		ella: sepp.PartnerConfig{
			ID:             "ella",
			Name:           "ella",
			PLMNs:          []wire.PlmnID{{Mcc: "001", Mnc: "01"}},
			FQDN:           ellaSEPPFQDN(1),
			N32cURI:        "https://" + net.JoinHostPort(ClusterAddress(1), strconv.Itoa(seppN32Port)),
			TrustAnchorPEM: ellaAnchorPEM,
		},
	}

	tuning := sepp.DefaultTuning()
	tuning.Reconcile = 5 * time.Second

	p.m = sepp.New(sepp.Config{
		Partners: func(context.Context) ([]sepp.PartnerConfig, error) {
			p.mu.Lock()
			defer p.mu.Unlock()

			return []sepp.PartnerConfig{p.ella}, nil
		},
		Identity: func(context.Context) (*sepp.IdentityConfig, error) { return &identity, nil },
		Operator: func(context.Context) (wire.PlmnID, error) {
			return wire.PlmnID{Mcc: seppPartnerMCC, Mnc: seppPartnerMNC}, nil
		},
		N32: config.N32Interface{Enabled: true, Address: "0.0.0.0"},
	}, zap.NewNop(), sepp.WithTuning(tuning))

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	p.m.Reconcile(ctx)

	addr := p.m.ListenAddr()
	if addr == nil {
		cancel()
		t.Fatalf("partner SEPP listener: %+v", p.m.ListenerStatus())
	}

	_, port, _ := net.SplitHostPort(addr.String())
	p.url = "https://" + net.JoinHostPort(clusterGateway(), port)

	go func() {
		defer close(p.done)

		p.m.Run(ctx, make(chan struct{}))
	}()

	t.Cleanup(p.stop)

	return p
}

func (p *partnerSEPP) stop() {
	p.cancel()
	<-p.done
}

func (p *partnerSEPP) setEllaAnchor(pem string) {
	p.mu.Lock()
	p.ella.TrustAnchorPEM = pem
	p.mu.Unlock()

	p.m.Reconcile(context.Background())
}

func (p *partnerSEPP) context() sepp.ContextStatus {
	return p.m.Context("ella")
}

func (p *partnerSEPP) other(fqdn string) (sepp.PeerContext, bool) {
	for _, pc := range p.context().OtherPeers {
		if pc.Peer == fqdn {
			return pc, true
		}
	}

	return sepp.PeerContext{}, false
}

func (p *partnerSEPP) sendCallback(ctx context.Context, seppID int) (int, wire.ProblemDetails, error) {
	status, pd, _, err := p.sendCallbackHeader(ctx, seppID)

	return status, pd, err
}

func (p *partnerSEPP) sendCallbackHeader(ctx context.Context, seppID int) (int, wire.ProblemDetails, http.Header, error) {
	url := "https://amf" + strconv.Itoa(seppID) + "." + seppEllaDomain + "/roaming-callbacks/v1/" + seppPartnerSUPI + "/uecm-dereg"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte(`{"deregReason":"SUBSCRIPTION_WITHDRAWN"}`)))
	if err != nil {
		return 0, wire.ProblemDetails{}, nil, err
	}

	req.Header.Set("Content-Type", wire.ContentTypeJSON)

	resp, err := p.m.Client("ella").Do(req)
	if err != nil {
		return 0, wire.ProblemDetails{}, nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	return resp.StatusCode, wire.ParseProblem(resp.StatusCode, body), resp.Header, nil
}

func waitUntil(ctx context.Context, timeout time.Duration, check func() error) error {
	deadline := time.Now().Add(timeout)

	for {
		err := check()
		if err == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func partnerN32BySEPP(ctx context.Context, nodes map[int]*client.Client, id string) (map[int]client.RoamingPartnerN32, error) {
	out := map[int]client.RoamingPartnerN32{}

	for seppID, c := range nodes {
		p, err := c.GetRoamingPartner(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("SEPP %d: %w", seppID, err)
		}

		if p.N32 != nil {
			out[seppID] = *p.N32
		}
	}

	return out, nil
}

func waitN32(ctx context.Context, nodes map[int]*client.Client, id string, want map[int]string) error {
	return waitUntil(ctx, seppWaitHandshake, func() error {
		got, err := partnerN32BySEPP(ctx, nodes, id)
		if err != nil {
			return err
		}

		if len(got) != len(want) {
			return fmt.Errorf("partner N32 = %+v, want SEPPs %v", got, want)
		}

		for seppID, state := range want {
			if got[seppID].State != state {
				return fmt.Errorf("SEPP %d is %s (%s), want %s", seppID, got[seppID].State, got[seppID].LastError, state)
			}
		}

		return nil
	})
}

func generatedAnchor(st *client.SEPPStatus) (client.SEPPTrustAnchor, bool) {
	for _, a := range st.TrustAnchors {
		if a.Source == "generated" {
			return a, true
		}
	}

	return client.SEPPTrustAnchor{}, false
}

func waitSEPPStatus(ctx context.Context, nodes map[int]*client.Client, check func(map[int]*client.SEPPStatus) error) (map[int]*client.SEPPStatus, error) {
	var out map[int]*client.SEPPStatus

	err := waitUntil(ctx, seppWaitHandshake, func() error {
		out = map[int]*client.SEPPStatus{}

		for seppID, c := range nodes {
			st, err := c.GetSEPPStatus(ctx)
			if err != nil {
				return fmt.Errorf("SEPP %d: %w", seppID, err)
			}

			if st.Listener.SEPPID != seppID {
				return fmt.Errorf("the node asked for SEPP %d has SEPP ID %d", seppID, st.Listener.SEPPID)
			}

			out[seppID] = st
		}

		return check(out)
	})

	return out, err
}

func storedSEPPCertificate(st *client.SEPPStatus, seppID int) (client.SEPPCertificate, bool) {
	for _, c := range st.Certificates {
		if c.SEPPID == seppID {
			return c, true
		}
	}

	return client.SEPPCertificate{}, false
}

func startSEPPNode(ctx context.Context, dc *DockerClient, leader *client.Client, nodeID int, peers []string) error {
	n32 := 0
	if seppNodesWithN32[nodeID] {
		n32 = seppN32Port
	}

	tok, err := leader.MintClusterJoinToken(ctx, &client.MintJoinTokenOptions{TTLSeconds: 600})
	if err != nil {
		return fmt.Errorf("mint join token for node %d: %w", nodeID, err)
	}

	if err := writeNodeConfigBody(haComposeDir, nodeID, nil, "", "", false, false, n32); err != nil {
		return err
	}

	service := haNodeServices[nodeID-1]

	if err := dc.ComposeUpServicesWithFile(ctx, haComposeDir, ComposeFile(), service); err != nil {
		return err
	}

	return joinViaAPI(ctx, APIAddressForCluster(nodeID), tok.Token, seedsExcluding(peers, ClusterAddressWithPort(nodeID, 7000)), "")
}

func bringUpSEPPCluster(t *testing.T, ctx context.Context, dc *DockerClient) ([]*client.Client, []string) {
	t.Helper()

	dc.ComposeCleanup(ctx)

	t.Cleanup(func() {
		dc.ComposeDownWithFile(context.Background(), haComposeDir, ComposeFile())
	})

	peers := []string{ClusterAddressWithPort(1, 7000), ClusterAddressWithPort(2, 7000), ClusterAddressWithPort(3, 7000)}

	fail := func(err error) {
		captureClusterLogs(t, dc, haComposeDir, haNodeServices, true)
		t.Fatal(err)
	}

	if err := writeNodeConfigBody(haComposeDir, 1, peers, "", "", false, false, seppN32Port); err != nil {
		fail(err)
	}

	if err := dc.ComposeUpServicesWithFile(ctx, haComposeDir, ComposeFile(), haNodeServices[0]); err != nil {
		fail(err)
	}

	node1, err := newInsecureClient(getHANodeURLs()[0])
	if err != nil {
		fail(err)
	}

	token, err := initializeAndGetAdminToken(ctx, node1)
	if err != nil {
		fail(err)
	}

	node1.SetToken(token)

	for nodeID := 2; nodeID <= 3; nodeID++ {
		if err := startSEPPNode(ctx, dc, node1, nodeID, peers); err != nil {
			fail(err)
		}
	}

	clients, err := newHANodeClients()
	if err != nil {
		fail(err)
	}

	for _, c := range clients {
		c.SetToken(token)
	}

	if err := waitForClusterReady(ctx, clients); err != nil {
		fail(err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		fail(err)
	}

	return clients, peers
}

func dockerCmd(ctx context.Context, args ...string) error {
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker %s: %w: %s", strings.Join(args, " "), err, out)
	}

	return nil
}

func TestIntegrationHASEPP(t *testing.T) {
	suites.Require(t, suites.HASEPP)

	beginHATest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Minute)
	defer cancel()

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	defer func() { _ = dc.Close() }()

	clients, peers := bringUpSEPPCluster(t, ctx, dc)

	t.Cleanup(func() {
		if t.Failed() {
			captureClusterLogs(t, dc, haComposeDir, haNodeServices, true)
		}
	})

	c1, c2, c3 := clients[0], clients[1], clients[2]
	all := map[int]*client.Client{1: c1, 2: c2, 3: c3}
	withoutNode3 := map[int]*client.Client{1: c1, 2: c2}

	HALog(t, "every node with interfaces.n32 gets a certificate signed by the cluster SEPP CA, without operator action")

	initial, err := waitSEPPStatus(ctx, all, func(sts map[int]*client.SEPPStatus) error {
		anchor, ok := generatedAnchor(sts[2])
		if !ok || len(sts[2].TrustAnchors) != 1 {
			return fmt.Errorf("trust anchors = %+v", sts[2].TrustAnchors)
		}

		if len(sts[2].Certificates) != 2 {
			return fmt.Errorf("certificates = %+v", sts[2].Certificates)
		}

		for seppID, n32 := range seppNodesWithN32 {
			l := sts[seppID].Listener
			c, stored := storedSEPPCertificate(sts[2], seppID)

			switch {
			case l.Configured != n32:
				return fmt.Errorf("SEPP %d = %+v", seppID, l)
			case n32 && (!stored || c.Source != "generated" || c.FQDN != ellaSEPPFQDN(seppID) || c.TrustAnchorFingerprint != anchor.Fingerprint || !l.Listening || l.Error != ""):
				return fmt.Errorf("SEPP %d = %+v, %+v", seppID, l, c)
			case !n32 && stored:
				return fmt.Errorf("SEPP %d has a certificate without interfaces.n32: %+v", seppID, c)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("SEPP status: %v", err)
	}

	clusterCA, _ := generatedAnchor(initial[1])
	initial1, _ := storedSEPPCertificate(initial[2], 1)
	initial3, _ := storedSEPPCertificate(initial[2], 3)

	HALog(t, "every SEPP node handshakes with the partner on its own")

	partner := startPartnerSEPP(t, clusterCA.PEM)

	created, err := c1.CreateRoamingPartner(ctx, &client.RoamingPartnerOptions{
		Name:        seppPartnerName,
		PLMNs:       []client.RoamingPLMN{{Mcc: seppPartnerMCC, Mnc: seppPartnerMNC}},
		ProfileName: "default",
		SEPP: &client.RoamingPartnerSEPP{
			FQDN:           seppPartnerFQDN,
			N32cURI:        partner.url,
			TrustAnchorPEM: partner.ca,
		},
	})
	if err != nil {
		t.Fatalf("create roaming partner: %v", err)
	}

	if err := waitN32(ctx, all, created.ID, map[int]string{1: "established", 3: "established"}); err != nil {
		t.Fatalf("partner N32 on each node: %v", err)
	}

	if err := waitUntil(ctx, seppWaitHandshake, func() error {
		st := partner.context()
		if _, ok := partner.other(ellaSEPPFQDN(3)); st.State != sepp.StateEstablished || !ok || len(st.OtherPeers) != 1 {
			return fmt.Errorf("partner context = %+v", st)
		}

		return nil
	}); err != nil {
		t.Fatalf("partner side: %v", err)
	}

	established := partner.context()

	HALog(t, "a callback for node 3's AMF sent to node 1 gets 503 without Retry-After; an unknown node's gets 404")

	status, pd, header, err := partner.sendCallbackHeader(ctx, 3)
	if err != nil || status != http.StatusServiceUnavailable || pd.Cause != wire.CauseNFCongestion || header.Get("Retry-After") != "" {
		t.Fatalf("callback for another node = %d %+v (Retry-After %q), %v", status, pd, header.Get("Retry-After"), err)
	}

	status, pd, err = partner.sendCallback(ctx, 9)
	if err != nil || status != http.StatusNotFound || pd.Cause != wire.CauseContextNotFound {
		t.Fatalf("callback for an unknown node = %d %+v, %v", status, pd, err)
	}

	HALog(t, "a node failure leaves the other node's context untouched")

	partner3, _ := partner.other(ellaSEPPFQDN(3))

	if err := dc.ComposeStopWithFile(ctx, haComposeDir, ComposeFile(), haNodeServices[2]); err != nil {
		t.Fatal(err)
	}

	if err := waitN32(ctx, withoutNode3, created.ID, map[int]string{1: "established"}); err != nil {
		t.Fatalf("partner N32 with node 3 down: %v", err)
	}

	if _, err := waitSEPPStatus(ctx, withoutNode3, func(sts map[int]*client.SEPPStatus) error {
		if c, ok := storedSEPPCertificate(sts[2], 3); !ok || c.Fingerprint != initial3.Fingerprint {
			return fmt.Errorf("certificates while node 3 is down = %+v", sts[2].Certificates)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if st := partner.context(); st.State != sepp.StateEstablished || !st.Since.Equal(established.Since) {
		t.Fatalf("node 3's failure changed node 1's context: %+v, was %+v", st, established)
	}

	if err := dc.ComposeStartWithFile(ctx, haComposeDir, ComposeFile(), haNodeServices[2]); err != nil {
		t.Fatal(err)
	}

	if err := waitForClusterReady(ctx, clients); err != nil {
		t.Fatal(err)
	}

	if err := waitN32(ctx, all, created.ID, map[int]string{1: "established", 3: "established"}); err != nil {
		t.Fatalf("node 3 after its restart: %v", err)
	}

	if err := waitUntil(ctx, seppWaitHandshake, func() error {
		if pc, ok := partner.other(ellaSEPPFQDN(3)); !ok || !pc.Since.After(partner3.Since) {
			return fmt.Errorf("partner context with node 3 = %+v, was %+v", pc, partner3)
		}

		return nil
	}); err != nil {
		t.Fatalf("restarted node 3 did not handshake again: %v", err)
	}

	if st := partner.context(); !st.Since.Equal(established.Since) {
		t.Fatalf("node 3's restart changed node 1's context: %+v, was %+v", st, established)
	}

	home := pki.PLMN{MCC: "001", MNC: "01"}

	operatorCA, operatorKey, err := pki.GenerateSEPPCA(home)
	if err != nil {
		t.Fatal(err)
	}

	operatorCAPEM := string(pki.EncodeCertPEM(operatorCA))

	HALog(t, "regenerating the cluster SEPP CA makes every node with a generated certificate reissue it; every node handshakes again once the partner trusts the new CA")

	regenerated, err := c2.RegenerateSEPPCA(ctx)
	if err != nil {
		t.Fatalf("regenerate the SEPP CA: %v", err)
	}

	if regenerated.TrustAnchor.Fingerprint == clusterCA.Fingerprint {
		t.Fatalf("regenerate = %+v", regenerated)
	}

	if err := waitN32(ctx, all, created.ID, map[int]string{1: "failed", 3: "failed"}); err != nil {
		t.Fatalf("handshakes with a CA the partner does not trust: %v", err)
	}

	partner.setEllaAnchor(regenerated.TrustAnchor.PEM + operatorCAPEM)

	if err := waitN32(ctx, all, created.ID, map[int]string{1: "established", 3: "established"}); err != nil {
		t.Fatalf("handshakes after the partner imported the new CA: %v", err)
	}

	if _, err := waitSEPPStatus(ctx, all, func(sts map[int]*client.SEPPStatus) error {
		if len(sts[1].TrustAnchors) != 1 || sts[1].TrustAnchors[0].Fingerprint != regenerated.TrustAnchor.Fingerprint {
			return fmt.Errorf("trust anchors = %+v", sts[1].TrustAnchors)
		}

		for seppID, before := range map[int]client.SEPPCertificate{1: initial1, 3: initial3} {
			c, ok := storedSEPPCertificate(sts[1], seppID)
			if !ok || c.Fingerprint == before.Fingerprint || c.TrustAnchorFingerprint != regenerated.TrustAnchor.Fingerprint {
				return fmt.Errorf("SEPP %d after the CA regeneration = %+v", seppID, c)
			}
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := waitUntil(ctx, seppWaitHandshake, func() error {
		if _, ok := partner.other(ellaSEPPFQDN(3)); !ok || partner.context().State != sepp.StateEstablished {
			return fmt.Errorf("partner context = %+v", partner.context())
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rotated := partner.context()
	rotated3, _ := partner.other(ellaSEPPFQDN(3))

	HALog(t, "a certificate imported on one node changes that node's identity only")

	leaf3, key3, err := pki.IssueSEPPCert(ellaSEPPFQDN(3), home, operatorCA, operatorKey)
	if err != nil {
		t.Fatal(err)
	}

	key3PEM, err := pki.EncodePrivateKeyPEM(key3)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(1100 * time.Millisecond)

	imported, err := c1.ImportSEPPCertificate(ctx, 3, &client.ImportSEPPCertificateOptions{
		CertificateChainPEM: string(pki.EncodeCertPEM(leaf3)) + operatorCAPEM,
		PrivateKeyPEM:       string(key3PEM),
	})
	if err != nil {
		t.Fatalf("import the certificate of SEPP 3: %v", err)
	}

	if imported.Source != "imported" || imported.FQDN != ellaSEPPFQDN(3) {
		t.Fatalf("import = %+v", imported)
	}

	if _, err := waitSEPPStatus(ctx, all, func(sts map[int]*client.SEPPStatus) error {
		if len(sts[2].TrustAnchors) != 2 {
			return fmt.Errorf("trust anchors = %+v", sts[2].TrustAnchors)
		}

		if c, _ := storedSEPPCertificate(sts[3], 3); c.Source != "imported" || c.Fingerprint != imported.Fingerprint {
			return fmt.Errorf("SEPP 3 = %+v", c)
		}

		if c, _ := storedSEPPCertificate(sts[1], 1); c.Source != "generated" {
			return fmt.Errorf("SEPP 1 = %+v", c)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := waitUntil(ctx, seppWaitHandshake, func() error {
		got, err := partnerN32BySEPP(ctx, all, created.ID)
		if err != nil {
			return err
		}

		if pc, ok := partner.other(ellaSEPPFQDN(3)); got[3].State != "established" || !ok || !pc.Since.After(rotated3.Since) {
			return fmt.Errorf("SEPP 3 = %+v, partner side %+v", got[3], pc)
		}

		if st := partner.context(); !st.Since.Equal(rotated.Since) {
			return fmt.Errorf("the import on node 3 changed node 1's context: %+v", st)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	HALog(t, "removing a member deletes its generated certificate; a node that joins with its SEPP ID gets a new one and handshakes without partner changes")

	if err := c1.DeleteSEPPCertificate(ctx, 3); err != nil {
		t.Fatalf("delete the certificate of SEPP 3: %v", err)
	}

	sts, err := waitSEPPStatus(ctx, all, func(sts map[int]*client.SEPPStatus) error {
		if c, ok := storedSEPPCertificate(sts[1], 3); !ok || c.Source != "generated" {
			return fmt.Errorf("SEPP 3 certificate = %+v", c)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	generated3, _ := storedSEPPCertificate(sts[1], 3)

	if err := waitN32(ctx, all, created.ID, map[int]string{1: "established", 3: "established"}); err != nil {
		t.Fatal(err)
	}

	node3, err := nodeIDOf(ctx, c3)
	if err != nil {
		t.Fatal(err)
	}

	if err := dc.ComposeStopWithFile(ctx, haComposeDir, ComposeFile(), haNodeServices[2]); err != nil {
		t.Fatal(err)
	}

	if err := c1.RemoveClusterMember(ctx, node3, true); err != nil {
		t.Fatalf("remove node 3: %v", err)
	}

	if err := waitForMemberCount(ctx, c1, 2, time.Minute); err != nil {
		t.Fatal(err)
	}

	if _, err := waitSEPPStatus(ctx, withoutNode3, func(sts map[int]*client.SEPPStatus) error {
		if _, ok := storedSEPPCertificate(sts[2], 3); ok || len(sts[2].Certificates) != 1 {
			return fmt.Errorf("certificates after the removal = %+v", sts[2].Certificates)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := waitN32(ctx, withoutNode3, created.ID, map[int]string{1: "established"}); err != nil {
		t.Fatal(err)
	}

	if err := dockerCmd(ctx, "compose", "-f", haComposeDir+ComposeFile(), "rm", "-f", "-s", haNodeServices[2]); err != nil {
		t.Fatal(err)
	}

	if err := dockerCmd(ctx, "volume", "rm", "ha_data3"); err != nil {
		t.Fatal(err)
	}

	if err := startSEPPNode(ctx, dc, c1, 3, peers); err != nil {
		t.Fatalf("join a new node 3: %v", err)
	}

	if err := waitForClusterReady(ctx, clients); err != nil {
		t.Fatal(err)
	}

	rejoinedID, err := nodeIDOf(ctx, c3)
	if err != nil || rejoinedID == "" || rejoinedID == node3 {
		t.Fatalf("node ID of the new node 3 = %q, %v (was %q)", rejoinedID, err, node3)
	}

	if _, err := waitSEPPStatus(ctx, all, func(sts map[int]*client.SEPPStatus) error {
		c, ok := storedSEPPCertificate(sts[3], 3)
		if !ok || c.Source != "generated" || c.FQDN != ellaSEPPFQDN(3) || c.Fingerprint == generated3.Fingerprint {
			return fmt.Errorf("SEPP 3 after the new node joined = %+v", c)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	HALogf(t, "new node %s holds SEPP 3 with a new key", rejoinedID)

	if err := waitN32(ctx, all, created.ID, map[int]string{1: "established", 3: "established"}); err != nil {
		t.Fatalf("the new node 3 did not handshake: %v", err)
	}

	HALog(t, "the handshake trigger runs on the node that serves it")

	if hs, err := c2.RehandshakeRoamingPartner(ctx, created.ID); err == nil {
		t.Fatalf("handshake on the node without interfaces.n32 = %+v", hs)
	}

	for seppID, c := range map[int]*client.Client{1: c1, 3: c3} {
		if hs, err := c.RehandshakeRoamingPartner(ctx, created.ID); err != nil || hs.N32.State == "unknown" {
			t.Fatalf("handshake on SEPP %d = %+v, %v", seppID, hs, err)
		}
	}

	if err := waitN32(ctx, all, created.ID, map[int]string{1: "established", 3: "established"}); err != nil {
		t.Fatal(err)
	}

	HALog(t, "deleting the partner closes N32 on every node")

	if err := waitUntil(ctx, seppWaitHandshake, func() error {
		if _, ok := partner.other(ellaSEPPFQDN(3)); !ok || partner.context().State != sepp.StateEstablished {
			return fmt.Errorf("partner context = %+v", partner.context())
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := c3.DeleteRoamingPartner(ctx, created.ID); err != nil {
		t.Fatalf("delete the partner: %v", err)
	}

	if _, err := c1.GetRoamingPartner(ctx, created.ID); err == nil {
		t.Fatal("the partner still exists")
	}

	if err := waitUntil(ctx, 30*time.Second, func() error {
		if status, _, err := partner.sendCallback(ctx, 1); err == nil && status < http.StatusBadRequest {
			return fmt.Errorf("N32-f after the deletion = %d", status)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
