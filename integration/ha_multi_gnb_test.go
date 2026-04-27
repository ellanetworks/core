package integration_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	// Side-effect import to register the multi/cluster_traffic scenario.
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// TestIntegration3GPPMultiGNB stands up a 3-node Ella Core cluster and
// three independent core-tester containers, each driving its own gNB
// homed on exactly one core node. Every gNB registers a pool of UEs in
// parallel and runs a GTP-U ping for each — 15 UEs total in flight
// across 3 cores.
//
// What this exercises that no other test does:
//
//   - Concurrent leader-bound writes from two followers. Each UE
//     registration triggers an AUSF sequenceNumber bump (a Raft-replicated
//     write); two of the three cores are followers and proxy their
//     proposes via proxy_middleware.go. Today only TestIntegrationHAFollowerProxy
//     touches that path, with one in-flight write from one follower.
//   - Per-node IP lease pool contention. Migration v9 added ip_leases.nodeID;
//     this is the first integration test that hits the lease allocator
//     from three nodes simultaneously.
//   - UPF locality. Each gNB's GTP-U traffic must terminate at its own
//     core's in-process UPF. A regression where the SMF returned the
//     wrong N3 endpoint in PDU Session Resource Setup would route across
//     the wrong UPF.
//   - Per-node BGP scoping (latent — not asserted here, but exercised:
//     each core advertises only routes for leases it owns).
func TestIntegration3GPPMultiGNB(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	const (
		composeDir  = "compose/ha-5g-multi-gnb/"
		composeFile = "compose.yaml"
		uesPerGNB   = 5
	)

	// Per-tester gNB topology. Each tester runs in its own container with
	// its own N2 + N3 IPs and is pinned to a single core. The IMSI pool
	// is partitioned by gNB so subscribers don't collide across testers.
	type gnbSpec struct {
		service  string // compose service name
		gnbID    string
		n2       string // gNB N2 address (cluster bridge)
		n3       string // gNB N3 address (n3 bridge)
		coreN2   string // target core's NGAP listener
		imsiBase string // first IMSI for this tester's UE pool
	}

	gnbs := []gnbSpec{
		{
			service:  "ella-core-tester-1",
			gnbID:    "000001",
			n2:       "10.100.0.21",
			n3:       "10.3.0.21",
			coreN2:   "10.100.0.11:38412",
			imsiBase: "001019756140100",
		},
		{
			service:  "ella-core-tester-2",
			gnbID:    "000002",
			n2:       "10.100.0.22",
			n3:       "10.3.0.22",
			coreN2:   "10.100.0.12:38412",
			imsiBase: "001019756140110",
		},
		{
			service:  "ella-core-tester-3",
			gnbID:    "000003",
			n2:       "10.100.0.23",
			n3:       "10.3.0.23",
			coreN2:   "10.100.0.13:38412",
			imsiBase: "001019756140120",
		},
	}

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	testerServices := make([]string, 0, len(gnbs)+1)
	for _, g := range gnbs {
		testerServices = append(testerServices, g.service)
	}

	testerServices = append(testerServices, "router")

	adminToken, nodeClients, err := bringUpHA3GPPCluster(ctx, dc, composeDir, composeFile, testerServices...)
	if err != nil {
		t.Fatalf("bring up cluster: %v", err)
	}

	t.Cleanup(func() {
		// Fresh context: the test ctx is cancelled by defer cancel()
		// when the test unwinds (including on t.Fatalf), which would
		// otherwise make every ComposeLogs call fail with context.Canceled.
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		for _, svc := range []string{"ella-core-1", "ella-core-2", "ella-core-3"} {
			logs, logErr := dc.ComposeLogs(cleanupCtx, composeDir, svc)
			if logErr != nil {
				t.Logf("=== %s logs: collection failed: %v ===", svc, logErr)
			} else {
				t.Logf("=== %s logs ===\n%s", svc, logs)
			}
		}

		dc.ComposeDownWithFile(cleanupCtx, composeDir, composeFile)
	})

	haClient, err := client.New(&client.Config{
		BaseURLs: []string{
			"http://10.100.0.11:5002",
			"http://10.100.0.12:5002",
			"http://10.100.0.13:5002",
		},
	})
	if err != nil {
		t.Fatalf("HA client: %v", err)
	}

	haClient.SetToken(adminToken)

	if err := configureNATAndRoute(ctx, haClient); err != nil {
		t.Fatalf("configure NAT + route: %v", err)
	}

	// Baseline fixture: operator, default profile/slice/data network/policy.
	fx := fixture.New(t, ctx, haClient)
	fx.OperatorDefault()
	fx.Profile(fixture.DefaultProfileSpec())
	fx.Slice(fixture.DefaultSliceSpec())
	fx.DataNetwork(fixture.DefaultDataNetworkSpec())
	fx.Policy(fixture.DefaultPolicySpec())

	// 15 subscribers (5 per gNB), all sharing the default Key/OpC/SQN
	// since none of them register more than once before the test ends.
	allIMSIs := make([]string, 0, len(gnbs)*uesPerGNB)
	subSpec := scenarios.FixtureSpec{}

	for _, g := range gnbs {
		for i := 0; i < uesPerGNB; i++ {
			imsi, err := offsetIMSI15(g.imsiBase, i)
			if err != nil {
				t.Fatalf("compute IMSI: %v", err)
			}

			subSpec.Subscribers = append(subSpec.Subscribers, scenarios.SubscriberSpec{
				IMSI:           imsi,
				Key:            scenarios.DefaultKey,
				OPc:            scenarios.DefaultOPC,
				SequenceNumber: scenarios.DefaultSequenceNumber,
				ProfileName:    scenarios.DefaultProfileName,
			})

			allIMSIs = append(allIMSIs, imsi)
		}
	}

	fx.Apply(subSpec)

	// Resolve each tester's container name so we can dc.Exec into it.
	testerContainers := make([]string, len(gnbs))

	for i, g := range gnbs {
		container, err := dc.ResolveComposeContainer(ctx, "ha-5g-multi-gnb", g.service)
		if err != nil {
			t.Fatalf("resolve tester container %s: %v", g.service, err)
		}

		testerContainers[i] = container
	}

	// Drive all three testers in parallel. Use a WaitGroup + collected
	// error slice (rather than errgroup) so a failure in one tester does
	// not cancel the others — we want to see ALL failures, not just the
	// first, since this test is specifically about behaviour under
	// concurrent load.
	var (
		wg    sync.WaitGroup
		errMu sync.Mutex
		errs  []string
	)

	for i, gn := range gnbs {
		i, gn := i, gn

		argv := []string{
			"core-tester", "run", "multi/cluster_traffic",
			"--ella-core-n2-address", gn.coreN2,
			"--gnb", fmt.Sprintf("gnb%d,n2=%s,n3=%s", i+1, gn.n2, gn.n3),
			"--ue-count", strconv.Itoa(uesPerGNB),
			"--imsi-base", gn.imsiBase,
			"--gnb-id", gn.gnbID,
			"--verbose",
		}

		wg.Add(1)

		go func() {
			defer wg.Done()

			t.Logf("starting scenario on %s (target core %s)", gn.service, gn.coreN2)

			out, execErr := dc.Exec(ctx, testerContainers[i], argv, false, 5*time.Minute, nil)
			if execErr != nil {
				errMu.Lock()

				errs = append(errs, fmt.Sprintf("%s: %v\n--- output ---\n%s", gn.service, execErr, out))
				errMu.Unlock()

				return
			}

			t.Logf("%s scenario completed", gn.service)
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		t.Fatalf("%d/%d scenarios failed:\n\n%s",
			len(errs), len(gnbs), strings.Join(errs, "\n\n"))
	}

	t.Log("all 3 scenarios passed; verifying cluster state")

	// All 15 subscribers must be Registered with at least one PDU
	// session. This is the primary functional assertion: ping success
	// inside each tester proves the per-UE flow worked, and this
	// confirms the leader's view of subscriber state matches.
	leader := findFirstLeader(ctx, t, nodeClients)

	for _, imsi := range allIMSIs {
		sub, err := leader.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsi})
		if err != nil {
			t.Fatalf("GetSubscriber(%s): %v", imsi, err)
		}

		if !sub.Status.Registered {
			t.Errorf("subscriber %s: expected Registered=true, got false", imsi)
		}

		if len(sub.PDUSessions) == 0 {
			t.Errorf("subscriber %s: expected >=1 PDU session, got 0", imsi)

			continue
		}

		ip := sub.PDUSessions[0].IPAddress
		if !strings.HasPrefix(ip, "10.45.") {
			t.Errorf("subscriber %s: PDU session IP %q not in expected pool 10.45.0.0/16", imsi, ip)
		}
	}

	// Membership and autopilot should still be healthy after the load —
	// nothing in the steady-state scenario should have destabilised the
	// cluster.
	assertMembershipConsistent(t, ctx, nodeClients)

	apState, err := leader.GetAutopilotState(ctx)
	if err != nil {
		t.Fatalf("GetAutopilotState: %v", err)
	}

	if !apState.Healthy {
		t.Errorf("autopilot reports unhealthy after load: %+v", apState)
	}

	if apState.FailureTolerance != 1 {
		t.Errorf("expected failureTolerance=1 after load, got %d", apState.FailureTolerance)
	}
}

// findFirstLeader returns whichever of clients reports Role=Leader.
// Fails the test if none do.
func findFirstLeader(ctx context.Context, t *testing.T, clients []*client.Client) *client.Client {
	t.Helper()

	for _, c := range clients {
		status, err := c.GetStatus(ctx)
		if err != nil || status.Cluster == nil {
			continue
		}

		if status.Cluster.Role == "Leader" {
			return c
		}
	}

	t.Fatal("no leader found")

	return nil
}

// offsetIMSI15 increments the last digits of base by offset and returns
// the result zero-padded back to 15 characters. Mirrors the helper in
// scenarios/multi but kept private to the integration test so the test
// has its own deterministic IMSI scheme without importing the scenario
// package's internals.
func offsetIMSI15(base string, offset int) (string, error) {
	if len(base) != 15 {
		return "", fmt.Errorf("base %q must be 15 digits", base)
	}

	n, err := strconv.ParseUint(base, 10, 64)
	if err != nil {
		return "", fmt.Errorf("parse base %q: %w", base, err)
	}

	out := strconv.FormatUint(n+uint64(offset), 10)
	if len(out) > 15 {
		return "", fmt.Errorf("base %q + offset %d overflows 15 digits", base, offset)
	}

	return strings.Repeat("0", 15-len(out)) + out, nil
}
