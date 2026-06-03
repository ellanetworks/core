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
)

// TestIntegrationHASnapshotInstallOnNewJoiner forces a Raft snapshot
// and log truncation on the leader, then joins a new voter and asserts
// it receives the snapshot via InstallSnapshot (rather than catching
// up by log replay), reads pre-snapshot rows locally, and continues to
// replicate writes after the install.
func TestIntegrationHASnapshotInstallOnNewJoiner(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	const composeDir = "compose/ha-scaleup/"

	HALogf(t, "Running HA snapshot install test in %s mode", DetectIPFamily())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	dockerClient, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	defer func() {
		if err := dockerClient.Close(); err != nil {
			HALogf(t, "failed to close docker client: %v", err)
		}
	}()

	composeFile := ComposeFile()

	dockerClient.ComposeCleanup(ctx)
	t.Cleanup(func() {
		dockerClient.ComposeDownWithFile(ctx, composeDir, composeFile)
	})

	fullPeers := []string{
		ClusterAddressWithPort(1, 7000),
		ClusterAddressWithPort(2, 7000),
		ClusterAddressWithPort(3, 7000),
		ClusterAddressWithPort(4, 7000),
	}

	// Low TrailingLogs forces the leader's log to be truncated past a
	// fresh joiner's nextIndex, so the joiner cannot catch up by log
	// replay alone — it must receive an InstallSnapshot RPC.
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
		t.Fatalf("find leader: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("nodes not ready: %v", err)
	}

	const preJoinSubscribers = 150

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
		t.Fatalf("leader applied index: %v", err)
	}

	HALogf(t, "leader applied index after pre-join writes: %d", preJoinIdx)

	if err := waitForFollowerConvergence(ctx, clients, preJoinIdx); err != nil {
		t.Fatalf("followers did not converge: %v", err)
	}

	leaderContainer, err := dockerClient.ResolveComposeContainer(ctx, "ha-scaleup", "ella-core-1")
	if err != nil {
		t.Fatalf("resolve leader container: %v", err)
	}

	hostTmp := t.TempDir()

	if err := waitForSnapshotFile(ctx, leaderContainer, hostTmp, 30*time.Second); err != nil {
		t.Fatalf("leader did not take a snapshot: %v", err)
	}

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

	if err := leader.PromoteClusterMember(ctx, 4); err != nil {
		t.Fatalf("promote node 4: %v", err)
	}

	if err := waitForMemberSuffrage(ctx, leader, 4, "voter"); err != nil {
		t.Fatalf("node 4 did not become voter: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, []*client.Client{node4Client}, preJoinIdx); err != nil {
		t.Fatalf("node 4 did not converge to index %d: %v", preJoinIdx, err)
	}

	node4Container, err := dockerClient.ResolveComposeContainer(ctx, "ha-scaleup", "ella-core-4")
	if err != nil {
		t.Fatalf("resolve node 4 container: %v", err)
	}

	if err := assertSnapshotInstalled(ctx, node4Container, hostTmp); err != nil {
		t.Fatalf("InstallSnapshot did not fire on node 4: %v", err)
	}

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
		t.Fatalf("leader applied index post-join: %v", err)
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
		t.Fatalf("node 4 returned IMSI %q, expected %q", sub.Imsi, lastIMSI)
	}

	assertMembershipConsistent(t, ctx, clients)
}

func snapshotIMSI(i int) string {
	return fmt.Sprintf("00101975614%04d", i)
}

type snapshotTunables struct {
	Interval     string
	Threshold    uint64
	TrailingLogs uint64
}

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

func waitForSnapshotFile(ctx context.Context, container, hostTmp string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	var lastOut, lastErr string

	for time.Now().Before(deadline) {
		out, err := listSnapshotDir(ctx, container, hostTmp)
		lastOut = out

		if err != nil {
			lastErr = err.Error()
		} else {
			lastErr = ""

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

	return fmt.Errorf("no snapshot in %s after %s; last out=%q lastErr=%q",
		container, timeout, lastOut, lastErr)
}

func assertSnapshotInstalled(ctx context.Context, container, hostTmp string) error {
	out, err := listSnapshotDir(ctx, container, hostTmp)
	if err != nil {
		return fmt.Errorf("list /data/raft/snapshots on %s: %w (out=%q)", container, err, out)
	}

	if !hasSnapshotEntry(out) {
		return fmt.Errorf("no snapshot entry in /data/raft/snapshots on %s; out=%q", container, out)
	}

	return nil
}

// listSnapshotDir enumerates /data/raft/snapshots inside a container.
// The image ships no shell utilities, so this copies the directory to
// the host with docker cp and reads it from Go.
func listSnapshotDir(ctx context.Context, container, hostTmp string) (string, error) {
	dest, err := os.MkdirTemp(hostTmp, "snap-")
	if err != nil {
		return "", fmt.Errorf("mkdir host scratch: %w", err)
	}

	defer func() { _ = os.RemoveAll(dest) }()

	cpCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cpCtx, "docker", "cp",
		fmt.Sprintf("%s:/data/raft/snapshots/.", container),
		dest,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return string(out), fmt.Errorf("docker cp: %w", err)
	}

	entries, err := os.ReadDir(dest)
	if err != nil {
		return "", fmt.Errorf("read copied dir: %w", err)
	}

	var b strings.Builder

	for _, e := range entries {
		b.WriteString(e.Name())
		b.WriteByte('\n')
	}

	return b.String(), nil
}

// hasSnapshotEntry reports whether `out` contains at least one
// snapshot directory name. The in-progress `tmp` subdirectory used by
// hashicorp/raft's FileSnapshotStore is ignored.
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
