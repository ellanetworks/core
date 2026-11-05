// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const (
	smscClusterOverlay    = "../sms/smsc-cluster.yaml"
	smscClusterComposeDir = "compose/core-tester/"
	smscClusterName       = "ella-smsc-cluster"
	smscClusterMOMetric   = `ellasmsc_messages_received_total{origin="mobile",result="accepted"}`
)

var smscClusterScenarios = []string{"sms/4g_mo_mt", "sms/5g_mo_mt", "sms/4g_absent_then_alert", "sms/5g_absent_then_alert"}

type smscClusterNode struct {
	service string
	address string
	host    string
	id      string
}

func (n *smscClusterNode) api() string {
	return "http://" + net.JoinHostPort(n.address, strconv.Itoa(smscAPIPort))
}

type smscCluster struct {
	nodes []*smscClusterNode
	token string
}

type smscClusterStatus struct {
	NodeID   string `json:"node_id"`
	LeaderID string `json:"leader_id"`
	Members  []struct {
		NodeID   string `json:"node_id"`
		Suffrage string `json:"suffrage"`
		Healthy  *bool  `json:"healthy"`
	} `json:"members"`
}

func TestIntegrationSMSCCluster(t *testing.T) {
	suites.Require(t, suites.HASMS)

	if DetectIPFamily() != IPv4Only {
		t.Skip("the SMSC cluster topology is IPv4 only")
	}

	ctx := context.Background()
	env := setupTesterEnv(ctx, t, smscClusterOverlay)

	c := &smscCluster{nodes: []*smscClusterNode{
		{service: "smsc-1", address: "10.3.0.15"},
		{service: "smsc-2", address: "10.3.0.16"},
		{service: "smsc-3", address: "10.3.0.17"},
	}}

	t.Cleanup(func() {
		if !t.Failed() {
			return
		}

		for _, n := range c.nodes {
			if logs, err := smscClusterCompose(context.Background(), "logs", "--no-color", n.service); err == nil {
				t.Logf("=== %s logs ===\n%s", n.service, logs)
			}
		}
	})

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	c.form(ctx, t)

	nodes := make([]client.SMSCNodeOptions, 0, len(c.nodes))
	for _, n := range c.nodes {
		nodes = append(nodes, client.SMSCNodeOptions{DiameterIdentity: n.host, Address: n.address, Port: smscDiameterPort})
	}

	if _, err := env.Client.CreateSMSC(ctx, &client.CreateSMSCOptions{
		Name:           smscClusterName,
		ServiceCentres: []string{smscServiceCentre},
		Nodes:          nodes,
	}); err != nil {
		t.Fatalf("add the SMSC: %v", err)
	}

	if err := env.Client.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{SMSNumber: smsNumber}); err != nil {
		t.Fatalf("enable SMS: %v", err)
	}

	if err := c.awaitLinks(ctx, env.Client, c.nodes); err != nil {
		t.Fatal(err)
	}

	t.Run("all_nodes_up", func(t *testing.T) {
		before := c.moCounts(ctx, t, c.nodes)
		c.runScenarios(ctx, t, env, c.nodes[0])
		after := c.moCounts(ctx, t, c.nodes)

		receivers := 0

		for i := range c.nodes {
			if after[i] > before[i] {
				receivers++
			}
		}

		if receivers < 2 {
			t.Fatalf("MO-Forward reached %d SMSC node(s), want at least 2 (before %v, after %v)", receivers, before, after)
		}
	})

	t.Run("follower_down", func(t *testing.T) {
		leader := c.leader(ctx, t, c.nodes)
		victim := c.others(leader)[0]

		t.Logf("killing the SMSC follower %s (%s)", victim.service, victim.host)

		if _, err := smscClusterCompose(ctx, "kill", victim.service); err != nil {
			t.Fatal(err)
		}

		c.runScenarios(ctx, t, env, leader)

		if err := c.awaitNodeState(ctx, env.Client, victim, false); err != nil {
			t.Fatal(err)
		}

		if _, err := smscClusterCompose(ctx, "start", victim.service); err != nil {
			t.Fatal(err)
		}

		if err := c.awaitHealthy(ctx, leader); err != nil {
			t.Fatal(err)
		}

		if err := c.awaitLinks(ctx, env.Client, c.nodes); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("leader_down", func(t *testing.T) {
		leader := c.leader(ctx, t, c.nodes)
		survivors := c.others(leader)

		t.Logf("killing the SMSC leader %s (%s)", leader.service, leader.host)

		if _, err := smscClusterCompose(ctx, "kill", leader.service); err != nil {
			t.Fatal(err)
		}

		next := c.leader(ctx, t, survivors)
		t.Logf("new SMSC leader %s (%s)", next.service, next.host)

		if err := c.awaitNodeState(ctx, env.Client, leader, false); err != nil {
			t.Fatal(err)
		}

		before := c.moCounts(ctx, t, survivors)
		c.runScenarios(ctx, t, env, next)
		after := c.moCounts(ctx, t, survivors)

		received := 0
		for i := range survivors {
			received += after[i] - before[i]
		}

		if received == 0 {
			t.Fatalf("no surviving SMSC node accepted an MO short message (before %v, after %v)", before, after)
		}
	})

	printTesterSummary(t)
}

func (c *smscCluster) runScenarios(ctx context.Context, t *testing.T, env *testerEnv, api *smscClusterNode) {
	t.Helper()

	for _, name := range smscClusterScenarios {
		sc, ok := scenarios.Get(name)
		Assert(t, ok, fmt.Sprintf("scenario %q not registered", name))

		tr := registerScenarioTest(t.Name() + "/" + name)

		t.Run(name, func(t *testing.T) {
			defer finishScenarioTest(t, tr)

			fx := fixture.New(t, ctx, env.Client)
			fx.Apply(sc.Fixture(scenarios.Env{}))

			env.RunScenario(ctx, t, name, tr, "--smsc-api-address", api.api(), "--smsc-api-token", c.token)
		})
	}
}

func (c *smscCluster) form(ctx context.Context, t *testing.T) {
	t.Helper()

	first := c.nodes[0]
	credentials := map[string]string{"email": smscAdminEmail, "password": smscAdminPassword}

	var session struct {
		Token string `json:"token"`
	}

	if err := retrySMSC(time.Minute, func() error {
		return callSMSCAt(ctx, first.api(), http.MethodPost, "", "/api/v1/init", credentials, &session)
	}); err != nil {
		t.Fatalf("initialize %s: %v", first.service, err)
	}

	operator := map[string]any{
		"mcc":                    "001",
		"mnc":                    "01",
		"service_centre_address": smscServiceCentre,
		"numbering":              map[string]string{"country_code": "1", "national_prefix": "1", "international_prefix": "011"},
	}

	delivery := map[string]any{
		"default_validity_seconds": 3600,
		"retry_intervals_seconds":  []int{600},
	}

	if err := callSMSCAt(ctx, first.api(), http.MethodPut, session.Token, "/api/v1/operator", operator, nil); err != nil {
		t.Fatalf("configure the SMSC operator: %v", err)
	}

	if err := callSMSCAt(ctx, first.api(), http.MethodPut, session.Token, "/api/v1/delivery", delivery, nil); err != nil {
		t.Fatalf("configure the SMSC delivery: %v", err)
	}

	for _, n := range c.nodes[1:] {
		var join struct {
			Token string `json:"token"`
		}

		if err := callSMSCAt(ctx, first.api(), http.MethodPost, session.Token, "/api/v1/cluster/tokens", map[string]int{}, &join); err != nil {
			t.Fatalf("create a join token for %s: %v", n.service, err)
		}

		if err := retrySMSC(time.Minute, func() error {
			return callSMSCAt(ctx, n.api(), http.MethodPost, "", "/api/v1/cluster/join", map[string]string{"token": join.Token}, nil)
		}); err != nil {
			t.Fatalf("join %s to the cluster: %v", n.service, err)
		}
	}

	var created struct {
		Token string `json:"token"`
	}

	name := fmt.Sprintf("integration-%d", time.Now().UnixNano())
	if err := callSMSCAt(ctx, first.api(), http.MethodPost, session.Token, "/api/v1/users/me/api-tokens", map[string]string{"name": name}, &created); err != nil {
		t.Fatalf("create an SMSC API token: %v", err)
	}

	c.token = created.Token

	if err := c.awaitHealthy(ctx, first); err != nil {
		t.Fatal(err)
	}

	for _, n := range c.nodes {
		var diameter struct {
			Host string `json:"host"`
		}

		var status smscClusterStatus

		if err := retrySMSC(time.Minute, func() error {
			if err := callSMSCAt(ctx, n.api(), http.MethodGet, c.token, "/api/v1/cluster", nil, &status); err != nil {
				return err
			}

			if err := callSMSCAt(ctx, n.api(), http.MethodGet, c.token, "/api/v1/diameter", nil, &diameter); err != nil {
				return err
			}

			if diameter.Host == "" || !strings.HasSuffix(diameter.Host, ".epc.mnc001.mcc001.3gppnetwork.org") {
				return fmt.Errorf("diameter host %q", diameter.Host)
			}

			return nil
		}); err != nil {
			t.Fatalf("read the Diameter host of %s: %v", n.service, err)
		}

		n.host, n.id = diameter.Host, status.NodeID
	}

	t.Logf("SMSC cluster formed: %s=%s, %s=%s, %s=%s", c.nodes[0].service, c.nodes[0].host, c.nodes[1].service, c.nodes[1].host, c.nodes[2].service, c.nodes[2].host)
}

func (c *smscCluster) awaitHealthy(ctx context.Context, via *smscClusterNode) error {
	return retrySMSC(2*time.Minute, func() error {
		var status smscClusterStatus
		if err := callSMSCAt(ctx, via.api(), http.MethodGet, c.token, "/api/v1/cluster", nil, &status); err != nil {
			return err
		}

		healthy := 0

		for _, m := range status.Members {
			if m.Suffrage == "voter" && m.Healthy != nil && *m.Healthy {
				healthy++
			}
		}

		if healthy != len(c.nodes) {
			return fmt.Errorf("%d of %d SMSC voters are healthy", healthy, len(c.nodes))
		}

		return nil
	})
}

func (c *smscCluster) leader(ctx context.Context, t *testing.T, among []*smscClusterNode) *smscClusterNode {
	t.Helper()

	var leader *smscClusterNode

	if err := retrySMSC(time.Minute, func() error {
		for _, n := range among {
			var status smscClusterStatus
			if err := callSMSCAt(ctx, n.api(), http.MethodGet, c.token, "/api/v1/cluster", nil, &status); err != nil {
				continue
			}

			for _, m := range among {
				if m.id == status.LeaderID {
					leader = m
					return nil
				}
			}
		}

		return fmt.Errorf("no SMSC leader among %d nodes", len(among))
	}); err != nil {
		t.Fatal(err)
	}

	return leader
}

func (c *smscCluster) others(skip *smscClusterNode) []*smscClusterNode {
	var out []*smscClusterNode

	for _, n := range c.nodes {
		if n != skip {
			out = append(out, n)
		}
	}

	return out
}

func (c *smscCluster) awaitLinks(ctx context.Context, cl *client.Client, nodes []*smscClusterNode) error {
	return retrySMSC(2*time.Minute, func() error {
		for _, n := range nodes {
			if err := c.nodeState(ctx, cl, n, true); err != nil {
				return err
			}

			if !c.seesHSS(ctx, n) {
				return fmt.Errorf("%s does not see the HSS", n.service)
			}
		}

		return nil
	})
}

func (c *smscCluster) awaitNodeState(ctx context.Context, cl *client.Client, n *smscClusterNode, open bool) error {
	return retrySMSC(time.Minute, func() error { return c.nodeState(ctx, cl, n, open) })
}

func (c *smscCluster) nodeState(ctx context.Context, cl *client.Client, n *smscClusterNode, open bool) error {
	smsc, err := cl.GetSMSC(ctx, smscClusterName)
	if err != nil {
		return err
	}

	for _, node := range smsc.Nodes {
		if node.Address != n.address {
			continue
		}

		isOpen := node.Status != nil && node.Status.State == "open"
		if isOpen != open {
			return fmt.Errorf("SMSC node %s (%s): status %+v, want open=%t", n.service, n.host, node.Status, open)
		}

		return nil
	}

	return fmt.Errorf("SMSC node %s is not configured in Ella Core", n.address)
}

func (c *smscCluster) seesHSS(ctx context.Context, n *smscClusterNode) bool {
	var diameter struct {
		Routes []struct {
			Application string   `json:"application"`
			Peers       []string `json:"peers"`
		} `json:"routes"`
	}

	if callSMSCAt(ctx, n.api(), http.MethodGet, c.token, "/api/v1/diameter", nil, &diameter) != nil {
		return false
	}

	for _, r := range diameter.Routes {
		if r.Application == "s6c" && len(r.Peers) > 0 {
			return true
		}
	}

	return false
}

func (c *smscCluster) moCounts(ctx context.Context, t *testing.T, nodes []*smscClusterNode) []int {
	t.Helper()

	counts := make([]int, len(nodes))

	for i, n := range nodes {
		v, err := smscMOCount(ctx, n)
		if err != nil {
			t.Fatalf("read the MO counter of %s: %v", n.service, err)
		}

		counts[i] = v
	}

	return counts
}

func smscMOCount(ctx context.Context, n *smscClusterNode) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.api()+"/api/v1/metrics", nil)
	if err != nil {
		return 0, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("GET /api/v1/metrics: %s: %s", resp.Status, raw)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, smscClusterMOMetric+" ") {
			continue
		}

		v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(line, smscClusterMOMetric)), 64)
		if err != nil {
			return 0, err
		}

		return int(v), nil
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return 0, fmt.Errorf("%s not exported", smscClusterMOMetric)
}

func smscClusterCompose(ctx context.Context, args ...string) (string, error) {
	argv := append([]string{"compose", "-f", ComposeFile(), "-f", smscClusterOverlay}, args...)

	cmd := exec.CommandContext(ctx, "docker", argv...)
	cmd.Dir = smscClusterComposeDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("docker %s: %w: %s", strings.Join(argv, " "), err, out)
	}

	return string(out), nil
}

func retrySMSC(timeout time.Duration, f func() error) error {
	deadline := time.Now().Add(timeout)

	for {
		err := f()
		if err == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return err
		}

		time.Sleep(time.Second)
	}
}
