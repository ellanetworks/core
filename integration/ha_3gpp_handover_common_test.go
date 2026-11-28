// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

var haHandoverMarkerPattern = regexp.MustCompile(scenarios.HAHandoverMarker + ` imsi=(\d+) ue_ip=(\S+) ran_ue_id=(\d+) core=(\S+)`)

type haHandoverRAT struct {
	system          string
	targetRadioName string
	rat             string
	acceptType      string
	ranUEID         func(*client.UEConnection) *int64
}

var (
	haHandover5G = haHandoverRAT{
		system:          "5G",
		targetRadioName: "Target-gNB",
		rat:             "5g",
		acceptType:      "RegistrationAccept",
		ranUEID:         func(c *client.UEConnection) *int64 { return c.RanUeNgapID },
	}
	haHandover4G = haHandoverRAT{
		system:          "4G",
		targetRadioName: "Target-S1eNB",
		rat:             "4g",
		acceptType:      "AttachAccept",
		ranUEID:         func(c *client.UEConnection) *int64 { return c.ENBUeS1apID },
	}
)

type haHandoverOutcome struct {
	imsi    string
	ueIP    string
	ranUEID int64
	core    string
}

func runHA3GPPHandover(t *testing.T, rat haHandoverRAT, scenario string) {
	t.Helper()

	if DetectIPFamily() != IPv4Only {
		t.Skipf("skipping: %s runs on the IPv4 ha-3gpp topology only", t.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	const composeDir = "compose/ha-3gpp/"

	var (
		nodeServices = [3]string{"ella-core-1", "ella-core-2", "ella-core-3"}
		nodeN2Addrs  = [3]string{"10.100.0.11:38412", "10.100.0.12:38412", "10.100.0.13:38412"}
		gnbs         = []scenarios.GNB{
			{Name: "gnb1", N2Address: "10.100.0.20", N3Address: "10.3.0.20"},
			{Name: "gnb2", N2Address: "10.100.0.21", N3Address: "10.3.0.21"},
		}
	)

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	adminToken, nodeClients, err := bringUpHA3GPPCluster(t, ctx, dc, composeDir, bringUpHA3GPPClusterOpts{
		ExtraServices: []string{"ella-core-tester", "router"},
	})
	if err != nil {
		t.Fatalf("bring up cluster: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		captureServiceLogs(t, dc, composeDir, append(nodeServices[:], "ella-core-tester"))

		dc.ComposeDownWithFile(cleanupCtx, composeDir, ha3GPPComposeFile)
	})

	haClient, err := client.New(&client.Config{
		BaseURLs: []string{"http://10.100.0.11:5002", "http://10.100.0.12:5002", "http://10.100.0.13:5002"},
	})
	if err != nil {
		t.Fatalf("HA client: %v", err)
	}

	haClient.SetToken(adminToken)

	if err := configureNATAndRoute(ctx, nodeClients); err != nil {
		t.Fatalf("configure NAT + route: %v", err)
	}

	fx := fixture.New(t, ctx, haClient)
	fx.OperatorDefault()
	fx.Profile(fixture.DefaultProfileSpec())
	fx.Slice(fixture.DefaultSliceSpec())
	fx.DataNetwork(fixture.DefaultDataNetworkSpec())
	fx.Policy(fixture.DefaultPolicySpec())

	sc, ok := scenarios.Get(scenario)
	Assert(t, ok, fmt.Sprintf("scenario %q not registered", scenario))

	spec := sc.Fixture(scenarios.Env{})
	fx.Apply(spec)

	leaderIdx, _, err := findLeader(ctx, nodeClients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	home := (leaderIdx + 1) % len(nodeServices)
	orderedN2 := orderLeaderFirst(nodeN2Addrs[:], home)

	HALogf(t, "leader is %s; the UE registers on follower %s (%s)", nodeServices[leaderIdx], nodeServices[home], orderedN2[0])

	testerContainer, err := dc.ResolveComposeContainer(ctx, "ha-3gpp", "ella-core-tester")
	if err != nil {
		t.Fatalf("resolve tester container: %v", err)
	}

	for _, cmd := range [][]string{
		{"ip", "addr", "add", gnbs[1].N2Address + "/24", "dev", "eth0"},
		{"ip", "addr", "add", gnbs[1].N3Address + "/24", "dev", "n3"},
	} {
		if out, err := dc.Exec(ctx, testerContainer, cmd, false, 10*time.Second, nil); err != nil {
			t.Fatalf("add the target gNB address (%v): %v\n%s", cmd, err, out)
		}
	}

	awaitDatapath(ctx, t, nodeClients...)

	argv := []string{"core-tester", "run", scenario}
	for _, addr := range orderedN2 {
		argv = append(argv, "--ella-core-n2-address", addr)
	}

	for _, g := range gnbs {
		argv = append(argv, "--gnb", fmt.Sprintf("%s,n2=%s,n3=%s", g.Name, g.N2Address, g.N3Address))
	}

	argv = append(argv, "--ip-version", string(IPv4Only), "--verbose")

	markerCh := make(chan struct{})
	output := &lockedBuffer{}
	writer := io.MultiWriter(output, newMarkerWriter(t, scenarios.HAHandoverMarker, markerCh))

	scenarioErr := make(chan error, 1)

	go func() {
		_, execErr := dc.Exec(ctx, testerContainer, argv, false, 6*time.Minute, writer)
		scenarioErr <- execErr
	}()

	select {
	case <-markerCh:
	case runErr := <-scenarioErr:
		t.Fatalf("scenario %s exited before the handover completed: %v", scenario, runErr)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for the handover: %v", ctx.Err())
	}

	outcome, err := parseHAHandoverOutcome(output.String())
	if err != nil {
		t.Fatal(err)
	}

	if got, want := hostOf(outcome.core), hostOf(orderedN2[0]); got != want {
		t.Fatalf("the UE was served by the core at %s, want %s", got, want)
	}

	HALogf(t, "handover done: imsi=%s ue_ip=%s target RAN UE ID %d on %s", outcome.imsi, outcome.ueIP, outcome.ranUEID, nodeServices[home])

	assertHAHandoverHomeNode(t, ctx, rat, nodeClients[home], nodeServices[home], outcome)

	for i, c := range nodeClients {
		if i == home {
			continue
		}

		assertNoUEContext(t, ctx, c, nodeServices[i], outcome.imsi)
	}

	for i, c := range nodeClients {
		want := 0.0
		if i == home {
			want = 1
		}

		got := sessionMetricSeries(t, ctx, c, "app_nas_messages_total", `direction="outbound"`, fmt.Sprintf("rat=%q", rat.rat), fmt.Sprintf("type=%q", rat.acceptType))
		if got != want {
			t.Errorf("%s sent %v %s %s messages, want %v", nodeServices[i], got, rat.system, rat.acceptType, want)
		}
	}

	if _, err := dc.Exec(ctx, testerContainer, []string{"pkill", "-INT", "-f", "core-tester run " + scenario}, false, 10*time.Second, nil); err != nil {
		t.Fatalf("stop scenario %s: %v", scenario, err)
	}

	select {
	case runErr := <-scenarioErr:
		if runErr != nil {
			t.Fatalf("scenario %s failed: %v", scenario, runErr)
		}
	case <-ctx.Done():
		t.Fatalf("scenario %s did not exit: %v", scenario, ctx.Err())
	}

	if len(spec.AssertUsageForIMSIs) > 0 {
		fixture.AssertUsagePositive(ctx, t, nodeClients[home], spec.AssertUsageForIMSIs, 30*time.Second)
	}
}

func parseHAHandoverOutcome(out string) (haHandoverOutcome, error) {
	m := haHandoverMarkerPattern.FindStringSubmatch(out)
	if m == nil {
		return haHandoverOutcome{}, fmt.Errorf("no %s line in the scenario output", scenarios.HAHandoverMarker)
	}

	ranID, err := strconv.ParseInt(m[3], 10, 64)
	if err != nil {
		return haHandoverOutcome{}, fmt.Errorf("parse RAN UE ID %q: %w", m[3], err)
	}

	return haHandoverOutcome{imsi: m[1], ueIP: m[2], ranUEID: ranID, core: m[4]}, nil
}

func hostOf(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}

	return host
}

func assertHAHandoverHomeNode(t *testing.T, ctx context.Context, rat haHandoverRAT, c *client.Client, service string, outcome haHandoverOutcome) {
	t.Helper()

	sub, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: outcome.imsi})
	if err != nil {
		t.Fatalf("GetSubscriber(%s) on %s: %v", outcome.imsi, service, err)
	}

	var reg *client.Registration

	for i := range sub.Registrations {
		if sub.Registrations[i].System == rat.system {
			reg = &sub.Registrations[i]
		}
	}

	if reg == nil || !reg.Registered {
		t.Fatalf("%s: subscriber %s has no %s registration after the handover: %+v", service, outcome.imsi, rat.system, sub.Registrations)
	}

	if reg.ConnectionState == nil || *reg.ConnectionState != "connected" {
		t.Errorf("%s: subscriber %s connection state %v, want connected", service, outcome.imsi, reg.ConnectionState)
	}

	if reg.Radio != rat.targetRadioName {
		t.Errorf("%s: subscriber %s is on radio %q, want the target %q", service, outcome.imsi, reg.Radio, rat.targetRadioName)
	}

	if reg.Connection == nil || rat.ranUEID(reg.Connection) == nil || *rat.ranUEID(reg.Connection) != outcome.ranUEID {
		t.Errorf("%s: subscriber %s RAN connection %+v, want the target's RAN UE ID %d", service, outcome.imsi, reg.Connection, outcome.ranUEID)
	}

	if len(sub.Sessions) != 1 {
		t.Fatalf("%s: subscriber %s has %d sessions after the handover, want 1: %+v", service, outcome.imsi, len(sub.Sessions), sub.Sessions)
	}

	if got := sub.Sessions[0].IPv4Address; got != outcome.ueIP {
		t.Errorf("%s: subscriber %s session IP %q after the handover, want the registration IP %q", service, outcome.imsi, got, outcome.ueIP)
	}
}

func assertNoUEContext(t *testing.T, ctx context.Context, c *client.Client, service, imsi string) {
	t.Helper()

	sub, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsi})
	if err != nil {
		t.Fatalf("GetSubscriber(%s) on %s: %v", imsi, service, err)
	}

	for _, reg := range sub.Registrations {
		if reg.Registered {
			t.Errorf("%s holds a %s registration for %s, want the UE context on its original node only", service, reg.System, imsi)
		}
	}

	if len(sub.Sessions) != 0 {
		t.Errorf("%s holds %d sessions for %s, want none", service, len(sub.Sessions), imsi)
	}
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}
