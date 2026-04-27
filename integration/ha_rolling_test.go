package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

const haRollingComposeDir = "compose/ha-rolling/"

// rollingBaselineImage is the "old" image: current source compiled
// without the synthetic-migrations build tag. Reports SchemaVersion =
// the highest registered migration (currently 9).
const rollingBaselineImage = "ella-core:rolling-baseline"

// rollingTargetImage is the "new" image: same source compiled with
// `-tags rolling_upgrade_test_synthetic`. Appends 3 trivial migrations
// to the registry, so SchemaVersion = baseline+3.
const rollingTargetImage = "ella-core:rolling-target"

// numSyntheticMigrations matches what migration_synthetic_rolling_upgrade.go
// appends. Drives the expected delta between baseline and target.
const numSyntheticMigrations = 3

// TestIntegrationHARollingUpgrade brings up a 3-node cluster on the
// baseline image, swaps each node one at a time to the target image
// (which carries 3 synthetic post-baseline migrations), and asserts
// the rolling-upgrade machinery behaves correctly throughout:
//
//   - Initial state: every node reports applied = baselineSchema, no
//     pendingMigration.
//   - After each follower swap: the upgraded node's binary advertises
//     a higher SchemaVersion and reports pendingMigration with a
//     laggard pointing at one of the still-baseline voters; the
//     non-upgraded nodes report no pendingMigration (their binaryMax
//     == applied).
//   - After the leader is rolled last: a new leader (one of the
//     upgraded followers) proposes the queued CmdMigrateShared
//     entries via Raft. Applied schema advances baselineSchema →
//     baseline+1 → baseline+2 → baseline+3 in lockstep across every
//     node.
//   - Final state: applied = baseline+3 everywhere, no pendingMigration.
//
// A background writer creates subscribers throughout. Transient
// failures during leadership transitions are tolerated; permanent
// failures fail the test.
//
// Out-of-scope for this test: feature-specific assertions (no v=N+1
// op exists today). When real post-baseline migrations and ops ship
// in the future, per-feature follow-up tests should run on top of
// this orchestration.
func TestIntegrationHARollingUpgrade(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	if err := ensureRollingImages(ctx, t); err != nil {
		t.Skipf("rolling-upgrade images unavailable: %v", err)
	}

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	// Both images get cluster.enabled=true configs from writeNodeConfig
	// at startup. Override compose's per-service image to baseline for
	// every node by exporting the env vars before bringup.
	t.Setenv("ELLA_CORE_1_IMAGE", rollingBaselineImage)
	t.Setenv("ELLA_CORE_2_IMAGE", rollingBaselineImage)
	t.Setenv("ELLA_CORE_3_IMAGE", rollingBaselineImage)

	t.Cleanup(func() {
		dc.ComposeDownWithFile(context.Background(), haRollingComposeDir, ComposeFile())
	})

	// bringUpHAClusterAt uses haNodeServices ("ella-core-{1,2,3}") and
	// the standard ClusterAddressWithPort scheme — both already match
	// our compose.yaml, so no further parameterisation is needed.
	clients, err := bringUpHAClusterAt(ctx, dc, haRollingComposeDir, haNodeServices, nil)
	if err != nil {
		t.Fatalf("bring up cluster on baseline image: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(context.Background(), dc, clients, t.Logf)
	})

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("not all baseline nodes became ready: %v", err)
	}

	// Read the baseline schema version off node 1 — whatever the
	// current source compiled to, that's our N. The target image will
	// be N + numSyntheticMigrations.
	baselineSchema := mustReadSchemaVersion(ctx, t, clients[0])
	targetSchema := baselineSchema + numSyntheticMigrations

	t.Logf("baseline schema = %d, target schema = %d", baselineSchema, targetSchema)

	// Initial state: every node on baseline, applied == baseline, no pending.
	for i, c := range clients {
		assertSchemaState(t, ctx, c, fmt.Sprintf("node %d (initial)", i+1), schemaState{
			schemaVersion:    baselineSchema,
			applied:          baselineSchema,
			pendingMigration: nil,
		})
	}

	// Background writer.
	writer := startSubscriberWriter(t, ctx, clients, "001019756150000")
	t.Cleanup(writer.stop)

	leaderIdx, _, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	// Upgrade order: followers first, leader last. Within followers,
	// any order works.
	upgradeOrder := make([]int, 0, len(clients))

	for i := range clients {
		if i != leaderIdx {
			upgradeOrder = append(upgradeOrder, i)
		}
	}

	upgradeOrder = append(upgradeOrder, leaderIdx)

	for step, nodeIdx := range upgradeOrder {
		nodeNum := nodeIdx + 1
		isLast := step == len(upgradeOrder)-1

		t.Logf("=== rolling step %d/%d: upgrade node %d (was %s) ===",
			step+1, len(upgradeOrder), nodeNum,
			roleAt(ctx, clients[nodeIdx]))

		swapNodeImage(t, ctx, dc, nodeNum, rollingTargetImage)

		if err := waitForNodeReady(ctx, clients[nodeIdx]); err != nil {
			t.Fatalf("node %d did not become ready after swap: %v", nodeNum, err)
		}

		// Wait for the upgraded node to self-announce its new
		// MaxSchemaVersion via the existing cluster_members machinery.
		// Polled because self-announce is async (a goroutine fires it
		// shortly after readiness).
		if err := waitForSchemaCondition(ctx, clients[nodeIdx], func(s *client.Status) error {
			if s.SchemaVersion != targetSchema {
				return fmt.Errorf("schemaVersion=%d, want %d", s.SchemaVersion, targetSchema)
			}

			return nil
		}, 30*time.Second); err != nil {
			t.Fatalf("node %d schemaVersion did not advance: %v", nodeNum, err)
		}

		if !isLast {
			// Mid-window: applied schema must still be baseline (the
			// leader is still on the baseline binary OR the floor of
			// voter MaxSchemaVersion is held down by an unswapped
			// node).
			expectedLaggards := remainingBaselineNodes(upgradeOrder, step+1)

			t.Logf("intermediate: expecting applied=%d, laggard ∈ %v",
				baselineSchema, expectedLaggards)

			// The just-upgraded node reports pendingMigration with one
			// of the still-baseline voters as the laggard.
			waitForPending := func(c *client.Client, label string) {
				if err := waitForSchemaCondition(ctx, c, func(s *client.Status) error {
					if s.Cluster == nil {
						return errors.New("cluster status missing")
					}

					if s.Cluster.AppliedSchemaVersion != baselineSchema {
						return fmt.Errorf("appliedSchemaVersion=%d, want %d",
							s.Cluster.AppliedSchemaVersion, baselineSchema)
					}

					if s.Cluster.PendingMigration == nil {
						return errors.New("pendingMigration nil; want non-nil for upgraded node")
					}

					p := s.Cluster.PendingMigration
					if p.CurrentSchema != baselineSchema {
						return fmt.Errorf("pending.currentSchema=%d, want %d", p.CurrentSchema, baselineSchema)
					}

					// Target may equal current (blocked behind a
					// laggard) or be partway up if some but not all
					// voters allow it. Either is acceptable; a value
					// > targetSchema is not.
					if p.TargetSchema > targetSchema {
						return fmt.Errorf("pending.targetSchema=%d, want <= %d", p.TargetSchema, targetSchema)
					}

					if !contains(expectedLaggards, p.LaggardNodeId) {
						return fmt.Errorf("pending.laggardNodeId=%d not in %v",
							p.LaggardNodeId, expectedLaggards)
					}

					return nil
				}, 15*time.Second); err != nil {
					t.Errorf("%s pendingMigration assertion failed: %v", label, err)
				}
			}

			waitForPending(clients[nodeIdx], fmt.Sprintf("node %d (just upgraded)", nodeNum))

			// Non-upgraded nodes still report no pendingMigration —
			// their local binaryMax equals applied.
			for _, otherIdx := range expectedLaggards {
				oc := clients[otherIdx-1]
				assertSchemaState(t, ctx, oc,
					fmt.Sprintf("node %d (still baseline, mid-roll)", otherIdx),
					schemaState{
						schemaVersion:    baselineSchema,
						applied:          baselineSchema,
						pendingMigration: nil,
					})
			}
		}
	}

	// Final state: every node on target. Once the new leader (from
	// among the previously-upgraded followers, since the original
	// leader was rolled last and the cluster re-elected during the
	// swap) calls CheckPendingMigrations, it sees voter floor =
	// targetSchema and proposes the queued migrations through Raft.
	t.Log("=== final: all nodes on target image; waiting for migrations to apply ===")

	for i, c := range clients {
		if err := waitForSchemaCondition(ctx, c, func(s *client.Status) error {
			if s.Cluster == nil {
				return errors.New("cluster status missing")
			}

			if s.Cluster.AppliedSchemaVersion != targetSchema {
				return fmt.Errorf("appliedSchemaVersion=%d, want %d",
					s.Cluster.AppliedSchemaVersion, targetSchema)
			}

			if s.Cluster.PendingMigration != nil {
				return fmt.Errorf("pendingMigration still non-nil: %+v", s.Cluster.PendingMigration)
			}

			if s.SchemaVersion != targetSchema {
				return fmt.Errorf("schemaVersion=%d, want %d", s.SchemaVersion, targetSchema)
			}

			return nil
		}, 60*time.Second); err != nil {
			t.Fatalf("node %d did not converge to target schema: %v", i+1, err)
		}
	}

	// Stop the writer and validate. Some transient failures during
	// the leader-swap window are expected; permanent failures aren't.
	writeReport, err := writer.stopAndReport()
	if err != nil {
		t.Fatalf("background writer reported a permanent failure: %v", err)
	}

	t.Logf("background writer: %d successes, %d transient failures, %d total attempts",
		writeReport.success, writeReport.transient, writeReport.attempts)

	if writeReport.success == 0 {
		t.Fatal("background writer made zero successful writes; rolling upgrade kept the cluster permanently unwriteable")
	}

	// Membership should still be consistent end-to-end.
	assertMembershipConsistent(t, ctx, clients)
}

// ---------------------------------------------------------------------------
// Helpers below are scoped to this test file. They could move to
// ha_helpers_test.go later if other rolling-upgrade-style tests appear,
// but for one consumer the cohesion of keeping them here is worth more
// than the future-DRY.
// ---------------------------------------------------------------------------

// schemaState is the expected shape of a node's /api/v1/status fields
// at a given point in the test. Each field is asserted independently.
type schemaState struct {
	schemaVersion    int                      // top-level binary max
	applied          int                      // cluster.appliedSchemaVersion
	pendingMigration *client.PendingMigration // expected pendingMigration; nil = absent
}

// assertSchemaState reads /api/v1/status on c and verifies the schema
// fields against expected. Fails the test on mismatch (uses Errorf so
// caller can surface multiple before bailing).
func assertSchemaState(t *testing.T, ctx context.Context, c *client.Client, label string, want schemaState) {
	t.Helper()

	s, err := c.GetStatus(ctx)
	if err != nil {
		t.Errorf("%s: GetStatus: %v", label, err)
		return
	}

	if s.SchemaVersion != want.schemaVersion {
		t.Errorf("%s: schemaVersion=%d, want %d", label, s.SchemaVersion, want.schemaVersion)
	}

	if s.Cluster == nil {
		t.Errorf("%s: cluster status missing", label)
		return
	}

	if s.Cluster.AppliedSchemaVersion != want.applied {
		t.Errorf("%s: appliedSchemaVersion=%d, want %d",
			label, s.Cluster.AppliedSchemaVersion, want.applied)
	}

	switch {
	case want.pendingMigration == nil && s.Cluster.PendingMigration != nil:
		t.Errorf("%s: pendingMigration=%+v, want nil",
			label, s.Cluster.PendingMigration)
	case want.pendingMigration != nil && s.Cluster.PendingMigration == nil:
		t.Errorf("%s: pendingMigration=nil, want %+v",
			label, want.pendingMigration)
	}
}

// waitForSchemaCondition polls c.GetStatus at 500ms cadence until cond
// returns nil or timeout elapses.
func waitForSchemaCondition(ctx context.Context, c *client.Client, cond func(*client.Status) error, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	var lastErr error

	for time.Now().Before(deadline) {
		s, err := c.GetStatus(ctx)
		if err == nil {
			if condErr := cond(s); condErr == nil {
				return nil
			} else {
				lastErr = condErr
			}
		} else {
			lastErr = err
		}

		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("timeout after %s: %w", timeout, lastErr)
}

// mustReadSchemaVersion returns SchemaVersion off c's /status, fatalling
// the test on error.
func mustReadSchemaVersion(ctx context.Context, t *testing.T, c *client.Client) int {
	t.Helper()

	s, err := c.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}

	return s.SchemaVersion
}

// roleAt returns the node's cluster role string for log context.
func roleAt(ctx context.Context, c *client.Client) string {
	s, err := c.GetStatus(ctx)
	if err != nil || s.Cluster == nil {
		return "?"
	}

	return s.Cluster.Role
}

// remainingBaselineNodes returns the 1-based node IDs of nodes in
// upgradeOrder that haven't been upgraded yet (index >= upgradedSoFar).
func remainingBaselineNodes(upgradeOrder []int, upgradedSoFar int) []int {
	out := make([]int, 0, len(upgradeOrder)-upgradedSoFar)

	for i := upgradedSoFar; i < len(upgradeOrder); i++ {
		out = append(out, upgradeOrder[i]+1)
	}

	return out
}

func contains(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}

	return false
}

// swapNodeImage stops node N's container, recreates it with the
// supplied image (via the per-service env-var override), and waits
// for the container to be back up. The Compose volume + IP plan stays
// the same; only the image changes.
func swapNodeImage(t *testing.T, ctx context.Context, dc *DockerClient, nodeNum int, image string) {
	t.Helper()

	service := fmt.Sprintf("ella-core-%d", nodeNum)
	envKey := fmt.Sprintf("ELLA_CORE_%d_IMAGE", nodeNum)

	if err := dc.ComposeStopWithFile(ctx, haRollingComposeDir, ComposeFile(), service); err != nil {
		t.Fatalf("stop %s: %v", service, err)
	}

	if err := dc.ComposeRecreateService(ctx, haRollingComposeDir, ComposeFile(), service, map[string]string{
		envKey: image,
	}); err != nil {
		t.Fatalf("recreate %s with image %s: %v", service, image, err)
	}
}

// ensureRollingImages verifies both rolling-upgrade images are present
// in the local docker daemon. Returns an error if either is missing,
// pointing at the build script. The test treats this as a skip
// condition rather than a failure: a dev environment without the
// images shouldn't block the suite.
func ensureRollingImages(ctx context.Context, t *testing.T) error {
	t.Helper()

	for _, img := range []string{rollingBaselineImage, rollingTargetImage} {
		cmd := exec.CommandContext(ctx, "docker", "image", "inspect", img)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("image %q not found in local docker daemon. "+
				"Run integration/compose/ha-rolling/build-images.sh after building ella-core:latest",
				img)
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Background subscriber writer
// ---------------------------------------------------------------------------

type subscriberWriter struct {
	cancel    context.CancelFunc
	done      chan struct{}
	success   atomic.Int64
	transient atomic.Int64
	attempts  atomic.Int64
	fatalErr  atomic.Pointer[error]
}

type writerReport struct {
	success   int64
	transient int64
	attempts  int64
}

// startSubscriberWriter launches a background goroutine that creates
// subscribers via a round-robin against the supplied clients. Errors
// classified as transient (503, "leadership lost", connection-refused
// during the leader-swap window) are counted but not fatal. Any other
// error fails the test on stopAndReport().
//
// imsiBase is the 15-digit IMSI of the first subscriber; subsequent
// subscribers increment the last digits. Choose a base that doesn't
// collide with any other test's subscribers.
func startSubscriberWriter(t *testing.T, parent context.Context, clients []*client.Client, imsiBase string) *subscriberWriter {
	t.Helper()

	ctx, cancel := context.WithCancel(parent)

	w := &subscriberWriter{
		cancel: cancel,
		done:   make(chan struct{}),
	}

	go func() {
		defer close(w.done)

		ticker := time.NewTicker(200 * time.Millisecond) // ~5 writes/sec
		defer ticker.Stop()

		var counter int64

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			c := clients[counter%int64(len(clients))]
			counter++

			imsi, err := offsetIMSI15(imsiBase, int(counter))
			if err != nil {
				e := fmt.Errorf("compute IMSI: %w", err)
				w.fatalErr.Store(&e)

				return
			}

			w.attempts.Add(1)

			err = c.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
				Imsi:           imsi,
				Key:            "0eefb0893e6f1c2855a3a244c6db1277",
				OPc:            "98da19bbc55e2a5b53857d10557b1d26",
				SequenceNumber: "000000000022",
				ProfileName:    "default",
			})
			switch {
			case err == nil:
				w.success.Add(1)
			case isTransientWriteError(err):
				w.transient.Add(1)
			default:
				e := fmt.Errorf("subscriber %s: %w", imsi, err)
				w.fatalErr.Store(&e)

				return
			}
		}
	}()

	return w
}

// stop cancels the writer and waits for the goroutine to exit.
// Safe to call multiple times.
func (w *subscriberWriter) stop() {
	w.cancel()
	<-w.done
}

// stopAndReport stops the writer and returns the success / transient
// counts. Returns an error if the writer terminated with a non-transient
// error.
func (w *subscriberWriter) stopAndReport() (writerReport, error) {
	w.cancel()
	<-w.done

	report := writerReport{
		success:   w.success.Load(),
		transient: w.transient.Load(),
		attempts:  w.attempts.Load(),
	}

	if e := w.fatalErr.Load(); e != nil {
		return report, *e
	}

	return report, nil
}

// isTransientWriteError matches errors that are expected during a
// leadership transition or rolling-upgrade swap. Conservative: only
// known patterns are tolerated.
func isTransientWriteError(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	for _, fragment := range []string{
		"503",
		"Service Unavailable",
		"leader unreachable",
		"no leader available",
		"leadership lost",
		"leadership changed",
		"context deadline exceeded",
		"connection refused",
		"EOF",
		"connection reset",
	} {
		if strings.Contains(msg, fragment) {
			return true
		}
	}

	return false
}
