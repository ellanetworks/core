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

// TestIntegration3GPPMultiGNB drives 3 gNBs (5 UEs each) against 3
// core nodes, one gNB per core. Exercises concurrent leader-bound
// writes from two followers (AUSF SQN bumps), the per-node IP lease
// allocator under cross-node contention, and per-core UPF locality
// for GTP-U termination.
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

	// imsis filled in below from imsiBase + uesPerGNB.
	type gnbSpec struct {
		service  string
		gnbID    string
		n2       string
		n3       string
		coreN2   string
		imsiBase string
		imsis    []string
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

	// 15 subscribers, 5 per gNB. AMF state (Registered, PDUSessions)
	// is per-node, so each gNB's IMSIs are stored on the spec so the
	// post-scenario assertions hit the right core.
	subSpec := scenarios.FixtureSpec{}

	for gi := range gnbs {
		gnbs[gi].imsis = make([]string, 0, uesPerGNB)

		for i := 0; i < uesPerGNB; i++ {
			imsi, err := offsetIMSI15(gnbs[gi].imsiBase, i)
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

			gnbs[gi].imsis = append(gnbs[gi].imsis, imsi)
		}
	}

	fx.Apply(subSpec)

	testerContainers := make([]string, len(gnbs))

	for i, g := range gnbs {
		container, err := dc.ResolveComposeContainer(ctx, "ha-5g-multi-gnb", g.service)
		if err != nil {
			t.Fatalf("resolve tester container %s: %v", g.service, err)
		}

		testerContainers[i] = container
	}

	// WaitGroup not errgroup: surface every tester's failure, not
	// just the first.
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

	// AMF state is per-node (UE context is not replicated; see
	// spec_security_ha.md). Query each gNB's home core for its own
	// UEs, not the leader. gnbs[i] ↔ nodeClients[i] by node-id order.
	for i, gn := range gnbs {
		c := nodeClients[i]

		for _, imsi := range gn.imsis {
			sub, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsi})
			if err != nil {
				t.Fatalf("GetSubscriber(%s) on %s: %v", imsi, gn.service, err)
			}

			if !sub.Status.Registered {
				t.Errorf("%s: subscriber %s: expected Registered=true, got false", gn.service, imsi)
			}

			if len(sub.PDUSessions) == 0 {
				t.Errorf("%s: subscriber %s: expected >=1 PDU session, got 0", gn.service, imsi)

				continue
			}

			ip := sub.PDUSessions[0].IPAddress
			if !strings.HasPrefix(ip, "10.45.") {
				t.Errorf("%s: subscriber %s: PDU session IP %q not in expected pool 10.45.0.0/16",
					gn.service, imsi, ip)
			}
		}
	}

	// Cluster-wide health post-load. GetAutopilotState is leader-only;
	// proxy middleware forwards from any node.
	assertMembershipConsistent(t, ctx, nodeClients)

	apState, err := nodeClients[0].GetAutopilotState(ctx)
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

// offsetIMSI15 returns base + offset zero-padded to 15 digits.
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
