//go:build open5gs

// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
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
	r4gDir             = "compose/roaming4g"
	r4gFile            = "docker-compose.yaml"
	r4gProject         = "ella-roaming4g"
	r4gHSSIdentity     = "hss.epc.mnc001.mcc001.3gppnetwork.org"
	r4gHSSRealm        = "epc.mnc001.mcc001.3gppnetwork.org"
	r4gHSSAddress      = "10.130.0.30"
	r4gTesterN2        = "10.130.0.20"
	r4gTesterN3        = "10.133.0.20"
	r4gPingDestination = "10.136.0.3"
	r4gKey             = "465B5CE8B199B49FAA5F0A2EE238A6BC"
	r4gOPc             = "E8ED289DEBA952E4283B54E88E6183CA"
	r4gVisitedMCC      = "999"
	r4gVisitedMNC      = "70"
	r4gProfile         = "roamers"
	r4gPartner         = "open5gs"
	r4gMarker          = "ROAMER_ATTACHED"
	r4gTesterTimeout   = 8 * time.Minute
)

type r4gStack struct {
	t        *testing.T
	ctx      context.Context
	dc       *DockerClient
	nodes    []*client.Client
	api      *client.Client
	hosts    []string
	partner  string
	moverRun *r4gTesterRun
}

func r4gNodeIP(n int) string { return fmt.Sprintf("10.130.0.%d", 10+n) }

func r4gRoamer(i int) string { return fmt.Sprintf("00101000000010%d", i) }

func r4gCompose(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", append([]string{"compose", "-f", r4gFile}, args...)...)
	cmd.Dir = r4gDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose %s: %w", strings.Join(args, " "), err)
	}

	return nil
}

func r4gWriteFile(rel string, body string) error {
	path, err := filepath.Abs(filepath.Join(r4gDir, rel))
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

func r4gWriteNodeConfig(n int, cluster bool, joinToken string) error {
	ip := r4gNodeIP(n)

	var clusterYAML strings.Builder

	if cluster {
		fmt.Fprintf(&clusterYAML, "cluster:\n  bind-address: \"%s:7000\"\n  peers:\n", ip)

		for i := 1; i <= 3; i++ {
			fmt.Fprintf(&clusterYAML, "    - \"%s:7000\"\n", r4gNodeIP(i))
		}

		if joinToken != "" {
			fmt.Fprintf(&clusterYAML, "  join-token: %q\n", joinToken)
		}
	}

	return r4gWriteFile(fmt.Sprintf("cfg/node%d/core.yaml", n), fmt.Sprintf(`logging:
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
    address: "10.133.0.%[2]d"
  n6:
    name: "n6"
  api:
    address: %[1]q
    port: 5002
  diameter:
    address: %[1]q
datapath:
  attach-mode: "xdp-generic"
%[3]s`, ip, 10+n, clusterYAML.String()))
}

func r4gWriteHSSConfig(hosts []string) error {
	var b strings.Builder

	fmt.Fprintf(&b, `Identity = %q;
Realm = %q;
ListenOn = %q;
TLS_Cred = "/cfg/hss.crt", "/cfg/hss.key";
TLS_CA = "/cfg/hss.crt";
NoRelay;
LoadExtension = "/usr/lib/x86_64-linux-gnu/freeDiameter/dbg_msg_dumps.fdx" : "0x8888";
LoadExtension = "/usr/lib/x86_64-linux-gnu/freeDiameter/dict_rfc5777.fdx";
LoadExtension = "/usr/lib/x86_64-linux-gnu/freeDiameter/dict_mip6i.fdx";
LoadExtension = "/usr/lib/x86_64-linux-gnu/freeDiameter/dict_nasreq.fdx";
LoadExtension = "/usr/lib/x86_64-linux-gnu/freeDiameter/dict_nas_mipv6.fdx";
LoadExtension = "/usr/lib/x86_64-linux-gnu/freeDiameter/dict_dcca.fdx";
LoadExtension = "/usr/lib/x86_64-linux-gnu/freeDiameter/dict_dcca_3gpp.fdx";
`, r4gHSSIdentity, r4gHSSRealm, r4gHSSAddress)

	for i, h := range hosts {
		fmt.Fprintf(&b, "ConnectPeer = %q { ConnectTo = %q; No_TLS; };\n", h, r4gNodeIP(i+1))
	}

	certPEM, keyPEM, err := r4gSelfSigned(r4gHSSIdentity)
	if err != nil {
		return err
	}

	if err := r4gWriteFile("cfg/hss/hss.crt", certPEM); err != nil {
		return err
	}

	if err := r4gWriteFile("cfg/hss/hss.key", keyPEM); err != nil {
		return err
	}

	return r4gWriteFile("cfg/hss/hss.conf", b.String())
}

func r4gSelfSigned(cn string) (string, string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		DNSNames:              []string{cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	return string(certPEM), string(keyPEM), nil
}

func r4gBringUp(t *testing.T, nodes int) *r4gStack {
	t.Helper()

	if DetectIPFamily() != IPv4Only {
		t.Skip("the 4G roaming e2e runs in IPv4 mode only")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	t.Cleanup(cancel)

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	_ = r4gCompose(ctx, "down", "-v", "--remove-orphans")

	s := &r4gStack{t: t, ctx: ctx, dc: dc}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		if t.Failed() || os.Getenv("ROAMING4G_LOGS") != "" {
			services := []string{"hss"}
			for i := 1; i <= nodes; i++ {
				services = append(services, fmt.Sprintf("ella-core-%d", i))
			}

			for _, svc := range services {
				logs, err := dc.ComposeLogs(cleanupCtx, r4gDir, svc)
				if err != nil {
					t.Logf("=== %s logs: %v ===", svc, err)
					continue
				}

				t.Logf("=== %s logs ===\n%s", svc, logs)
			}
		}

		if os.Getenv("ROAMING4G_KEEP") == "" {
			_ = r4gCompose(cleanupCtx, "down", "-v", "--remove-orphans")
		}
	})

	if err := r4gWriteNodeConfig(1, nodes > 1, ""); err != nil {
		t.Fatalf("write node 1 config: %v", err)
	}

	if err := r4gCompose(ctx, "up", "-d", "ella-core-1"); err != nil {
		t.Fatal(err)
	}

	node1, err := client.New(&client.Config{BaseURL: "http://" + r4gNodeIP(1) + ":5002"})
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

		if err := r4gWriteNodeConfig(n, true, tok.Token); err != nil {
			t.Fatalf("write node %d config: %v", n, err)
		}

		if err := r4gCompose(ctx, "up", "-d", fmt.Sprintf("ella-core-%d", n)); err != nil {
			t.Fatal(err)
		}

		c, err := client.New(&client.Config{BaseURL: fmt.Sprintf("http://%s:5002", r4gNodeIP(n))})
		if err != nil {
			t.Fatal(err)
		}

		c.SetToken(token)
		s.nodes = append(s.nodes, c)
	}

	if nodes > 1 {
		if err := waitForClusterReady(ctx, s.nodes); err != nil {
			t.Fatalf("cluster not ready: %v", err)
		}

		if err := waitForAllNodesReady(ctx, s.nodes); err != nil {
			t.Fatalf("nodes not ready: %v", err)
		}

		urls := make([]string, 0, nodes)
		for n := 1; n <= nodes; n++ {
			urls = append(urls, fmt.Sprintf("http://%s:5002", r4gNodeIP(n)))
		}

		s.api, err = client.New(&client.Config{BaseURLs: urls})
		if err != nil {
			t.Fatal(err)
		}

		s.api.SetToken(token)
	} else {
		s.api = node1
	}

	if err := r4gCompose(ctx, "up", "-d", "ella-core-tester", "router"); err != nil {
		t.Fatal(err)
	}

	s.provision()
	s.startHSS()

	return s
}

func (s *r4gStack) provision() {
	t := s.t
	t.Helper()

	if err := s.api.UpdateOperatorID(s.ctx, &client.UpdateOperatorIDOptions{Mcc: r4gVisitedMCC, Mnc: r4gVisitedMNC}); err != nil {
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
	fx.Profile(fixture.ProfileSpec{Name: r4gProfile, UeAmbrUplink: "500 Mbps", UeAmbrDownlink: "500 Mbps"})
	fx.Policy(fixture.PolicySpec{
		Name: "roamers-internet", ProfileName: r4gProfile, SliceName: scenarios.DefaultSliceName,
		DataNetworkName: scenarios.DefaultDNN, SessionAmbrUplink: "200 Mbps", SessionAmbrDownlink: "200 Mbps",
		Var5qi: 9, Arp: 15,
	})

	peer, err := s.api.CreateRoamingDiameterPeer(s.ctx, &client.RoamingDiameterPeerOptions{
		DiameterIdentity: r4gHSSIdentity, Address: r4gHSSAddress, Port: 3868,
	})
	if err != nil {
		t.Fatalf("create roaming Diameter peer: %v", err)
	}

	partner, err := s.api.CreateRoamingPartner(s.ctx, &client.RoamingPartnerOptions{
		Name: r4gPartner, PLMNs: []client.RoamingPLMN{{Mcc: "001", Mnc: "01"}}, ProfileName: r4gProfile, DiameterPeerID: peer.ID,
	})
	if err != nil {
		t.Fatalf("create roaming partner: %v", err)
	}

	s.partner = partner.ID
}

func (s *r4gStack) startHSS() {
	t := s.t
	t.Helper()

	s.hosts = make([]string, len(s.nodes))

	for i, c := range s.nodes {
		err := r4gPoll(s.ctx, time.Minute, func() error {
			st, err := c.GetDiameterStatus(s.ctx)
			if err != nil {
				return err
			}

			if st.Host == "" {
				return errors.New("no Diameter identity yet")
			}

			s.hosts[i] = st.Host

			return nil
		})
		if err != nil {
			t.Fatalf("Diameter identity of node %d: %v", i+1, err)
		}

		t.Logf("node %d Origin-Host %s", i+1, s.hosts[i])
	}

	if err := r4gWriteHSSConfig(s.hosts); err != nil {
		t.Fatalf("write HSS config: %v", err)
	}

	if err := r4gCompose(s.ctx, "up", "-d", "hss"); err != nil {
		t.Fatal(err)
	}

	for i := range s.nodes {
		s.waitHSSPeer(i + 1)
	}
}

func (s *r4gStack) waitHSSPeer(n int) {
	s.t.Helper()

	err := r4gPoll(s.ctx, 2*time.Minute, func() error {
		st, err := s.nodes[n-1].GetDiameterStatus(s.ctx)
		if err != nil {
			return err
		}

		for _, p := range st.Peers {
			if p.Host == r4gHSSIdentity || p.Address == r4gHSSAddress {
				if p.State == "open" {
					return nil
				}

				return fmt.Errorf("HSS peer is %s (%s)", p.State, p.Error)
			}
		}

		return errors.New("no HSS peer")
	})
	if err != nil {
		s.t.Fatalf("node %d HSS peer: %v", n, err)
	}
}

func r4gPoll(ctx context.Context, timeout time.Duration, f func() error) error {
	deadline := time.Now().Add(timeout)

	for {
		err := f()
		if err == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

type r4gTesterRun struct {
	done   chan error
	marker chan struct{}
}

func (s *r4gStack) tester(node int, imsi string, args ...string) *r4gTesterRun {
	s.t.Helper()

	container, err := s.dc.ResolveComposeContainer(s.ctx, r4gProject, "ella-core-tester")
	if err != nil {
		s.t.Fatalf("resolve tester container: %v", err)
	}

	argv := []string{
		"core-tester", "run", "s1enb/roaming/attach",
		"--ella-core-n2-address", r4gNodeIP(node) + ":38412",
		"--gnb", fmt.Sprintf("gnb1,n2=%s,n3=%s", r4gTesterN2, r4gTesterN3),
		"--imsi", imsi, "--key", r4gKey, "--opc", r4gOPc,
		"--mcc", r4gVisitedMCC, "--mnc", r4gVisitedMNC,
		"--ping-destination", r4gPingDestination,
		"--verbose",
	}
	argv = append(argv, args...)

	run := &r4gTesterRun{done: make(chan error, 1), marker: make(chan struct{})}
	writer := newMarkerWriter(s.t, r4gMarker, run.marker)

	go func() {
		_, err := s.dc.Exec(s.ctx, container, argv, false, r4gTesterTimeout, writer)
		run.done <- err
	}()

	return run
}

func (r *r4gTesterRun) wait(t *testing.T, what string) {
	t.Helper()

	if err := <-r.done; err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

func (r *r4gTesterRun) attached(t *testing.T, what string) {
	t.Helper()

	select {
	case <-r.marker:
	case err := <-r.done:
		t.Fatalf("%s ended before the attach: %v", what, err)
	case <-time.After(2 * time.Minute):
		t.Fatalf("%s: no attach within 2 minutes", what)
	}
}

type r4gHSSRecord struct {
	MMEHost   string `json:"mme_host"`
	MMERealm  string `json:"mme_realm"`
	PurgeFlag bool   `json:"purge_flag"`
}

func (s *r4gStack) mongo(js string) string {
	s.t.Helper()

	container, err := s.dc.ResolveComposeContainer(s.ctx, r4gProject, "mongodb")
	if err != nil {
		s.t.Fatalf("resolve mongodb container: %v", err)
	}

	out, err := s.dc.Exec(s.ctx, container, []string{"mongosh", "--quiet", "mongodb://localhost/open5gs?replicaSet=rs0", "--eval", js}, false, 30*time.Second, nil)
	if err != nil {
		s.t.Fatalf("mongosh: %v", err)
	}

	return strings.TrimSpace(out)
}

func (s *r4gStack) hssRecord(imsi string) r4gHSSRecord {
	s.t.Helper()

	out := s.mongo(fmt.Sprintf(`const d = db.subscribers.findOne({imsi: %q}); print(JSON.stringify({mme_host: typeof d.mme_host === "string" ? d.mme_host : "", mme_realm: typeof d.mme_realm === "string" ? d.mme_realm : "", purge_flag: d.purge_flag === true}))`, imsi))

	var rec r4gHSSRecord
	if err := json.Unmarshal([]byte(out[strings.LastIndex(out, "{"):]), &rec); err != nil {
		s.t.Fatalf("parse HSS record %q: %v", out, err)
	}

	return rec
}

func (s *r4gStack) waitHSSRecord(imsi, what string, ok func(r4gHSSRecord) bool) r4gHSSRecord {
	s.t.Helper()

	var rec r4gHSSRecord

	err := r4gPoll(s.ctx, 30*time.Second, func() error {
		rec = s.hssRecord(imsi)
		if ok(rec) {
			return nil
		}

		return fmt.Errorf("HSS record %+v", rec)
	})
	if err != nil {
		s.t.Fatalf("%s: %v", what, err)
	}

	return rec
}

func (s *r4gStack) registeredEPS(node int) float64 {
	return sessionMetricSeries(s.t, s.ctx, s.nodes[node-1], "app_roaming_visitors", `rat="4g"`)
}

func (s *r4gStack) waitRegisteredEPS(node int, want float64, what string) {
	s.t.Helper()

	err := r4gPoll(s.ctx, 30*time.Second, func() error {
		if got := s.registeredEPS(node); got != want {
			return fmt.Errorf("node %d has %v registered EPS roamers, want %v", node, got, want)
		}

		return nil
	})
	if err != nil {
		s.t.Fatalf("%s: %v", what, err)
	}
}

func TestRoaming4GOpen5GS(t *testing.T) {
	s := r4gBringUp(t, 1)

	t.Run("unknown PLMN is rejected with #11", func(t *testing.T) {
		s.tester(1, "001020000000001", "--expect-reject", "11").wait(t, "unknown PLMN")
	})

	t.Run("attach, LBO data and detach with purge", func(t *testing.T) {
		imsi := r4gRoamer(1)

		s.tester(1, imsi, "--expect-apn", scenarios.DefaultDNN).wait(t, "roamer attach")

		rec := s.waitHSSRecord(imsi, "PUR after the detach", func(r r4gHSSRecord) bool { return r.PurgeFlag })
		if rec.MMEHost != s.hosts[0] {
			t.Fatalf("HSS registered MME %q, want Ella's %q", rec.MMEHost, s.hosts[0])
		}

		err := r4gPoll(s.ctx, 75*time.Second, func() error {
			usage, err := s.api.ListRoamingUsage(s.ctx, &client.ListRoamingUsageParams{IMSI: imsi})
			if err != nil {
				return err
			}

			for _, u := range usage {
				if u.Mcc == "001" && u.Mnc == "01" && u.PartnerName == r4gPartner && u.UplinkBytes > 0 && u.DownlinkBytes > 0 {
					return nil
				}
			}

			return fmt.Errorf("roaming usage %+v", usage)
		})
		if err != nil {
			t.Fatalf("roaming usage of the roamer: %v", err)
		}

		home, err := s.api.ListUsagePerSubscriber(s.ctx, &client.ListUsageParams{Subscriber: imsi})
		if err != nil {
			t.Fatalf("subscriber usage: %v", err)
		}

		for _, u := range home {
			if u.TotalBytes > 0 {
				t.Fatalf("visitor usage recorded as subscriber usage: %+v", home)
			}
		}
	})

	t.Run("authentication resynchronisation with the home HSS", func(t *testing.T) {
		s.tester(1, r4gRoamer(2), "--sqn", "000000100000", "--expect-resyncs", "1", "--ping=false").wait(t, "resync attach")
	})

	t.Run("HSS-initiated IDR updates the UE-AMBR", func(t *testing.T) {
		imsi := r4gRoamer(3)
		run := s.tester(1, imsi, "--ping=false", "--await-ue-ambr-dl", "100", "--await-ue-ambr-ul", "50")
		run.attached(t, "IDR roamer")

		s.mongo(fmt.Sprintf(`db.subscribers.updateOne({imsi: %q}, {$set: {ambr: {downlink: {value: NumberInt(100), unit: NumberInt(2)}, uplink: {value: NumberInt(50), unit: NumberInt(2)}}}})`, imsi))

		run.wait(t, "IDR roamer")
	})

	t.Run("HSS-initiated IDR barring E-UTRAN detaches the roamer", func(t *testing.T) {
		imsi := r4gRoamer(8)
		run := s.tester(1, imsi, "--ping=false", "--await-network-detach", "--expect-detach-cause", "15")
		run.attached(t, "ARD roamer")

		s.mongo(fmt.Sprintf(`db.subscribers.updateOne({imsi: %q}, {$set: {access_restriction_data: NumberInt(48)}})`, imsi))

		run.wait(t, "ARD roamer")
		s.waitHSSRecord(imsi, "PUR after the network detach", func(r r4gHSSRecord) bool { return r.PurgeFlag })
	})

	t.Run("re-attach while registered keeps the HSS registration", func(t *testing.T) {
		imsi := r4gRoamer(4)

		s.tester(1, imsi, "--ping=false", "--detach=false").wait(t, "first attach")
		s.tester(1, imsi, "--ping=false").wait(t, "second attach")

		s.waitHSSRecord(imsi, "PUR after the final detach", func(r r4gHSSRecord) bool { return r.PurgeFlag })
	})
}

func TestRoaming4GOpen5GSHA(t *testing.T) {
	s := r4gBringUp(t, 3)

	mover := r4gRoamer(5)

	t.Run("a roamer moving to another node is cancelled on the old one", func(t *testing.T) {
		s.tester(1, mover, "--detach=false").wait(t, "attach on node 1")
		s.waitRegisteredEPS(1, 1, "roamer registered on node 1")

		run := s.tester(2, mover, "--await-network-detach", "--expect-detach-cause", "11")
		run.attached(t, "attach on node 2")

		rec := s.waitHSSRecord(mover, "the HSS registers node 2", func(r r4gHSSRecord) bool { return r.MMEHost == s.hosts[1] && !r.PurgeFlag })
		t.Logf("HSS record after the move: %+v", rec)

		s.waitRegisteredEPS(1, 0, "CLR MME_UPDATE drops the context on node 1")

		time.Sleep(15 * time.Second)

		if got := s.registeredEPS(2); got != 1 {
			t.Fatalf("node 2 has %v registered EPS subscribers after the CLR on node 1, want 1", got)
		}

		if rec := s.hssRecord(mover); rec.PurgeFlag || rec.MMEHost != s.hosts[1] {
			t.Fatalf("node 1 purged the moved roamer at the HSS: %+v", rec)
		}

		s.moverRun = run
	})

	crashed := r4gRoamer(6)

	t.Run("a restarted node drops its crash leftovers", func(t *testing.T) {
		s.tester(3, crashed, "--ping=false", "--detach=false").wait(t, "attach on node 3")

		if err := composeKill(s.ctx, r4gDir, r4gFile, "ella-core-3"); err != nil {
			t.Fatal(err)
		}

		if err := r4gCompose(s.ctx, "up", "-d", "ella-core-3"); err != nil {
			t.Fatal(err)
		}

		if err := waitForAllNodesReady(s.ctx, s.nodes); err != nil {
			t.Fatalf("nodes not ready after the restart: %v", err)
		}

		s.waitHSSPeer(3)
		s.waitRegisteredEPS(3, 0, "node 3 holds no context after the restart")

		if rec := s.hssRecord(crashed); rec.PurgeFlag {
			t.Fatalf("the restarted node sent a PUR for its crash leftover: %+v", rec)
		}
	})

	t.Run("deleting the partner detaches its roamers on every node", func(t *testing.T) {
		if s.moverRun == nil {
			t.Skip("no roamer on node 2")
		}

		other := r4gRoamer(7)
		run := s.tester(3, other, "--ping=false", "--enb-index", "1", "--await-network-detach", "--expect-detach-cause", "11")
		run.attached(t, "attach on node 3")

		start := time.Now()

		if err := s.api.DeleteRoamingPartner(s.ctx, s.partner); err != nil {
			t.Fatalf("delete the partner with roamers on nodes 2 and 3: %v", err)
		}

		t.Logf("partner deleted after %s", time.Since(start))

		s.moverRun.wait(t, "roamer on node 2")
		run.wait(t, "roamer on node 3")

		for _, imsi := range []string{mover, other} {
			s.waitHSSRecord(imsi, "PUR of "+imsi, func(r r4gHSSRecord) bool { return r.PurgeFlag })
		}

		for n := 1; n <= 3; n++ {
			s.waitRegisteredEPS(n, 0, fmt.Sprintf("node %d holds no roamer", n))
		}
	})
}
