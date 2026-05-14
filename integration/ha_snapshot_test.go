package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

// TestIntegrationHASnapshotInstallOnNewJoiner exercises the end-to-end
// Raft snapshot pipeline: Snapshot → Persist → store → InstallSnapshot
// RPC → Restore → Reopen → resume Apply.
//
// Steps:
//
//  1. Bring up a 3-node cluster with aggressive snapshot tunables
//     (snapshot-threshold=50, trailing-logs=5, snapshot-interval=2s).
//     Low TrailingLogs is essential: without it, hashicorp/raft retains
//     10240 trailing log entries after each snapshot, and a fresh joiner
//     could catch up via log replay alone — never exercising
//     InstallSnapshot.
//
//  2. Write ~150 subscribers on the leader, deliberately overshooting
//     the snapshot threshold so the periodic check fires at least one
//     snapshot AND the log gets truncated past the joiner's nextIndex.
//
//  3. Add a 4th node as nonvoter, then promote.
//
//  4. Assert two things:
//     a. The joiner's /data/raft/snapshots/ directory contains at least
//     one snapshot directory. A node can only have a snapshot file
//     from either taking one itself (impossible inside the 2s window
//     between join and the assertion: it has not written anything,
//     so its post-restore log-since-last-snapshot is 0) or receiving
//     InstallSnapshot. Presence ⇒ InstallSnapshot fired.
//     b. The joiner can serve a read of a pre-snapshot subscriber
//     directly (no proxy to leader), proving the snapshot's data
//     landed in its local SQLite.
//
//  5. Continue writing on the leader and assert the joiner keeps up
//     (post-snapshot log replay after Restore).
func TestIntegrationHASnapshotInstallOnNewJoiner(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	// Snapshot test reuses the 4-service scaleup topology; the only
	// thing that changes is per-node core.yaml content.
	const composeDir = "compose/ha-scaleup/"

	ipFamily := DetectIPFamily()
	t.Logf("Running HA snapshot install test in %s mode", ipFamily)

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

	composeFile := ComposeFile()

	dockerClient.ComposeCleanup(ctx)
	t.Cleanup(func() {
		dockerClient.ComposeDownWithFile(ctx, composeDir, composeFile)
	})

	t.Log("bringing up 3-node cluster with aggressive snapshot tunables")

	// Include node 4's peer in every config from the start so the joiner
	// doesn't need a separate config rewrite of existing nodes.
	fullPeers := []string{
		ClusterAddressWithPort(1, 7000),
		ClusterAddressWithPort(2, 7000),
		ClusterAddressWithPort(3, 7000),
		ClusterAddressWithPort(4, 7000),
	}

	snapshotCfg := snapshotTunables{
		Interval:     "2s",
		Threshold:    50,
		TrailingLogs: 5,
	}

	clients, err := bringUpHASnapshotCluster(t, ctx, dockerClient, composeDir, fullPeers, snapshotCfg)
	if err != nil {
		t.Fatalf("bring up 3-node cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(t, ctx, dockerClient, composeDir, haNodeServices, clients)
	})

	_, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("failed to find leader: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("not all nodes became ready: %v", err)
	}

	// Phase 1: write enough subscribers to force a snapshot + log truncation.
	const preJoinSubscribers = 150

	t.Logf("writing %d subscribers to force snapshot + log truncation", preJoinSubscribers)

	preJoinIMSIs := make([]string, 0, preJoinSubscribers)

	for i := 0; i < preJoinSubscribers; i++ {
		imsi := snapshotIMSI(i)
		preJoinIMSIs = append(preJoinIMSIs, imsi)

		if err := leader.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
			Imsi:           imsi,
			Key:            "0eefb0893e6f1c2855a3a244c6db1277",
			OPc:            "98da19bbc55e2a5b53857d10557b1d26",
			SequenceNumber: "000000000022",
			ProfileName:    "default",
		}); err != nil {
			t.Fatalf("create subscriber %q (i=%d): %v", imsi, i, err)
		}
	}

	preJoinIdx, err := leaderAppliedIndex(ctx, leader)
	if err != nil {
		t.Fatalf("get leader applied index: %v", err)
	}

	t.Logf("leader applied index after pre-join writes: %d", preJoinIdx)

	if err := waitForFollowerConvergence(ctx, clients, preJoinIdx); err != nil {
		t.Fatalf("followers did not converge after pre-join writes: %v", err)
	}

	// Wait long enough for the periodic snapshot check (interval=2s) to
	// fire at least once after the threshold was crossed.
	t.Log("waiting for leader to take a snapshot")

	leaderContainer, err := dockerClient.ResolveComposeContainer(ctx, "ha-scaleup", "ella-core-1")
	if err != nil {
		t.Fatalf("resolve leader container: %v", err)
	}

	if err := waitForSnapshotFile(ctx, dockerClient, leaderContainer, 30*time.Second); err != nil {
		t.Fatalf("leader did not take a snapshot: %v", err)
	}

	t.Log("leader has at least one snapshot on disk; staging + starting node 4 as nonvoter")

	if err := stageAndStartJoinerWithSnapshotConfig(ctx, dockerClient, leader, composeDir,
		"ella-core-4", 4, fullPeers, "nonvoter", snapshotCfg); err != nil {
		t.Fatalf("stage + start node 4: %v", err)
	}

	node4URL := APIAddressForCluster(4)

	node4Client, err := newInsecureClient(node4URL)
	if err != nil {
		t.Fatalf("client for node 4: %v", err)
	}

	node4Client.SetToken(clients[0].GetToken())

	if err := waitForMemberSuffrage(ctx, leader, 4, "nonvoter"); err != nil {
		t.Fatalf("node 4 did not join as nonvoter: %v", err)
	}

	t.Log("node 4 joined as nonvoter; promoting to voter")

	if err := leader.PromoteClusterMember(ctx, 4); err != nil {
		t.Fatalf("promote node 4: %v", err)
	}

	if err := waitForMemberSuffrage(ctx, leader, 4, "voter"); err != nil {
		t.Fatalf("node 4 did not become voter: %v", err)
	}

	// Wait for node 4 to catch up to the pre-join index. This is the
	// path under test: the leader's log no longer contains entries
	// before (snapshotIndex - trailingLogs), so node 4 must receive
	// InstallSnapshot.
	if err := waitForFollowerConvergence(ctx, []*client.Client{node4Client}, preJoinIdx); err != nil {
		t.Fatalf("node 4 did not converge to pre-join index %d: %v", preJoinIdx, err)
	}

	t.Logf("node 4 converged to applied index %d", preJoinIdx)

	// Assertion (a): node 4 has a snapshot directory on disk. It cannot
	// have generated one itself yet — it just joined, has done zero
	// writes locally, and the snapshot threshold check uses logs-since-
	// last-snapshot which is at most a handful of replicated entries
	// since Restore reset its counter. Any snapshot file therefore came
	// from InstallSnapshot.
	node4Container, err := dockerClient.ResolveComposeContainer(ctx, "ha-scaleup", "ella-core-4")
	if err != nil {
		t.Fatalf("resolve node 4 container: %v", err)
	}

	if err := assertSnapshotInstalled(ctx, dockerClient, node4Container); err != nil {
		t.Fatalf("InstallSnapshot did not fire on node 4: %v", err)
	}

	t.Log("node 4 has snapshot directory on disk — InstallSnapshot path exercised")

	// Assertion (b): a representative sample of pre-snapshot subscribers
	// is readable from node 4 directly (read goes to local FSM, not
	// proxied to leader). Sampling the first, middle, and last covers
	// rows that were written well before any snapshot AND rows that
	// straddle the snapshot boundary.
	for _, idx := range []int{0, preJoinSubscribers / 2, preJoinSubscribers - 1} {
		imsi := preJoinIMSIs[idx]

		sub, err := node4Client.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsi})
		if err != nil {
			t.Fatalf("read pre-snapshot subscriber %q from node 4: %v", imsi, err)
		}

		if sub.Imsi != imsi {
			t.Fatalf("node 4 returned IMSI %q, expected %q", sub.Imsi, imsi)
		}
	}

	t.Log("node 4 served pre-snapshot reads locally; writing more entries to exercise post-snapshot replay")

	// Phase 2: continue writing on the leader and verify node 4 keeps
	// up. This validates the post-Restore Apply path: after Restore
	// resets the FSM, subsequent committed log entries must still apply.
	const postJoinSubscribers = 50

	for i := 0; i < postJoinSubscribers; i++ {
		imsi := snapshotIMSI(preJoinSubscribers + i)

		if err := leader.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
			Imsi:           imsi,
			Key:            "0eefb0893e6f1c2855a3a244c6db1277",
			OPc:            "98da19bbc55e2a5b53857d10557b1d26",
			SequenceNumber: "000000000022",
			ProfileName:    "default",
		}); err != nil {
			t.Fatalf("create post-join subscriber %q (i=%d): %v", imsi, i, err)
		}
	}

	postJoinIdx, err := leaderAppliedIndex(ctx, leader)
	if err != nil {
		t.Fatalf("get leader applied index post-join: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, []*client.Client{node4Client}, postJoinIdx); err != nil {
		t.Fatalf("node 4 did not converge post-join: %v", err)
	}

	lastIMSI := snapshotIMSI(preJoinSubscribers + postJoinSubscribers - 1)

	sub, err := node4Client.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: lastIMSI})
	if err != nil {
		t.Fatalf("read post-snapshot subscriber %q from node 4: %v", lastIMSI, err)
	}

	if sub.Imsi != lastIMSI {
		t.Fatalf("node 4 returned post-snapshot IMSI %q, expected %q", sub.Imsi, lastIMSI)
	}

	t.Log("node 4 served post-snapshot reads locally; snapshot install + replay validated")

	assertMembershipConsistent(t, ctx, clients)
}

// snapshotIMSI returns a deterministic 15-digit IMSI for index i.
// Range chosen to be disjoint from IMSIs used by other HA tests.
func snapshotIMSI(i int) string {
	return fmt.Sprintf("00101975614%04d", i)
}

// snapshotTunables are the three knobs that govern when Raft snapshots
// fire and how aggressively the log is truncated afterwards. Test-only
// values are deliberately low — production defaults would never produce
// a snapshot in the lifetime of an integration test.
type snapshotTunables struct {
	Interval     string
	Threshold    uint64
	TrailingLogs uint64
}

// bringUpHASnapshotCluster mirrors bringUpHAClusterAt but writes each
// node's core.yaml via writeSnapshotNodeConfig so the snapshot tunables
// take effect from process start. Returns admin-authed clients for the
// 3 initial nodes (node 4 is started later by the test body).
func bringUpHASnapshotCluster(t *testing.T, ctx context.Context, dc *DockerClient, composeDir string, peers []string, cfg snapshotTunables) ([]*client.Client, error) {
	t.Helper()

	dc.ComposeCleanup(ctx)

	services := haNodeServices

	fail := func(err error) ([]*client.Client, error) {
		captureClusterLogs(t, dc, composeDir, services)
		return nil, err
	}

	if err := writeSnapshotNodeConfig(composeDir, 1, peers, "", "", cfg); err != nil {
		return fail(err)
	}

	composeFile := ComposeFile()

	if err := dc.ComposeUpServicesWithFile(ctx, composeDir, composeFile, services[0]); err != nil {
		return fail(fmt.Errorf("start node 1: %w", err))
	}

	node1, err := newInsecureClient(getHANodeURLs()[0])
	if err != nil {
		return fail(err)
	}

	if err := waitForNodeReady(ctx, node1); err != nil {
		return fail(fmt.Errorf("node 1 never became ready: %w", err))
	}

	adminToken, err := initializeAndGetAdminToken(ctx, node1)
	if err != nil {
		return fail(err)
	}

	node1.SetToken(adminToken)

	for i := 1; i < len(services); i++ {
		nodeID := i + 1

		if err := stageAndStartJoinerWithSnapshotConfig(ctx, dc, node1, composeDir,
			services[i], nodeID, peers, "", cfg); err != nil {
			return fail(err)
		}
	}

	clients, err := newHANodeClients()
	if err != nil {
		return fail(err)
	}

	for _, c := range clients {
		c.SetToken(adminToken)
	}

	if err := waitForClusterReady(ctx, clients); err != nil {
		return fail(fmt.Errorf("cluster not ready: %w", err))
	}

	return clients, nil
}

// stageAndStartJoinerWithSnapshotConfig mints a join token, writes the
// joiner's core.yaml with both the token and the snapshot tunables,
// then starts the service.
func stageAndStartJoinerWithSnapshotConfig(ctx context.Context, dc *DockerClient, leader *client.Client, composeDir, service string, nodeID int, peers []string, initialSuffrage string, cfg snapshotTunables) error {
	tok, err := leader.MintClusterJoinToken(ctx, &client.MintJoinTokenOptions{
		NodeID:     nodeID,
		TTLSeconds: 600,
	})
	if err != nil {
		return fmt.Errorf("mint join token for node %d: %w", nodeID, err)
	}

	if err := writeSnapshotNodeConfig(composeDir, nodeID, peers, tok.Token, initialSuffrage, cfg); err != nil {
		return err
	}

	return dc.ComposeUpServicesWithFile(ctx, composeDir, ComposeFile(), service)
}

// writeSnapshotNodeConfig is writeNodeConfig + the three snapshot knobs.
// Kept local to this file because no other test cares about these knobs;
// folding them into the shared helper would clutter every other call site.
func writeSnapshotNodeConfig(composeDir string, nodeID int, peers []string, joinToken, initialSuffrage string, cfg snapshotTunables) error {
	cfgDir, err := filepath.Abs(filepath.Join(composeDir, "cfg", fmt.Sprintf("node%d", nodeID)))
	if err != nil {
		return fmt.Errorf("abs path %s: %w", composeDir, err)
	}

	if err := os.MkdirAll(cfgDir, 0o777); err != nil {
		return fmt.Errorf("mkdir %s: %w", cfgDir, err)
	}

	if err := os.Chmod(cfgDir, 0o777); err != nil {
		return fmt.Errorf("chmod %s: %w", cfgDir, err)
	}

	addr := ClusterAddress(nodeID)

	var peersYAML strings.Builder

	for _, p := range peers {
		fmt.Fprintf(&peersYAML, "      - %q\n", p)
	}

	joinTokenLine := ""
	if joinToken != "" {
		joinTokenLine = fmt.Sprintf("  join-token: %q\n", joinToken)
	}

	suffrageLine := ""
	if initialSuffrage != "" {
		suffrageLine = fmt.Sprintf("  initial-suffrage: %q\n", initialSuffrage)
	}

	body := fmt.Sprintf(`logging:
  system:
    level: "debug"
    output: "stdout"
  audit:
    output: "stdout"
db:
  path: "/data/ella.db"
interfaces:
  n2:
    address: %q
    port: 38412
  n3:
    name: "eth0"
  n6:
    name: "n6"
  api:
    address: %q
    port: 5002
xdp:
  attach-mode: "generic"
cluster:
  enabled: true
  node-id: %d
  bind-address: %q
  snapshot-interval: %q
  snapshot-threshold: %d
  trailing-logs: %d
  peers:
%s%s%s`,
		addr, addr, nodeID, ClusterAddressWithPort(nodeID, 7000),
		cfg.Interval, cfg.Threshold, cfg.TrailingLogs,
		peersYAML.String(), joinTokenLine, suffrageLine)

	return os.WriteFile(filepath.Join(cfgDir, "core.yaml"), []byte(body), 0o644)
}

// waitForSnapshotFile polls the container's /data/raft/snapshots/
// directory until at least one snapshot subdirectory exists.
// hashicorp/raft's FileSnapshotStore writes snapshots as
// `<dir>/snapshots/<term>-<index>-<timestamp>/` plus an in-progress
// `tmp/` directory; we look for any non-tmp entry.
func waitForSnapshotFile(ctx context.Context, dc *DockerClient, container string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	var lastOut string

	for time.Now().Before(deadline) {
		out, err := dc.Exec(ctx, container, []string{"ls", "-1", "/data/raft/snapshots"}, false, 5*time.Second, nil)
		if err == nil {
			lastOut = out

			if hasSnapshotEntry(out) {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	return fmt.Errorf("no snapshot in %s after %s; last ls output: %q", container, timeout, lastOut)
}

// assertSnapshotInstalled is the synchronous variant used after the
// joiner has already converged. By that point, if a snapshot was
// installed, it has been on disk for at least one full apply cycle.
func assertSnapshotInstalled(ctx context.Context, dc *DockerClient, container string) error {
	out, err := dc.Exec(ctx, container, []string{"ls", "-1", "/data/raft/snapshots"}, false, 5*time.Second, nil)
	if err != nil {
		return fmt.Errorf("ls /data/raft/snapshots on %s: %w", container, err)
	}

	if !hasSnapshotEntry(out) {
		return fmt.Errorf("no snapshot entry in /data/raft/snapshots on %s; ls output: %q", container, out)
	}

	return nil
}

// hasSnapshotEntry returns true when the `ls` output of
// /data/raft/snapshots contains at least one entry that is not the
// in-progress `tmp` directory and not blank.
func hasSnapshotEntry(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" || name == "tmp" {
			continue
		}

		return true
	}

	return false
}
