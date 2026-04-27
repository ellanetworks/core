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

const (
	haRollingComposeDir = "compose/ha-rolling/"

	// rollingBaselineImage: current source, no build tag.
	rollingBaselineImage = "ella-core:rolling-baseline"
	// rollingTargetImage: built with -tags rolling_upgrade_test_synthetic,
	// which appends 3 no-op migrations to the registry.
	rollingTargetImage = "ella-core:rolling-target"

	// Must match migration_synthetic_rolling_upgrade.go.
	numSyntheticMigrations = 3
)

// TestIntegrationHARollingUpgrade rolls a 3-node cluster from baseline
// to target image one node at a time and asserts cluster state
// transitions through /api/v1/status (cluster.appliedSchemaVersion,
// cluster.pendingMigration). A background writer validates the
// cluster stays writable throughout.
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

	t.Setenv("ELLA_CORE_1_IMAGE", rollingBaselineImage)
	t.Setenv("ELLA_CORE_2_IMAGE", rollingBaselineImage)
	t.Setenv("ELLA_CORE_3_IMAGE", rollingBaselineImage)

	t.Cleanup(func() {
		dc.ComposeDownWithFile(context.Background(), haRollingComposeDir, ComposeFile())
	})

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

	baselineSchema := mustReadSchemaVersion(ctx, t, clients[0])
	targetSchema := baselineSchema + numSyntheticMigrations

	t.Logf("baseline schema = %d, target schema = %d", baselineSchema, targetSchema)

	for i, c := range clients {
		assertSchemaState(t, ctx, c, fmt.Sprintf("node %d (initial)", i+1), schemaState{
			schemaVersion:    baselineSchema,
			applied:          baselineSchema,
			pendingMigration: nil,
		})
	}

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

		// Self-announce of MaxSchemaVersion is async; poll.
		if err := waitForSchemaCondition(ctx, clients[nodeIdx], func(s *client.Status) error {
			if s.SchemaVersion != targetSchema {
				return fmt.Errorf("schemaVersion=%d, want %d", s.SchemaVersion, targetSchema)
			}

			return nil
		}, 30*time.Second); err != nil {
			t.Fatalf("node %d schemaVersion did not advance: %v", nodeNum, err)
		}

		if !isLast {
			expectedLaggards := remainingBaselineNodes(upgradeOrder, step+1)

			t.Logf("intermediate: expecting applied=%d, laggard ∈ %v",
				baselineSchema, expectedLaggards)

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

					// Target ≤ targetSchema: may equal current (blocked)
					// or be partway up.
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

	t.Log("=== final: all nodes on target; waiting for migrations to apply ===")

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
		t.Fatal("background writer made zero successful writes; cluster was permanently unwriteable")
	}

	assertMembershipConsistent(t, ctx, clients)
}

type schemaState struct {
	schemaVersion    int
	applied          int
	pendingMigration *client.PendingMigration // nil = expect absent
}

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

func mustReadSchemaVersion(ctx context.Context, t *testing.T, c *client.Client) int {
	t.Helper()

	s, err := c.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}

	return s.SchemaVersion
}

func roleAt(ctx context.Context, c *client.Client) string {
	s, err := c.GetStatus(ctx)
	if err != nil || s.Cluster == nil {
		return "?"
	}

	return s.Cluster.Role
}

// remainingBaselineNodes returns 1-based node IDs not yet upgraded.
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

// startSubscriberWriter creates subscribers round-robin at ~5/s.
// Transient errors (see isTransientWriteError) are counted; any other
// error fails stopAndReport. imsiBase must be a 15-digit IMSI not
// shared with other tests.
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

func (w *subscriberWriter) stop() {
	w.cancel()
	<-w.done
}

// stopAndReport returns an error iff the writer terminated with a
// non-transient failure.
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

// isTransientWriteError matches errors expected during leadership
// transitions; conservative — only known patterns are tolerated.
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
