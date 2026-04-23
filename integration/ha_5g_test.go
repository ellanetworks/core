package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	// Side-effect import to register the ha/failover_connectivity scenario.
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// TestIntegration5GHAFailover brings up a 3-node Raft cluster plus a
// core-tester sidecar, exercises registration + connectivity against the
// primary core, kills the primary, and exercises registration +
// connectivity against the surviving cluster.
//
// Lives in its own workflow (integration-tests-ha5g.yaml) because it
// needs both the ella-core and ella-core-tester images; the
// integration-tests-ha.yaml workflow loads only ella-core. The test
// function is named so it does NOT match the `-run TestIntegrationHA`
// filter used by integration-tests-ha.yaml.
//
// Passes only if the ha/failover_connectivity scenario exits 0.
func TestIntegration5GHAFailover(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	const (
		composeDir  = "compose/ha-5g/"
		composeFile = "compose.yaml"

		primaryService   = "ella-core-1"
		secondaryService = "ella-core-2"
		tertiaryService  = "ella-core-3"

		primaryAPIURL   = "http://10.100.0.11:5002"
		secondaryAPIURL = "http://10.100.0.12:5002"
		tertiaryAPIURL  = "http://10.100.0.13:5002"

		primaryN2   = "10.100.0.11:38412"
		secondaryN2 = "10.100.0.12:38412"
		tertiaryN2  = "10.100.0.13:38412"

		gnbN2 = "10.100.0.20"
		gnbN3 = "10.3.0.20"
	)

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	dc.ComposeDownWithFile(ctx, composeDir, composeFile)

	if err := dc.ComposeUpWithFile(ctx, composeDir, composeFile); err != nil {
		t.Fatalf("compose up: %v", err)
	}

	t.Cleanup(func() {
		for _, svc := range []string{primaryService, secondaryService, tertiaryService} {
			if logs, err := dc.ComposeLogs(ctx, composeDir, svc); err == nil {
				t.Logf("=== %s logs ===\n%s", svc, logs)
			}
		}

		dc.ComposeDownWithFile(context.Background(), composeDir, composeFile)
	})

	haClient, err := client.New(&client.Config{
		BaseURLs: []string{primaryAPIURL, secondaryAPIURL, tertiaryAPIURL},
	})
	if err != nil {
		t.Fatalf("ella HA client: %v", err)
	}

	if err := waitForHAClusterReady(ctx, haClient); err != nil {
		t.Fatalf("cluster did not form: %v", err)
	}

	if err := bootstrapTesterCore(ctx, haClient); err != nil {
		t.Fatalf("bootstrap core: %v", err)
	}

	fx := fixture.New(t, ctx, haClient)
	fx.OperatorDefault()
	fx.Profile(fixture.DefaultProfileSpec())
	fx.Slice(fixture.DefaultSliceSpec())
	fx.DataNetwork(fixture.DefaultDataNetworkSpec())
	fx.Policy(fixture.DefaultPolicySpec())

	// The failover scenario declares its own Fixture() (the default
	// subscriber). Apply it via the spec rather than hardcoding here so
	// this test stays aligned with the scenario's declared needs.
	sc, ok := scenarios.Get("ha/failover_connectivity")
	if !ok {
		t.Fatalf("scenario ha/failover_connectivity not registered")
	}

	spec := sc.Fixture()
	fx.Apply(spec)

	testerContainer, err := dc.ResolveComposeContainer(ctx, "ha-5g", "ella-core-tester")
	if err != nil {
		t.Fatalf("resolve tester container: %v", err)
	}

	// Kick off the scenario. Stdout is mirrored to the test log AND scanned
	// for the PHASE1_DONE marker so we can synchronise the kill.
	markerCh := make(chan struct{})

	writer := newMarkerWriter(t, "PHASE1_DONE", markerCh)

	argv := []string{
		"core-tester", "run", "ha/failover_connectivity",
		"--ella-core-n2-address", primaryN2,
		"--ella-core-n2-address", secondaryN2,
		"--ella-core-n2-address", tertiaryN2,
		"--gnb", fmt.Sprintf("gnb1,n2=%s,n3=%s", gnbN2, gnbN3),
		"--verbose",
	}

	scenarioErr := make(chan error, 1)

	go func() {
		_, err := dc.Exec(ctx, testerContainer, argv, false, 5*time.Minute, writer)
		scenarioErr <- err
	}()

	select {
	case <-markerCh:
		t.Logf("phase 1 complete; killing %s", primaryService)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for phase-1 marker: %v", ctx.Err())
	case err := <-scenarioErr:
		t.Fatalf("scenario exited before phase-1 marker: %v", err)
	}

	// Stop the primary core. Docker sends SIGTERM with a grace period;
	// the SCTP association closes cleanly, the gNB's receiver sees EOF,
	// and the gNB promotes peer[1] (secondaryN2). NAT is enabled on Ella
	// Core, so N6 return traffic to the new UPF is unicast-routed via
	// the existing NAT flow — no router-route update needed.
	if err := dc.ComposeStopWithFile(ctx, composeDir, composeFile, primaryService); err != nil {
		t.Fatalf("stop %s: %v", primaryService, err)
	}

	select {
	case err := <-scenarioErr:
		if err != nil {
			t.Fatalf("scenario failed: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("scenario did not exit: %v", ctx.Err())
	}

	t.Log("failover scenario passed both phases")
}

// waitForHAClusterReady polls the HA client until it can list cluster
// members and at least one node reports leadership. Deadline is bounded
// by the caller's ctx.
func waitForHAClusterReady(ctx context.Context, c *client.Client) error {
	deadline := time.Now().Add(2 * time.Minute)

	for {
		members, err := c.ListClusterMembers(ctx)
		if err == nil {
			for _, m := range members {
				if m.IsLeader {
					return nil
				}
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for cluster to elect a leader (last err: %v)", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}

// markerWriter mirrors writes to a testing.T log AND watches for a
// substring. When the substring first appears, it closes a channel so
// the orchestrator can synchronise with the scenario.
//
// Subtle: docker exec does not guarantee chunk boundaries align with
// lines. The buffered scan below handles partial lines, so the marker
// can appear split across writes without being missed.
type markerWriter struct {
	t      *testing.T
	marker []byte
	buf    bytes.Buffer
	ch     chan<- struct{}
	once   sync.Once
	mu     sync.Mutex
}

func newMarkerWriter(t *testing.T, marker string, found chan<- struct{}) io.Writer {
	t.Helper()

	return &markerWriter{
		t:      t,
		marker: []byte(marker),
		ch:     found,
	}
}

func (w *markerWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(p)

	for {
		b := w.buf.Bytes()

		idx := bytes.IndexByte(b, '\n')
		if idx < 0 {
			break
		}

		line := string(b[:idx])
		w.buf.Next(idx + 1)

		w.t.Log(strings.TrimRight(line, "\r"))

		if bytes.Contains([]byte(line), w.marker) {
			w.once.Do(func() { close(w.ch) })
		}
	}

	return len(p), nil
}
