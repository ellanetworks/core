//go:build open5gs

// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
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
	roamingFile          = "docker-compose.yaml"
	roamingKey           = "465B5CE8B199B49FAA5F0A2EE238A6BC"
	roamingOPc           = "E8ED289DEBA952E4283B54E88E6183CA"
	roamingVisitedMCC    = "999"
	roamingVisitedMNC    = "70"
	roamingProfile       = "roamers"
	roamingPartner       = "open5gs"
	roamingTesterTimeout = 8 * time.Minute
)

type roamingRAT struct {
	name        string
	octet       int
	scenario    string
	marker      string
	milestone   string
	logServices []string
	interfaces  string
	bringUp     time.Duration
}

func (r roamingRAT) dir() string { return "compose/roaming" + r.name }

func (r roamingRAT) project() string { return "ella-roaming" + r.name }

func (r roamingRAT) env(suffix string) string {
	return os.Getenv("ROAMING" + strings.ToUpper(r.name) + "_" + suffix)
}

func (r roamingRAT) nodeIP(n int) string { return fmt.Sprintf("10.%d.0.%d", r.octet, 10+n) }

func (r roamingRAT) compose(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", append([]string{"compose", "-f", roamingFile}, args...)...)
	cmd.Dir = r.dir()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose %s: %w", strings.Join(args, " "), err)
	}

	return nil
}

func (r roamingRAT) writeFile(rel string, body string) error {
	path, err := filepath.Abs(filepath.Join(r.dir(), rel))
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

func (r roamingRAT) writeNodeConfig(n, nodes int, joinToken string) error {
	ip := r.nodeIP(n)

	var clusterYAML strings.Builder

	if nodes > 1 {
		fmt.Fprintf(&clusterYAML, "cluster:\n  bind-address: \"%s:7000\"\n  peers:\n", ip)

		for i := 1; i <= nodes; i++ {
			fmt.Fprintf(&clusterYAML, "    - \"%s:7000\"\n", r.nodeIP(i))
		}

		if joinToken != "" {
			fmt.Fprintf(&clusterYAML, "  join-token: %q\n", joinToken)
		}
	}

	return r.writeFile(fmt.Sprintf("cfg/node%d/core.yaml", n), fmt.Sprintf(`logging:
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
    address: "10.%[2]d.0.%[3]d"
  n6:
    name: "n6"
  api:
    address: %[1]q
    port: 5002
%[4]sdatapath:
  attach-mode: "xdp-generic"
%[5]s`, ip, r.octet+3, 10+n, fmt.Sprintf(r.interfaces, ip), clusterYAML.String()))
}

type roamingStack struct {
	t       *testing.T
	ctx     context.Context
	dc      *DockerClient
	rat     roamingRAT
	nodes   []*client.Client
	api     *client.Client
	partner string
}

func roamingBringUp(t *testing.T, rat roamingRAT, nodes int) *roamingStack {
	t.Helper()

	if DetectIPFamily() != IPv4Only {
		t.Skipf("the %s roaming e2e runs in IPv4 mode only", strings.ToUpper(rat.name))
	}

	ctx, cancel := context.WithTimeout(context.Background(), rat.bringUp)
	t.Cleanup(cancel)

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	_ = rat.compose(ctx, "down", "-v", "--remove-orphans")

	s := &roamingStack{t: t, ctx: ctx, dc: dc, rat: rat}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		if t.Failed() || rat.env("LOGS") != "" {
			services := append([]string{}, rat.logServices...)
			for i := 1; i <= nodes; i++ {
				services = append(services, fmt.Sprintf("ella-core-%d", i))
			}

			for _, svc := range services {
				logs, err := dc.ComposeLogs(cleanupCtx, rat.dir(), svc)
				if err != nil {
					t.Logf("=== %s logs: %v ===", svc, err)
					continue
				}

				t.Logf("=== %s logs ===\n%s", svc, logs)
			}
		}

		if rat.env("KEEP") == "" {
			_ = rat.compose(cleanupCtx, "down", "-v", "--remove-orphans")
		}
	})

	if err := rat.writeNodeConfig(1, nodes, ""); err != nil {
		t.Fatalf("write node 1 config: %v", err)
	}

	if err := rat.compose(ctx, "up", "-d", "ella-core-1"); err != nil {
		t.Fatal(err)
	}

	node1, err := client.New(&client.Config{BaseURL: "http://" + rat.nodeIP(1) + ":5002"})
	if err != nil {
		t.Fatal(err)
	}

	token, err := initializeAndGetAdminToken(ctx, node1)
	if err != nil {
		t.Fatalf("initialize node 1: %v", err)
	}

	node1.SetToken(token)
	s.nodes = []*client.Client{node1}
	s.api = node1

	for n := 2; n <= nodes; n++ {
		tok, err := node1.MintClusterJoinToken(ctx, &client.MintJoinTokenOptions{TTLSeconds: 600})
		if err != nil {
			t.Fatalf("mint join token for node %d: %v", n, err)
		}

		if err := rat.writeNodeConfig(n, nodes, tok.Token); err != nil {
			t.Fatalf("write node %d config: %v", n, err)
		}

		if err := rat.compose(ctx, "up", "-d", fmt.Sprintf("ella-core-%d", n)); err != nil {
			t.Fatal(err)
		}

		c, err := client.New(&client.Config{BaseURL: fmt.Sprintf("http://%s:5002", rat.nodeIP(n))})
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
			urls = append(urls, fmt.Sprintf("http://%s:5002", rat.nodeIP(n)))
		}

		s.api, err = client.New(&client.Config{BaseURLs: urls})
		if err != nil {
			t.Fatal(err)
		}

		s.api.SetToken(token)
	}

	s.provision()

	return s
}

func (s *roamingStack) provision() {
	t := s.t
	t.Helper()

	if err := s.api.UpdateOperatorID(s.ctx, &client.UpdateOperatorIDOptions{Mcc: roamingVisitedMCC, Mnc: roamingVisitedMNC}); err != nil {
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
	fx.Profile(fixture.ProfileSpec{Name: roamingProfile, UeAmbrUplink: "500 Mbps", UeAmbrDownlink: "500 Mbps"})
	fx.Policy(fixture.PolicySpec{
		Name: "roamers-internet", ProfileName: roamingProfile, SliceName: scenarios.DefaultSliceName,
		DataNetworkName: scenarios.DefaultDNN, SessionAmbrUplink: "200 Mbps", SessionAmbrDownlink: "200 Mbps",
		Var5qi: 9, Arp: 15,
	})
}

func (s *roamingStack) startTester() {
	s.t.Helper()

	if err := s.rat.compose(s.ctx, "up", "-d", "ella-core-tester", "router"); err != nil {
		s.t.Fatal(err)
	}
}

func (s *roamingStack) waitRoamerMSISDN(imsi, want string) {
	s.t.Helper()

	err := roamingPoll(s.ctx, 10*time.Second, func() error {
		r, err := s.api.GetRoamer(s.ctx, imsi)
		if err != nil {
			return err
		}

		if r.Msisdn != want {
			return fmt.Errorf("roamer MSISDN %q, want %q", r.Msisdn, want)
		}

		return nil
	})
	if err != nil {
		s.t.Fatalf("MSISDN of roamer %s: %v", imsi, err)
	}
}

func roamingPoll(ctx context.Context, timeout time.Duration, f func() error) error {
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

type roamingTesterRun struct {
	done      chan error
	marker    chan struct{}
	milestone string
}

func (s *roamingStack) tester(node int, imsi string, args ...string) *roamingTesterRun {
	s.t.Helper()

	container, err := s.dc.ResolveComposeContainer(s.ctx, s.rat.project(), "ella-core-tester")
	if err != nil {
		s.t.Fatalf("resolve tester container: %v", err)
	}

	o := s.rat.octet

	argv := []string{
		"core-tester", "run", s.rat.scenario,
		"--ella-core-n2-address", s.rat.nodeIP(node) + ":38412",
		"--gnb", fmt.Sprintf("gnb1,n2=10.%d.0.20,n3=10.%d.0.20", o, o+3),
		"--imsi", imsi, "--key", roamingKey, "--opc", roamingOPc,
		"--mcc", roamingVisitedMCC, "--mnc", roamingVisitedMNC,
		"--ping-destination", fmt.Sprintf("10.%d.0.3", o+6),
		"--verbose",
	}
	argv = append(argv, args...)

	run := &roamingTesterRun{done: make(chan error, 1), marker: make(chan struct{}), milestone: s.rat.milestone}
	writer := newMarkerWriter(s.t, s.rat.marker, run.marker)

	go func() {
		_, err := s.dc.Exec(s.ctx, container, argv, false, roamingTesterTimeout, writer)
		run.done <- err
	}()

	return run
}

func (r *roamingTesterRun) wait(t *testing.T, what string) {
	t.Helper()

	if err := <-r.done; err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

func (r *roamingTesterRun) reached(t *testing.T, what string) {
	t.Helper()

	select {
	case <-r.marker:
	case err := <-r.done:
		t.Fatalf("%s ended before the %s: %v", what, r.milestone, err)
	case <-time.After(2 * time.Minute):
		t.Fatalf("%s: no %s within 2 minutes", what, r.milestone)
	}
}

func (s *roamingStack) registered(node int) float64 {
	return sessionMetricSeries(s.t, s.ctx, s.nodes[node-1], "app_registered_subscribers", fmt.Sprintf("rat=%q", s.rat.name), `roaming="true"`)
}

func (s *roamingStack) waitRegistered(node int, want float64, what string) {
	s.t.Helper()

	err := roamingPoll(s.ctx, 45*time.Second, func() error {
		if got := s.registered(node); got != want {
			return fmt.Errorf("node %d has %v registered %s roamers, want %v", node, got, strings.ToUpper(s.rat.name), want)
		}

		return nil
	})
	if err != nil {
		s.t.Fatalf("%s: %v", what, err)
	}
}
