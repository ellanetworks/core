//go:build open5gs

// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

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
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const (
	r5gDir             = "compose/roaming5g"
	r5gFile            = "docker-compose.yaml"
	r5gProject         = "ella-roaming5g"
	r5gTesterN2        = "10.150.0.20"
	r5gTesterN3        = "10.153.0.20"
	r5gUDM             = "10.150.0.34"
	r5gPingDestination = "10.156.0.3"
	r5gKey             = "465B5CE8B199B49FAA5F0A2EE238A6BC"
	r5gOPc             = "E8ED289DEBA952E4283B54E88E6183CA"
	r5gVisitedMCC      = "999"
	r5gVisitedMNC      = "70"
	r5gProfile         = "roamers"
	r5gPartner         = "open5gs"
	r5gHomeSEPP        = "sepp1.5gc.mnc001.mcc001.3gppnetwork.org"
	r5gMarker          = "ROAMER_REGISTERED"
	r5gN32Port         = 7443
	r5gTesterTimeout   = 8 * time.Minute
)

type r5gStack struct {
	t       *testing.T
	ctx     context.Context
	dc      *DockerClient
	nodes   []*client.Client
	api     *client.Client
	partner string
	home    tls.Certificate
}

func r5gNodeIP(n int) string { return fmt.Sprintf("10.150.0.%d", 10+n) }

func r5gRoamer(i int) string { return fmt.Sprintf("00101000000020%d", i) }

func r5gEllaSEPP(n int) string { return fmt.Sprintf("sepp%d.5gc.mnc070.mcc999.3gppnetwork.org", n) }

func r5gCompose(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", append([]string{"compose", "-f", r5gFile}, args...)...)
	cmd.Dir = r5gDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose %s: %w", strings.Join(args, " "), err)
	}

	return nil
}

func r5gWriteFile(rel string, body string) error {
	path, err := filepath.Abs(filepath.Join(r5gDir, rel))
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o777); err != nil {
		return err
	}

	if err := os.Chmod(dir, 0o777); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(body), 0o644)
}

func r5gWriteNodeConfig(n, nodes int, joinToken string) error {
	ip := r5gNodeIP(n)

	var clusterYAML strings.Builder

	if nodes > 1 {
		fmt.Fprintf(&clusterYAML, "cluster:\n  bind-address: \"%s:7000\"\n  peers:\n", ip)

		for i := 1; i <= nodes; i++ {
			fmt.Fprintf(&clusterYAML, "    - \"%s:7000\"\n", r5gNodeIP(i))
		}

		if joinToken != "" {
			fmt.Fprintf(&clusterYAML, "  join-token: %q\n", joinToken)
		}
	}

	return r5gWriteFile(fmt.Sprintf("cfg/node%d/core.yaml", n), fmt.Sprintf(`logging:
  system:
    level: "debug"
    output: "stdout"
  audit:
    output: "stdout"
db:
  path: "/data/ella.db"
interfaces:
  n2:
    address: %[1]q
    port: 38412
  n3:
    address: "10.153.0.%[2]d"
  n6:
    name: "n6"
  n32:
    address: %[1]q
    port: %[4]d
  api:
    address: %[1]q
    port: 5002
datapath:
  attach-mode: "xdp-generic"
%[3]s`, ip, 10+n, clusterYAML.String(), r5gN32Port))
}

func r5gIssue(template, parent *x509.Certificate, pub any, signer *ecdsa.PrivateKey) (*x509.Certificate, error) {
	der, err := x509.CreateCertificate(rand.Reader, template, parent, pub, signer)
	if err != nil {
		return nil, err
	}

	return x509.ParseCertificate(der)
}

func (s *r5gStack) writeHomeSEPP(ellaCA string, nodes int) {
	t := s.t
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{Organization: []string{"Open5GS"}, CommonName: "Open5GS Root CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	ca, err := r5gIssue(caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	leaf, err := r5gIssue(&x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano() + 1),
		Subject:      pkix.Name{Organization: []string{"Open5GS"}, CommonName: r5gHomeSEPP},
		DNSNames:     []string{r5gHomeSEPP},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw})

	s.home = tls.Certificate{Certificate: [][]byte{leaf.Raw, ca.Raw}, PrivateKey: key, Leaf: leaf}

	var clients strings.Builder

	for n := 1; n <= nodes; n++ {
		fmt.Fprintf(&clients, `        - receiver: %[1]s
          uri: https://%[1]s:%[2]d
          target_plmn_id:
            mcc: 999
            mnc: 70
          cacert: /sepp/ella-ca.crt
          client_cert: /sepp/open5gs-sepp.crt
          client_private_key: /sepp/open5gs-sepp.key
`, r5gEllaSEPP(n), r5gN32Port)
	}

	config := fmt.Sprintf(`logger:
  level: debug
global:
  max:
    ue: 64
sepp:
  serving:
    - plmn_id:
        mcc: 001
        mnc: 01
  sbi:
    server:
      - address: %[1]s
        port: 80
    client:
      nrf:
        - uri: http://nrf.5gc.mnc001.mcc001.3gppnetwork.org
  n32:
    server:
      - sender: %[1]s
        scheme: https
        address: 0.0.0.0
        port: 7777
        private_key: /sepp/open5gs-sepp.key
        cert: /sepp/open5gs-sepp.crt
        verify_client: true
        verify_client_cacert: /sepp/ella-ca.crt
    client:
      sepp:
%[2]s`, r5gHomeSEPP, clients.String())

	for name, body := range map[string]string{
		"cfg/sepp/open5gs-sepp.crt": string(certPEM),
		"cfg/sepp/open5gs-sepp.key": string(keyPEM),
		"cfg/sepp/open5gs-ca.crt":   string(caPEM),
		"cfg/sepp/ella-ca.crt":      ellaCA,
		"cfg/sepp/sepp.yaml":        config,
	} {
		if err := r5gWriteFile(name, body); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func r5gBringUp(t *testing.T, nodes int) *r5gStack {
	t.Helper()

	if DetectIPFamily() != IPv4Only {
		t.Skip("the 5G roaming e2e runs in IPv4 mode only")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	t.Cleanup(cancel)

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	_ = r5gCompose(ctx, "down", "-v", "--remove-orphans")

	s := &r5gStack{t: t, ctx: ctx, dc: dc}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		if t.Failed() || os.Getenv("ROAMING5G_LOGS") != "" {
			services := []string{"sepp", "udm", "ausf"}
			for i := 1; i <= nodes; i++ {
				services = append(services, fmt.Sprintf("ella-core-%d", i))
			}

			for _, svc := range services {
				logs, err := dc.ComposeLogs(cleanupCtx, r5gDir, svc)
				if err != nil {
					t.Logf("=== %s logs: %v ===", svc, err)
					continue
				}

				t.Logf("=== %s logs ===\n%s", svc, logs)
			}
		}

		if os.Getenv("ROAMING5G_KEEP") == "" {
			_ = r5gCompose(cleanupCtx, "down", "-v", "--remove-orphans")
		}
	})

	if err := r5gWriteNodeConfig(1, nodes, ""); err != nil {
		t.Fatalf("write node 1 config: %v", err)
	}

	if err := r5gCompose(ctx, "up", "-d", "ella-core-1"); err != nil {
		t.Fatal(err)
	}

	node1, err := client.New(&client.Config{BaseURL: "http://" + r5gNodeIP(1) + ":5002"})
	if err != nil {
		t.Fatal(err)
	}

	token, err := initializeAndGetAdminToken(ctx, node1)
	if err != nil {
		t.Fatalf("initialize node 1: %v", err)
	}

	node1.SetToken(token)
	s.nodes = []*client.Client{node1}

	for n := 2; n <= nodes; n++ {
		tok, err := node1.MintClusterJoinToken(ctx, &client.MintJoinTokenOptions{TTLSeconds: 600})
		if err != nil {
			t.Fatalf("mint join token for node %d: %v", n, err)
		}

		if err := r5gWriteNodeConfig(n, nodes, tok.Token); err != nil {
			t.Fatalf("write node %d config: %v", n, err)
		}

		if err := r5gCompose(ctx, "up", "-d", fmt.Sprintf("ella-core-%d", n)); err != nil {
			t.Fatal(err)
		}

		c, err := client.New(&client.Config{BaseURL: fmt.Sprintf("http://%s:5002", r5gNodeIP(n))})
		if err != nil {
			t.Fatal(err)
		}

		c.SetToken(token)
		s.nodes = append(s.nodes, c)
	}

	s.api = node1

	if nodes > 1 {
		if err := waitForClusterReady(ctx, s.nodes); err != nil {
			t.Fatalf("cluster not ready: %v", err)
		}

		if err := waitForAllNodesReady(ctx, s.nodes); err != nil {
			t.Fatalf("nodes not ready: %v", err)
		}
	}

	s.provision()

	ellaCA := s.ellaTrustAnchor(nodes)
	s.writeHomeSEPP(ellaCA, nodes)

	if err := r5gCompose(ctx, "up", "-d", "mongodb", "provision", "nrf", "udr", "udm", "ausf", "sepp"); err != nil {
		t.Fatal(err)
	}

	s.createPartner()

	for n := 1; n <= nodes; n++ {
		s.waitN32(n)
	}

	if err := r5gCompose(ctx, "up", "-d", "ella-core-tester", "router"); err != nil {
		t.Fatal(err)
	}

	return s
}

func (s *r5gStack) provision() {
	t := s.t
	t.Helper()

	if err := s.api.UpdateOperatorID(s.ctx, &client.UpdateOperatorIDOptions{Mcc: r5gVisitedMCC, Mnc: r5gVisitedMNC}); err != nil {
		t.Fatalf("update operator ID: %v", err)
	}

	if err := s.api.UpdateOperatorTracking(s.ctx, &client.UpdateOperatorTrackingOptions{SupportedTacs: []string{scenarios.DefaultTAC}}); err != nil {
		t.Fatalf("update operator tracking: %v", err)
	}

	for i, c := range s.nodes {
		if err := c.UpdateNATInfo(s.ctx, &client.UpdateNATInfoOptions{Enabled: true}); err != nil {
			t.Fatalf("enable NAT on node %d: %v", i+1, err)
		}
	}

	fx := fixture.New(t, s.ctx, s.api)
	fx.Slice(fixture.DefaultSliceSpec())
	fx.DataNetwork(fixture.DefaultDataNetworkSpec())
	fx.Profile(fixture.ProfileSpec{Name: r5gProfile, UeAmbrUplink: "500 Mbps", UeAmbrDownlink: "500 Mbps"})
	fx.Policy(fixture.PolicySpec{
		Name: "roamers-internet", ProfileName: r5gProfile, SliceName: scenarios.DefaultSliceName,
		DataNetworkName: scenarios.DefaultDNN, SessionAmbrUplink: "200 Mbps", SessionAmbrDownlink: "200 Mbps",
		Var5qi: 9, Arp: 15,
	})
}

func (s *r5gStack) ellaTrustAnchor(nodes int) string {
	t := s.t
	t.Helper()

	var anchor string

	err := r4gPoll(s.ctx, 2*time.Minute, func() error {
		for i, c := range s.nodes[:nodes] {
			st, err := c.GetSEPPStatus(s.ctx)
			if err != nil {
				return err
			}

			if len(st.TrustAnchors) != 1 || !st.Node.Listening || st.Node.Certificate == nil {
				return fmt.Errorf("SEPP status of node %d: %d trust anchors, listening %t, certificate %t", i+1, len(st.TrustAnchors), st.Node.Listening, st.Node.Certificate != nil)
			}

			anchor = st.TrustAnchors[0].PEM
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Ella SEPP identity: %v", err)
	}

	return anchor
}

func (s *r5gStack) createPartner() {
	t := s.t
	t.Helper()

	caPEM, err := os.ReadFile(filepath.Join(r5gDir, "cfg/sepp/open5gs-ca.crt"))
	if err != nil {
		t.Fatal(err)
	}

	partner, err := s.api.CreateRoamingPartner(s.ctx, &client.RoamingPartnerOptions{
		Name: r5gPartner, PLMNs: []client.RoamingPLMN{{Mcc: "001", Mnc: "01"}}, ProfileName: r5gProfile,
		SEPP: &client.RoamingPartnerSEPP{
			FQDN:           r5gHomeSEPP,
			N32cURI:        "https://" + r5gHomeSEPP + ":7777",
			TrustAnchorPEM: string(caPEM),
			NRFAPIRoot:     "http://nrf.5gc.mnc001.mcc001.3gppnetwork.org",
		},
	})
	if err != nil {
		t.Fatalf("create roaming partner: %v", err)
	}

	s.partner = partner.ID
}

func (s *r5gStack) waitN32(node int) {
	s.t.Helper()

	err := r4gPoll(s.ctx, 3*time.Minute, func() error {
		p, err := s.nodes[node-1].GetRoamingPartner(s.ctx, s.partner)
		if err != nil {
			return err
		}

		if p.N32 != nil && p.N32.State == "established" {
			return nil
		}

		return fmt.Errorf("N32 context %+v", p.N32)
	})
	if err != nil {
		s.t.Fatalf("node %d N32 context: %v", node, err)
	}
}

type r5gTesterRun struct {
	done   chan error
	marker chan struct{}
}

func (s *r5gStack) tester(node int, imsi string, args ...string) *r5gTesterRun {
	s.t.Helper()

	container, err := s.dc.ResolveComposeContainer(s.ctx, r5gProject, "ella-core-tester")
	if err != nil {
		s.t.Fatalf("resolve tester container: %v", err)
	}

	argv := []string{
		"core-tester", "run", "gnb/roaming/register",
		"--ella-core-n2-address", r5gNodeIP(node) + ":38412",
		"--gnb", fmt.Sprintf("gnb1,n2=%s,n3=%s", r5gTesterN2, r5gTesterN3),
		"--imsi", imsi, "--key", r5gKey, "--opc", r5gOPc,
		"--mcc", r5gVisitedMCC, "--mnc", r5gVisitedMNC,
		"--ping-destination", r5gPingDestination,
		"--verbose",
	}
	argv = append(argv, args...)

	run := &r5gTesterRun{done: make(chan error, 1), marker: make(chan struct{})}
	writer := newMarkerWriter(s.t, r5gMarker, run.marker)

	go func() {
		_, err := s.dc.Exec(s.ctx, container, argv, false, r5gTesterTimeout, writer)
		run.done <- err
	}()

	return run
}

func (r *r5gTesterRun) wait(t *testing.T, what string) {
	t.Helper()

	if err := <-r.done; err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

func (r *r5gTesterRun) registered(t *testing.T, what string) {
	t.Helper()

	select {
	case <-r.marker:
	case err := <-r.done:
		t.Fatalf("%s ended before the registration: %v", what, err)
	case <-time.After(2 * time.Minute):
		t.Fatalf("%s: no registration within 2 minutes", what)
	}
}

type r5gUDMRegistration struct {
	AmfInstanceID    string `json:"amfInstanceId"`
	DeregCallbackURI string `json:"deregCallbackUri"`
	PurgeFlag        bool   `json:"purgeFlag"`
	Guami            struct {
		AmfID string `json:"amfId"`
	} `json:"guami"`
}

func r5gH2C() *http.Client {
	tr := &http.Transport{}
	tr.Protocols = new(http.Protocols)
	tr.Protocols.SetUnencryptedHTTP2(true)

	return &http.Client{Transport: tr, Timeout: 10 * time.Second}
}

func (s *r5gStack) udmRegistration(imsi string) (r5gUDMRegistration, int, error) {
	req, err := http.NewRequestWithContext(s.ctx, http.MethodGet, fmt.Sprintf("http://%s/nudm-uecm/v1/imsi-%s/registrations/amf-3gpp-access", r5gUDM, imsi), nil)
	if err != nil {
		return r5gUDMRegistration{}, 0, err
	}

	req.Host = "udm.5gc.mnc001.mcc001.3gppnetwork.org"

	resp, err := r5gH2C().Do(req)
	if err != nil {
		return r5gUDMRegistration{}, 0, err
	}

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	var reg r5gUDMRegistration

	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(body, &reg); err != nil {
			return reg, resp.StatusCode, fmt.Errorf("parse %s: %w", body, err)
		}
	}

	return reg, resp.StatusCode, nil
}

func (s *r5gStack) waitUDM(imsi, what string, ok func(r5gUDMRegistration, int) bool) r5gUDMRegistration {
	s.t.Helper()

	var reg r5gUDMRegistration

	err := r4gPoll(s.ctx, 30*time.Second, func() error {
		r, status, err := s.udmRegistration(imsi)
		if err != nil {
			return err
		}

		reg = r

		if ok(r, status) {
			return nil
		}

		return fmt.Errorf("UDM registration %+v (HTTP %d)", r, status)
	})
	if err != nil {
		s.t.Fatalf("%s: %v", what, err)
	}

	return reg
}

func (s *r5gStack) notify(ownerNode int, imsi, kind string, body any) (int, string) {
	const node = 1

	s.t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		s.t.Fatal(err)
	}

	url := fmt.Sprintf("https://%s:%d/roaming-callbacks/v1/imsi-%s/%s", r5gNodeIP(node), r5gN32Port, imsi, kind)

	req, err := http.NewRequestWithContext(s.ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		s.t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("3gpp-Sbi-Target-apiRoot", fmt.Sprintf("https://amf%d.5gc.mnc070.mcc999.3gppnetwork.org", ownerNode))

	c := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{ForceAttemptHTTP2: true, TLSClientConfig: &tls.Config{
		MinVersion:         tls.VersionTLS12,
		NextProtos:         []string{"h2"},
		Certificates:       []tls.Certificate{s.home},
		ServerName:         r5gEllaSEPP(node),
		InsecureSkipVerify: true, //nolint:gosec
	}}}

	resp, err := c.Do(req)
	if err != nil {
		s.t.Fatalf("N32-f callback to node %d: %v", node, err)
	}

	defer func() { _ = resp.Body.Close() }()

	out, _ := io.ReadAll(resp.Body)

	return resp.StatusCode, string(out)
}

func (s *r5gStack) registered(node int) float64 {
	return sessionMetricSeries(s.t, s.ctx, s.nodes[node-1], "app_registered_subscribers", `rat="5g"`, `roaming="true"`)
}

func (s *r5gStack) waitRegistered(node int, want float64, what string) {
	s.t.Helper()

	err := r4gPoll(s.ctx, 45*time.Second, func() error {
		if got := s.registered(node); got != want {
			return fmt.Errorf("node %d has %v registered 5G roamers, want %v", node, got, want)
		}

		return nil
	})
	if err != nil {
		s.t.Fatalf("%s: %v", what, err)
	}
}

func (s *r5gStack) sessionFlags() []string {
	if os.Getenv("ROAMING5G_NO_SESSION") != "" {
		return []string{"--session=false", "--ping=false"}
	}

	return nil
}

func TestRoaming5GOpen5GS(t *testing.T) {
	s := r5gBringUp(t, 1)

	t.Run("unknown PLMN is rejected with #11", func(t *testing.T) {
		s.tester(1, "001020000000001", "--home-mnc", "02", "--expect-reject", "11").wait(t, "unknown PLMN")
	})

	t.Run("registration through the home AUSF and UDM, LBO data, deregistration with purge", func(t *testing.T) {
		imsi := r5gRoamer(1)
		run := s.tester(1, imsi, s.sessionFlags()...)
		run.registered(t, "roamer registration")

		reg := s.waitUDM(imsi, "UECM registration at the home UDM", func(r r5gUDMRegistration, status int) bool {
			return status == http.StatusOK && !r.PurgeFlag
		})

		if !strings.HasPrefix(reg.DeregCallbackURI, "https://amf1.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/imsi-"+imsi) || reg.AmfInstanceID == "" {
			t.Fatalf("UECM registration %+v", reg)
		}

		run.wait(t, "roamer registration")

		s.waitUDM(imsi, "purge at the home UDM after the deregistration", func(r r5gUDMRegistration, status int) bool {
			return status == http.StatusOK && r.PurgeFlag
		})
	})

	t.Run("authentication resynchronisation with the home AUSF", func(t *testing.T) {
		s.tester(1, r5gRoamer(2), append([]string{"--sqn", "000000100000"}, s.sessionFlags()...)...).wait(t, "resync registration")
	})

	t.Run("UDM-initiated deregistration (SUBSCRIPTION_WITHDRAWN) of a connected roamer over N32", func(t *testing.T) {
		imsi := r5gRoamer(3)
		run := s.tester(1, imsi, "--ping=false", "--await-network-deregistration")
		run.registered(t, "roamer registration")

		s.waitUDM(imsi, "UECM registration", func(r r5gUDMRegistration, status int) bool { return status == http.StatusOK && !r.PurgeFlag })

		_, err := s.api.GetLocation(s.ctx, &client.GetLocationOptions{SUPI: "imsi-" + imsi, Method: "cell_id"})
		if err == nil || !strings.Contains(err.Error(), "server error 403") {
			t.Fatalf("locating the roamer: %v, want 403: Open5GS has no LCS privacy profile", err)
		}

		status, body := s.notify(1, imsi, "uecm-dereg", map[string]string{"deregReason": "SUBSCRIPTION_WITHDRAWN", "accessType": "3GPP_ACCESS"})
		if status != http.StatusNoContent {
			t.Fatalf("deregistration notification: %d %s", status, body)
		}

		run.wait(t, "network-initiated deregistration")
		s.waitRegistered(1, 0, "the withdrawn roamer is gone")

		if reg, _, err := s.udmRegistration(imsi); err != nil || reg.PurgeFlag {
			t.Fatalf("the AMF purged a registration the home UDM had withdrawn: %+v %v", reg, err)
		}
	})

	t.Run("UDM-initiated deregistration of an idle roamer is implicit", func(t *testing.T) {
		imsi := r5gRoamer(6)
		s.tester(1, imsi, "--session=false", "--ping=false", "--deregister=false").wait(t, "roamer registration")
		s.waitRegistered(1, 1, "the idle roamer stays registered")

		status, body := s.notify(1, imsi, "uecm-dereg", map[string]string{"deregReason": "SUBSCRIPTION_WITHDRAWN"})
		if status != http.StatusNoContent {
			t.Fatalf("deregistration notification: %d %s", status, body)
		}

		s.waitRegistered(1, 0, "the idle roamer is removed")

		if status, _ := s.notify(1, imsi, "uecm-dereg", map[string]string{"deregReason": "SUBSCRIPTION_WITHDRAWN"}); status != http.StatusNotFound {
			t.Fatalf("notification for a removed roamer: %d, want 404", status)
		}

		change := map[string]any{"notifyItems": []map[string]any{{"resourceId": "imsi-" + imsi + "/am-data", "changes": []map[string]string{{"op": "REPLACE", "path": "/subscribedUeAmbr"}}}}}
		if status, body := s.notify(1, imsi, "sdm-change", change); status != http.StatusNotFound || !strings.Contains(body, "SUBSCRIPTION_NOT_FOUND") {
			t.Fatalf("data change for a removed roamer: %d %s, want 404 SUBSCRIPTION_NOT_FOUND", status, body)
		}

		if reg, _, err := s.udmRegistration(imsi); err != nil || reg.PurgeFlag {
			t.Fatalf("the AMF purged a registration the home UDM had withdrawn: %+v %v", reg, err)
		}
	})

	t.Run("deleting the partner deregisters its roamers with #11", func(t *testing.T) {
		imsi := r5gRoamer(4)
		run := s.tester(1, imsi, "--ping=false", "--await-network-deregistration", "--expect-dereg-cause", "11")
		run.registered(t, "roamer registration")

		if err := s.api.DeleteRoamingPartner(s.ctx, s.partner); err != nil {
			t.Fatalf("delete the partner with a roamer: %v", err)
		}

		run.wait(t, "network-initiated deregistration")

		s.waitUDM(imsi, "purge at the home UDM after the withdrawal", func(r r5gUDMRegistration, status int) bool {
			return status == http.StatusOK && r.PurgeFlag
		})

		s.waitRegistered(1, 0, "no roamer left")
	})
}

func TestRoaming5GOpen5GSHA(t *testing.T) {
	s := r5gBringUp(t, 2)

	imsi := r5gRoamer(5)

	t.Run("a callback for another node's AMF is refused", func(t *testing.T) {
		run := s.tester(2, imsi, "--session=false", "--ping=false", "--deregister=false")
		run.wait(t, "roamer registration on node 2")

		reg := s.waitUDM(imsi, "UECM registration from node 2", func(r r5gUDMRegistration, status int) bool {
			return status == http.StatusOK && !r.PurgeFlag
		})

		if !strings.HasPrefix(reg.DeregCallbackURI, "https://amf2.") {
			t.Fatalf("callback URI %q, want node 2's AMF", reg.DeregCallbackURI)
		}

		status, body := s.notify(2, imsi, "uecm-dereg", map[string]string{"deregReason": "SUBSCRIPTION_WITHDRAWN"})
		if status != http.StatusNotFound || !strings.Contains(body, "CONTEXT_NOT_FOUND") {
			t.Fatalf("callback for node 2 sent to node 1: %d %s", status, body)
		}
	})

	t.Run("the roamer moves to another node", func(t *testing.T) {
		s.tester(1, imsi, "--session=false", "--ping=false", "--deregister=false", "--gnb-index", "1").wait(t, "roamer registration on node 1")

		reg := s.waitUDM(imsi, "UECM registration from node 1", func(r r5gUDMRegistration, status int) bool {
			return status == http.StatusOK && strings.HasPrefix(r.DeregCallbackURI, "https://amf1.")
		})

		if reg.PurgeFlag {
			t.Fatalf("UDM registration purged after the move: %+v", reg)
		}

		s.waitRegistered(2, 0, "node 2 releases the superseded roamer")

		time.Sleep(15 * time.Second)

		if reg, _, err := s.udmRegistration(imsi); err != nil || reg.PurgeFlag || !strings.HasPrefix(reg.DeregCallbackURI, "https://amf1.") {
			t.Fatalf("node 2 purged the moved roamer at the home UDM: %+v %v", reg, err)
		}
	})
}
