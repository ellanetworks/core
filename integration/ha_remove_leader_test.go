package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

// TestIntegrationHARemoveLeader exercises the common operator workflow
// "retire the current leader." Today the only remove-member coverage is
// TestIntegrationHAScaleUpDown, which removes the joiner (node 4), never
// the leader. This test pins the contract operators rely on when they
// decommission the node that happens to be leader at the moment.
//
// What this test pins:
//
//  1. Drain on the leader transfers leadership; a new leader emerges.
//  2. RemoveClusterMember(leaderID) succeeds after the drain.
//  3. Writes against the survivors continue throughout the transition
//     with bounded transient errors and zero permanent failures.
//  4. The removed leader is fenced from accepting writes against itself
//     (matches the scale-down fence assertion).
//  5. Cluster_members reports exactly 2 members after the operation,
//     consistent across survivors.
//  6. The 2-node survivor cluster is steady-state writable.
//
// The background writer targets only the surviving nodes so write
// attempts against the soon-to-be-removed leader are not counted; the
// removed leader's fence behaviour is asserted separately. This keeps
// the test's transient-error bound focused on the genuine leadership-
// transition window rather than on requests routed to a doomed peer.
func TestIntegrationHARemoveLeader(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	dockerClient, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	defer func() {
		if err := dockerClient.Close(); err != nil {
			t.Logf("failed to close docker client: %v", err)
		}
	}()

	clients, err := bringUpHACluster(t, ctx, dockerClient)
	if err != nil {
		t.Fatalf("bring up HA cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(t, ctx, dockerClient, haComposeDir, haNodeServices, clients)
	})

	leaderIdx, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("nodes not ready: %v", err)
	}

	leaderStatus, err := leader.GetStatus(ctx)
	if err != nil || leaderStatus.Cluster == nil {
		t.Fatalf("read leader status: %v", err)
	}

	leaderNodeID := leaderStatus.Cluster.NodeID

	t.Logf("removing leader: nodeID=%d (clients[%d])", leaderNodeID, leaderIdx)

	// Build the survivor client slice — every client except the current
	// leader. Round-robin writes against these continue across the
	// drain/remove transition without ever hitting the doomed peer's
	// fence (which would surface as a permanent 502).
	survivors := make([]*client.Client, 0, len(clients)-1)

	for i, c := range clients {
		if i != leaderIdx {
			survivors = append(survivors, c)
		}
	}

	// Start the background writer before any membership change. IMSI
	// base is disjoint from every other HA test's range.
	writer := startSubscriberWriter(t, ctx, survivors, "001019756160000")

	// Phase 1: drain the leader. DrainClusterMember on the leader
	// transfers leadership; the existing TestIntegrationHADrainLeadership
	// proves this in isolation. Here it's a precondition for the
	// remove call (Remove without drain returns 4xx unless force=true,
	// per client/cluster.go:157-178).
	drainResp, err := leader.DrainClusterMember(ctx, leaderNodeID, &client.DrainOptions{DeadlineSeconds: 30})
	if err != nil {
		writer.stop()
		t.Fatalf("drain leader: %v", err)
	}

	if drainResp.DrainState != "draining" && drainResp.DrainState != "drained" {
		writer.stop()
		t.Fatalf("post-drain state = %q, want draining or drained", drainResp.DrainState)
	}

	newLeader, err := waitForNewLeader(ctx, survivors)
	if err != nil {
		writer.stop()
		t.Fatalf("no new leader after drain: %v", err)
	}

	if err := waitForDrainState(ctx, newLeader, leaderNodeID, "drained", 60*time.Second); err != nil {
		writer.stop()
		t.Fatalf("former leader did not reach drained state: %v", err)
	}

	// Phase 2: remove the (now-drained, no-longer-leader) member from
	// the raft configuration.
	t.Logf("removing former leader nodeID=%d from cluster", leaderNodeID)

	if err := newLeader.RemoveClusterMember(ctx, leaderNodeID, false); err != nil {
		writer.stop()
		t.Fatalf("RemoveClusterMember(%d): %v", leaderNodeID, err)
	}

	if err := waitForMemberCount(ctx, newLeader, 2, 60*time.Second); err != nil {
		writer.stop()
		t.Fatalf("cluster did not shrink to 2 members: %v", err)
	}

	// Phase 3: stop the writer and validate. Some transient errors are
	// expected during the leadership transition; permanent errors are not.
	report, werr := writer.stopAndReport()
	if werr != nil {
		t.Fatalf("background writer reported a permanent failure: %v", werr)
	}

	t.Logf("background writer: success=%d transient=%d attempts=%d",
		report.success, report.transient, report.attempts)

	if report.success == 0 {
		t.Fatal("zero successful writes during leader-remove window; cluster was permanently unwriteable")
	}

	// Cap transient errors at half the total attempts. The leadership-
	// transfer window is bounded (election timeout + a few RPC retries)
	// so the steady-state survivor writes after that window should
	// dominate.
	if report.transient > report.attempts/2 {
		t.Fatalf("transient error rate too high: %d/%d", report.transient, report.attempts)
	}

	// Phase 4: the removed leader's API must reject writes against
	// itself (matches the existing scale-down fence assertion in
	// TestIntegrationHAScaleUpDown).
	if _, err := leader.GetStatus(ctx); err != nil {
		t.Fatalf("removed leader unreachable, cannot exercise fence: %v", err)
	}

	const fencedIMSI = "001019756160999"

	if err := leader.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi:           fencedIMSI,
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	}); err == nil {
		t.Fatal("CreateSubscriber via removed leader succeeded; fence regression")
	} else {
		t.Logf("write via removed leader correctly rejected: %v", err)
	}

	if _, err := newLeader.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: fencedIMSI}); err == nil {
		t.Fatal("subscriber written via removed leader landed on the cluster; fence is broken")
	} else if !isExpectedNotFound(err) {
		t.Logf("unexpected error reading fenced IMSI (acceptable but worth logging): %v", err)
	}

	// Phase 5: cluster_members consistent across the 2 survivors and
	// the surviving 2-node cluster accepts a fresh steady-state write.
	assertMembershipConsistent(t, ctx, survivors)

	const postRemoveIMSI = "001019756161000"

	if err := newLeader.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi:           postRemoveIMSI,
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	}); err != nil {
		t.Fatalf("steady-state write on 2-node cluster: %v", err)
	}

	idx, err := leaderAppliedIndex(ctx, newLeader)
	if err != nil {
		t.Fatalf("leader applied index: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, survivors, idx); err != nil {
		t.Fatalf("surviving follower did not converge after steady-state write: %v", err)
	}
}

// waitForMemberCount polls ListClusterMembers on c until the count
// matches want, or timeout elapses.
func waitForMemberCount(ctx context.Context, c *client.Client, want int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	var last int

	for time.Now().Before(deadline) {
		members, err := c.ListClusterMembers(ctx)
		if err == nil {
			last = len(members)
			if last == want {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	return &memberCountError{want: want, got: last, timeout: timeout}
}

type memberCountError struct {
	want, got int
	timeout   time.Duration
}

func (e *memberCountError) Error() string {
	return "expected " + itoaInt(e.want) + " members, got " + itoaInt(e.got) + " after " + e.timeout.String()
}

func itoaInt(n int) string {
	if n == 0 {
		return "0"
	}

	neg := n < 0
	if neg {
		n = -n
	}

	var buf [20]byte

	i := len(buf)

	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	if neg {
		i--
		buf[i] = '-'
	}

	return string(buf[i:])
}

// isExpectedNotFound matches the "subscriber doesn't exist" error
// returned by GetSubscriber. The fence assertion above prefers this
// over a stricter equality check because the error text passes through
// multiple wrapping layers.
func isExpectedNotFound(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	return strings.Contains(msg, "not found") || strings.Contains(msg, "404")
}
