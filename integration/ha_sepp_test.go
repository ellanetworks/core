// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"bytes"
	"context"
	"errors"
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
	tuning.BackoffMax = 5 * time.Second
	tuning.N32cRequestsPerMinute = 600

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
	url := "https://amf" + strconv.Itoa(seppID) + "." + seppEllaDomain + "/roaming-callbacks/v1/" + seppPartnerSUPI + "/uecm-dereg"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte(`{"deregReason":"SUBSCRIPTION_WITHDRAWN"}`)))
	if err != nil {
		return 0, wire.ProblemDetails{}, err
	}

	req.Header.Set("Content-Type", wire.ContentTypeJSON)

	resp, err := p.m.Client("ella").Do(req)
	if err != nil {
		return 0, wire.ProblemDetails{}, err
	}

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	return resp.StatusCode, wire.ParseProblem(resp.StatusCode, body), nil
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

func seppNodesByID(st *client.SEPPStatus) map[int]client.SEPPNodeStatus {
	out := map[int]client.SEPPNodeStatus{}
	for _, n := range st.Nodes {
		out[n.SEPPID] = n
	}

	return out
}

func partnerN32BySEPP(ctx context.Context, c *client.Client, id string) (map[int]client.RoamingPartnerN32, error) {
	p, err := c.GetRoamingPartner(ctx, id)
	if err != nil {
		return nil, err
	}

	out := map[int]client.RoamingPartnerN32{}
	for _, n := range p.N32 {
		out[n.SEPPID] = n
	}

	return out, nil
}

func waitN32(ctx context.Context, c *client.Client, id string, want map[int]string) (map[int]client.RoamingPartnerN32, error) {
	var got map[int]client.RoamingPartnerN32

	err := waitUntil(ctx, seppWaitHandshake, func() error {
		var err error

		got, err = partnerN32BySEPP(ctx, c, id)
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

	return got, err
}

func generatedAnchor(st *client.SEPPStatus) (client.SEPPTrustAnchor, bool) {
	for _, a := range st.TrustAnchors {
		if a.Source == "generated" {
			return a, true
		}
	}

	return client.SEPPTrustAnchor{}, false
}

func waitSEPPStatus(ctx context.Context, c *client.Client, check func(*client.SEPPStatus) error) (*client.SEPPStatus, error) {
	var st *client.SEPPStatus

	err := waitUntil(ctx, seppWaitHandshake, func() error {
		var err error

		st, err = c.GetSEPPStatus(ctx)
		if err != nil {
			return err
		}

		return check(st)
	})

	return st, err
}

func otherNodeRejections(node int) (int, error) {
	body, err := scrapeMetrics(APIAddressForCluster(node))
	if err != nil {
		return 0, err
	}

	for line := range strings.SplitSeq(string(body), "\n") {
		if strings.HasPrefix(line, "app_sepp_n32f_rejected_total{") && strings.Contains(line, `partner="`+seppPartnerName+`"`) && strings.Contains(line, `reason="other_node"`) {
			fields := strings.Fields(line)

			v, err := strconv.ParseFloat(fields[len(fields)-1], 64)
			if err != nil {
				return 0, err
			}

			return int(v), nil
		}
	}

	return 0, nil
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

	HALog(t, "every node with interfaces.n32 gets a certificate signed by the cluster SEPP CA, without operator action")

	initial, err := waitSEPPStatus(ctx, c2, func(st *client.SEPPStatus) error {
		anchor, ok := generatedAnchor(st)
		if !ok || len(st.TrustAnchors) != 1 {
			return fmt.Errorf("trust anchors = %+v", st.TrustAnchors)
		}

		nodes := seppNodesByID(st)
		if len(nodes) != 3 {
			return fmt.Errorf("nodes = %+v", st.Nodes)
		}

		for seppID, n32 := range seppNodesWithN32 {
			n := nodes[seppID]

			switch {
			case n.NodeID == "" || !n.Reachable || n.N32Configured != n32:
				return fmt.Errorf("SEPP %d = %+v", seppID, n)
			case n32 && (n.Source != "generated" || n.FQDN != ellaSEPPFQDN(seppID) || n.Certificate == nil || n.Certificate.TrustAnchorFingerprint != anchor.Fingerprint || !n.Listening || n.Error != ""):
				return fmt.Errorf("SEPP %d = %+v", seppID, n)
			case !n32 && (n.Certificate != nil || n.Source != ""):
				return fmt.Errorf("SEPP %d has a certificate without interfaces.n32: %+v", seppID, n)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("SEPP status: %v", err)
	}

	clusterCA, _ := generatedAnchor(initial)
	initialNodes := seppNodesByID(initial)

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

	established, err := waitN32(ctx, c2, created.ID, map[int]string{1: "established", 3: "established"})
	if err != nil {
		t.Fatalf("partner N32 from the node without interfaces.n32: %v", err)
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

	for _, c := range []*client.Client{c1, c3} {
		got, err := partnerN32BySEPP(ctx, c, created.ID)
		if err != nil || len(got) != 2 || got[1].State != "established" || got[3].State != "established" {
			t.Fatalf("partner N32 aggregated from another node = %+v, %v", got, err)
		}
	}

	HALog(t, "a callback for node 3's AMF sent to node 1 gets 404 and counts as other_node")

	before, err := otherNodeRejections(1)
	if err != nil {
		t.Fatal(err)
	}

	status, pd, err := partner.sendCallback(ctx, 3)
	if err != nil || status != http.StatusNotFound || pd.Cause != wire.CauseContextNotFound {
		t.Fatalf("callback for another node = %d %+v, %v", status, pd, err)
	}

	if after, err := otherNodeRejections(1); err != nil || after != before+1 {
		t.Fatalf("other_node rejections on node 1 = %d (was %d), %v", after, before, err)
	}

	HALog(t, "a node failure leaves the other node's context untouched and shows the node as unreachable")

	partner3, _ := partner.other(ellaSEPPFQDN(3))

	if err := dc.ComposeStopWithFile(ctx, haComposeDir, ComposeFile(), haNodeServices[2]); err != nil {
		t.Fatal(err)
	}

	if _, err := waitN32(ctx, c1, created.ID, map[int]string{1: "established", 3: "unknown"}); err != nil {
		t.Fatalf("partner N32 with node 3 down: %v", err)
	}

	if _, err := waitSEPPStatus(ctx, c2, func(st *client.SEPPStatus) error {
		n := seppNodesByID(st)[3]
		if n.Reachable || n.Error == "" || n.NodeID != initialNodes[3].NodeID || n.Certificate == nil {
			return fmt.Errorf("SEPP 3 while its node is down = %+v", n)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if got, _ := partnerN32BySEPP(ctx, c1, created.ID); got[1].Since != established[1].Since {
		t.Fatalf("node 3's failure changed node 1's context: %+v, was %+v", got[1], established[1])
	}

	if st := partner.context(); st.State != sepp.StateEstablished {
		t.Fatalf("partner context with node 1 after node 3's failure = %+v", st)
	}

	if err := dc.ComposeStartWithFile(ctx, haComposeDir, ComposeFile(), haNodeServices[2]); err != nil {
		t.Fatal(err)
	}

	if err := waitForClusterReady(ctx, clients); err != nil {
		t.Fatal(err)
	}

	if _, err := waitN32(ctx, c1, created.ID, map[int]string{1: "established", 3: "established"}); err != nil {
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

	if got, _ := partnerN32BySEPP(ctx, c1, created.ID); got[1].Since != established[1].Since {
		t.Fatalf("node 3's restart changed node 1's context: %+v, was %+v", got[1], established[1])
	}

	home := pki.PLMN{MCC: "001", MNC: "01"}

	operatorCA, operatorKey, err := pki.GenerateSEPPCA(home)
	if err != nil {
		t.Fatal(err)
	}

	operatorCAPEM := string(pki.EncodeCertPEM(operatorCA))

	HALog(t, "regenerating the cluster SEPP CA reissues every generated certificate; every node handshakes again once the partner trusts the new CA")

	regenerated, err := c2.RegenerateSEPPCA(ctx)
	if err != nil {
		t.Fatalf("regenerate the SEPP CA: %v", err)
	}

	if len(regenerated.Reissued) != 2 || regenerated.TrustAnchor.Fingerprint == clusterCA.Fingerprint {
		t.Fatalf("regenerate = %+v", regenerated)
	}

	if _, err := waitN32(ctx, c1, created.ID, map[int]string{1: "failed", 3: "failed"}); err != nil {
		t.Fatalf("handshakes with a CA the partner does not trust: %v", err)
	}

	partner.setEllaAnchor(regenerated.TrustAnchor.PEM + operatorCAPEM)

	rotated, err := waitN32(ctx, c3, created.ID, map[int]string{1: "established", 3: "established"})
	if err != nil {
		t.Fatalf("handshakes after the partner imported the new CA: %v", err)
	}

	if _, err := waitSEPPStatus(ctx, c1, func(st *client.SEPPStatus) error {
		if len(st.TrustAnchors) != 1 || st.TrustAnchors[0].Fingerprint != regenerated.TrustAnchor.Fingerprint {
			return fmt.Errorf("trust anchors = %+v", st.TrustAnchors)
		}

		for _, seppID := range []int{1, 3} {
			n := seppNodesByID(st)[seppID]
			if n.Certificate == nil || n.Certificate.Fingerprint == initialNodes[seppID].Certificate.Fingerprint || n.Certificate.TrustAnchorFingerprint != regenerated.TrustAnchor.Fingerprint {
				return fmt.Errorf("SEPP %d after the CA regeneration = %+v", seppID, n)
			}
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

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

	if _, err := waitSEPPStatus(ctx, c2, func(st *client.SEPPStatus) error {
		if len(st.TrustAnchors) != 2 {
			return fmt.Errorf("trust anchors = %+v", st.TrustAnchors)
		}

		nodes := seppNodesByID(st)
		if nodes[3].Source != "imported" || nodes[3].Certificate == nil || nodes[3].Certificate.Fingerprint != imported.Fingerprint || nodes[1].Source != "generated" {
			return fmt.Errorf("nodes = %+v", st.Nodes)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := waitUntil(ctx, seppWaitHandshake, func() error {
		got, err := partnerN32BySEPP(ctx, c1, created.ID)
		if err != nil {
			return err
		}

		if got[3].State != "established" || got[3].Since == rotated[3].Since {
			return fmt.Errorf("SEPP 3 = %+v", got[3])
		}

		if got[1].Since != rotated[1].Since {
			return fmt.Errorf("the import on node 3 changed node 1's context: %+v", got[1])
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	HALog(t, "removing a member deletes its generated certificate; a node that joins with its SEPP ID gets a new one and handshakes without partner changes")

	if _, err := c1.GenerateSEPPCertificate(ctx, 3); err != nil {
		t.Fatalf("generate the certificate of SEPP 3: %v", err)
	}

	if _, err := waitN32(ctx, c1, created.ID, map[int]string{1: "established", 3: "established"}); err != nil {
		t.Fatal(err)
	}

	generated3, err := c1.GetSEPPCertificate(ctx, 3)
	if err != nil || generated3.Source != "generated" {
		t.Fatalf("SEPP 3 certificate = %+v, %v", generated3, err)
	}

	if err := dc.ComposeStopWithFile(ctx, haComposeDir, ComposeFile(), haNodeServices[2]); err != nil {
		t.Fatal(err)
	}

	if err := c1.RemoveClusterMember(ctx, client.NodeID(initialNodes[3].NodeID), true); err != nil {
		t.Fatalf("remove node 3: %v", err)
	}

	if err := waitForMemberCount(ctx, c1, 2, time.Minute); err != nil {
		t.Fatal(err)
	}

	if _, err := waitSEPPStatus(ctx, c2, func(st *client.SEPPStatus) error {
		if _, ok := seppNodesByID(st)[3]; ok || len(st.Nodes) != 2 {
			return fmt.Errorf("nodes after the removal = %+v", st.Nodes)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := c1.GetSEPPCertificate(ctx, 3); err == nil {
		t.Fatal("the generated certificate of the removed node is still stored")
	}

	if _, err := waitN32(ctx, c1, created.ID, map[int]string{1: "established"}); err != nil {
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

	rejoined, err := waitSEPPStatus(ctx, c1, func(st *client.SEPPStatus) error {
		n := seppNodesByID(st)[3]
		if n.NodeID == "" || n.NodeID == initialNodes[3].NodeID || n.Source != "generated" || n.FQDN != ellaSEPPFQDN(3) || n.Certificate == nil || n.Certificate.Fingerprint == generated3.Fingerprint {
			return fmt.Errorf("SEPP 3 after the new node joined = %+v", n)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	HALogf(t, "new node %s holds SEPP 3 with a new key", seppNodesByID(rejoined)[3].NodeID)

	if _, err := waitN32(ctx, c2, created.ID, map[int]string{1: "established", 3: "established"}); err != nil {
		t.Fatalf("the new node 3 did not handshake: %v", err)
	}

	HALog(t, "the handshake trigger runs on every node with interfaces.n32")

	hs, err := c2.RehandshakeRoamingPartner(ctx, created.ID)
	if err != nil || len(hs.N32) != 2 || hs.N32[0].SEPPID != 1 || hs.N32[1].SEPPID != 3 {
		t.Fatalf("handshake fan-out = %+v, %v", hs, err)
	}

	if _, err := waitN32(ctx, c2, created.ID, map[int]string{1: "established", 3: "established"}); err != nil {
		t.Fatal(err)
	}

	HALog(t, "deleting the partner tears N32 down on every node")

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

	if err := waitUntil(ctx, 30*time.Second, func() error {
		st := partner.context()
		if st.State == sepp.StateEstablished || len(st.OtherPeers) != 0 {
			return fmt.Errorf("partner context after the deletion = %+v", st)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := c1.GetRoamingPartner(ctx, created.ID); err == nil {
		t.Fatal("the partner still exists")
	}

	if status, _, err := partner.sendCallback(ctx, 1); !errors.Is(err, sepp.ErrNoContext) {
		t.Fatalf("N32-f after the deletion = %d, %v", status, err)
	}
}
