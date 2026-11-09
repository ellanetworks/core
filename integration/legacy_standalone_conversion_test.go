// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/suites"
)

var legacyChain = []string{
	"v1.19.0",
	"v1.20.0",
	"v1.21.0",
}

const (
	raftSnapshotThreshold = 8192

	legacyChurnEntries = raftSnapshotThreshold + 200

	legacySnapshotWait = 5 * time.Minute
)

func legacyImage(tag string) string {
	return "ella-core:legacy-" + tag
}

func TestIntegrationLegacyStandaloneConversion(t *testing.T) {
	suites.Require(t, suites.LegacyConversion)

	beginHATest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	ensureLegacyChainImages(t, ctx)

	dc := newStandaloneUpgradeDocker(t, ctx)

	node, oldest := startStandaloneOnImage(t, ctx, dc, legacyImage(legacyChain[0]))

	snapshotIMSIs := createStandaloneSubscribers(t, ctx, node, "001019756180000", 5)

	container, err := dc.ResolveComposeContainer(ctx, "ha-rolling", standaloneUpgradeService)
	if err != nil {
		t.Fatalf("resolve standalone container: %v", err)
	}

	scratch := t.TempDir()

	churnUntilSnapshot(t, ctx, node, container, scratch)

	snapshotStamp := newestSnapshotSchema(t, ctx, container, scratch)
	if snapshotStamp != oldest.SchemaVersion {
		t.Fatalf("snapshot on %s stamped schema %d, want %d", legacyChain[0], snapshotStamp, oldest.SchemaVersion)
	}

	previous := oldest

	var afterSnapshotIMSIs []string

	for i, tag := range legacyChain[1:] {
		swapNodeImage(t, ctx, dc, 1, legacyImage(tag))

		if err := waitForNodeReady(ctx, node); err != nil {
			t.Fatalf("node not ready on %s: %v", tag, err)
		}

		status := mustReadStatus(ctx, t, node)
		if status.SchemaVersion < previous.SchemaVersion {
			t.Fatalf("%s schema %d < previous release schema %d; the chain must not downgrade",
				tag, status.SchemaVersion, previous.SchemaVersion)
		}

		HALogf(t, "hop to %s: version %s, schema %d", tag, status.Version, status.SchemaVersion)

		assertSubscribersPresent(t, ctx, node, snapshotIMSIs, "after the hop to "+tag)

		base := fmt.Sprintf("00101975619%d000", i)
		afterSnapshotIMSIs = append(afterSnapshotIMSIs, createStandaloneSubscribers(t, ctx, node, base, 3)...)

		previous = status
	}

	if stamp := newestSnapshotSchema(t, ctx, container, scratch); stamp >= previous.SchemaVersion {
		t.Fatalf("newest snapshot stamped schema %d, live schema %d: no schema gap left to convert; "+
			"a later snapshot absorbed it, or the chain has no local migration (check legacyChain)",
			stamp, previous.SchemaVersion)
	}

	HALogf(t, "legacy store ready: snapshot at schema %d, live schema %d", snapshotStamp, previous.SchemaVersion)

	if err := writeNodeConfig(haRollingComposeDir, 1, nil, "", ""); err != nil {
		t.Fatalf("write cluster config: %v", err)
	}

	swapNodeImage(t, ctx, dc, 1, rollingTargetImage)

	if err := waitForNodeReady(ctx, node); err != nil {
		t.Fatalf("legacy store did not come up as a cluster seed on %s: %v", rollingTargetImage, err)
	}

	target := mustReadStatus(ctx, t, node)
	if !target.Cluster.Enabled {
		t.Fatal("the converted node must run in cluster mode")
	}

	if target.SchemaVersion < previous.SchemaVersion {
		t.Fatalf("target schema %d < %s schema %d; rebuild %s from a tree newer than the chain",
			target.SchemaVersion, legacyChain[len(legacyChain)-1], previous.SchemaVersion, rollingTargetImage)
	}

	if err := waitForSchemaCondition(ctx, node, func(s *client.Status) error {
		if s.Cluster.AppliedSchemaVersion != target.SchemaVersion {
			return fmt.Errorf("applied schema %d, want %d", s.Cluster.AppliedSchemaVersion, target.SchemaVersion)
		}

		return nil
	}, 60*time.Second); err != nil {
		t.Fatalf("converted seed did not migrate to the target schema: %v", err)
	}

	assertSeedCheckpointed(t, ctx, dc)

	assertSubscribersPresent(t, ctx, node, snapshotIMSIs, "on the converted seed")
	assertSubscribersPresent(t, ctx, node, afterSnapshotIMSIs, "on the converted seed")

	t.Setenv("ELLA_CORE_2_IMAGE", rollingTargetImage)
	t.Setenv("ELLA_CORE_3_IMAGE", rollingTargetImage)

	peers := []string{
		ClusterAddressWithPort(1, 7000),
		ClusterAddressWithPort(2, 7000),
		ClusterAddressWithPort(3, 7000),
	}

	clients := []*client.Client{node}

	for nodeNum := 2; nodeNum <= 3; nodeNum++ {
		if err := stageAndStartJoiner(ctx, dc, node, haRollingComposeDir, haNodeServices[nodeNum-1], nodeNum, peers, ""); err != nil {
			t.Fatalf("join node %d to the converted seed: %v", nodeNum, err)
		}

		c, err := newInsecureClient(APIAddressForCluster(nodeNum))
		if err != nil {
			t.Fatalf("client for node %d: %v", nodeNum, err)
		}

		c.SetToken(node.GetToken())
		clients = append(clients, c)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(t, context.Background(), dc, haRollingComposeDir, haNodeServices, clients)
	})

	if err := waitForClusterReady(ctx, clients); err != nil {
		t.Fatalf("cluster grown from the legacy seed never became ready: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("nodes not ready: %v", err)
	}

	for i, c := range clients {
		where := fmt.Sprintf("on node %d", i+1)
		assertSubscribersPresent(t, ctx, c, snapshotIMSIs, where)
		assertSubscribersPresent(t, ctx, c, afterSnapshotIMSIs, where)
	}

	leaderIdx, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	leaderStatus := mustReadStatus(ctx, t, leader)

	if _, err := leader.DrainClusterMember(ctx, leaderStatus.Cluster.NodeID); err != nil {
		t.Fatalf("drain the leader: %v", err)
	}

	survivors := make([]*client.Client, 0, len(clients)-1)

	for i, c := range clients {
		if i != leaderIdx {
			survivors = append(survivors, c)
		}
	}

	newLeader, err := waitForNewLeader(ctx, survivors)
	if err != nil {
		t.Fatalf("no new leader after draining node %d: %v", leaderIdx+1, err)
	}

	createSubscriberWhenSchemaCurrent(t, ctx, newLeader, "001019756189000")

	idx, err := leaderAppliedIndex(ctx, newLeader)
	if err != nil {
		t.Fatalf("new leader applied index: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, survivors, idx); err != nil {
		t.Fatalf("survivors did not converge after the leadership change: %v", err)
	}

	for _, c := range survivors {
		assertSubscribersPresent(t, ctx, c, snapshotIMSIs, "after the leadership change")
		assertSubscribersPresent(t, ctx, c, afterSnapshotIMSIs, "after the leadership change")
	}
}

func ensureLegacyChainImages(t *testing.T, ctx context.Context) {
	t.Helper()

	images := []string{rollingTargetImage}
	for _, tag := range legacyChain {
		images = append(images, legacyImage(tag))
	}

	for _, img := range images {
		if err := exec.CommandContext(ctx, "docker", "image", "inspect", img).Run(); err != nil {
			t.Fatalf("image %q not found; run LEGACY_CHAIN=1 integration/compose/ha-rolling/build-images.sh after building %s",
				img, rollingTargetImage)
		}
	}
}

func churnUntilSnapshot(t *testing.T, ctx context.Context, c *client.Client, container, scratch string) {
	t.Helper()

	const churnIMSI = "001019756189999"

	for written := 0; written < legacyChurnEntries; written += 2 {
		if err := c.CreateSubscriber(ctx, standaloneSubscriber(churnIMSI)); err != nil {
			t.Fatalf("churn create after %d entries: %v", written, err)
		}

		if err := c.DeleteSubscriber(ctx, &client.DeleteSubscriberOptions{ID: churnIMSI}); err != nil {
			t.Fatalf("churn delete after %d entries: %v", written, err)
		}
	}

	HALogf(t, "wrote %d log entries; waiting for raft's snapshot", legacyChurnEntries)

	if err := waitForSnapshotFile(ctx, container, scratch, legacySnapshotWait); err != nil {
		t.Fatalf("no snapshot after %d log entries: %v", legacyChurnEntries, err)
	}
}

func newestSnapshotSchema(t *testing.T, ctx context.Context, container, scratch string) int {
	t.Helper()

	dest, err := os.MkdirTemp(scratch, "snapshots-")
	if err != nil {
		t.Fatalf("mkdir scratch: %v", err)
	}

	cpCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if out, err := exec.CommandContext(cpCtx, "docker", "cp",
		container+":/data/raft/snapshots/.", dest).CombinedOutput(); err != nil {
		t.Fatalf("copy snapshot store: %v: %s", err, out)
	}

	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatalf("read snapshot store: %v", err)
	}

	type snapshot struct {
		name  string
		index uint64
	}

	var snaps []snapshot

	for _, e := range entries {
		parts := strings.Split(e.Name(), "-")
		if !e.IsDir() || len(parts) != 3 || strings.HasSuffix(e.Name(), ".tmp") {
			continue
		}

		index, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			continue
		}

		snaps = append(snaps, snapshot{name: e.Name(), index: index})
	}

	if len(snaps) == 0 {
		t.Fatal("snapshot store is empty")
	}

	sort.Slice(snaps, func(i, j int) bool { return snaps[i].index > snaps[j].index })

	f, err := os.Open(filepath.Join(dest, snaps[0].name, "state.bin"))
	if err != nil {
		t.Fatalf("open newest snapshot: %v", err)
	}

	defer func() { _ = f.Close() }()

	header := make([]byte, 16)
	if _, err := io.ReadFull(f, header); err != nil {
		t.Fatalf("read newest snapshot header: %v", err)
	}

	if string(header[:4]) != "ELSN" {
		t.Fatalf("newest snapshot %s has no ELSN header (%q)", snaps[0].name, header[:4])
	}

	return int(binary.BigEndian.Uint32(header[8:12]))
}

func assertSeedCheckpointed(t *testing.T, ctx context.Context, dc *DockerClient) {
	t.Helper()

	logs, err := dc.ComposeLogs(ctx, haRollingComposeDir, standaloneUpgradeService)
	if err != nil {
		t.Fatalf("read seed logs: %v", err)
	}

	if !strings.Contains(logs, "Raft: sole-server checkpoint complete") {
		t.Fatal("the converted seed never logged a completed sole-server checkpoint")
	}

	if strings.Contains(logs, "FSM: command failed in batch") {
		t.Fatal("the converted seed hit an FSM apply failure: the legacy log was replayed against the wrong schema")
	}
}
