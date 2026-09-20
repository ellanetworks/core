// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/integration/suites"
)

// Reproduces the outage: an upgraded node must not apply v20 while peers
// are older, and must survive a lease-delete changeset replicated by an
// old leader.
func TestReproMixedSchemaLeaseDelete(t *testing.T) {
	suites.Require(t, suites.RollingUpgrade)

	beginHATest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	if err := ensureRollingImages(ctx, t); err != nil {
		t.Fatalf("rolling images: %v", err)
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

	clients, err := bringUpHAClusterMode(t, ctx, dc, haRollingComposeDir, haNodeServices, nil, true)
	if err != nil {
		t.Fatalf("bring up baseline cluster: %v", err)
	}

	leaderIdx, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	// Upgrade a follower, leaving the leader on the baseline image.
	upgradeIdx := (leaderIdx + 1) % len(clients)
	victimIdx := (leaderIdx + 2) % len(clients)

	swapNodeImage(t, ctx, dc, upgradeIdx+1, rollingTargetImage)

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("nodes not ready after upgrading node %d: %v", upgradeIdx+1, err)
	}

	upgraded, err := clients[upgradeIdx].GetStatus(ctx)
	if err != nil {
		t.Fatalf("status of upgraded node: %v", err)
	}

	t.Logf("upgraded node %d: appliedSchema=%d", upgradeIdx+1, upgraded.Cluster.AppliedSchemaVersion)

	if upgraded.Cluster.AppliedSchemaVersion >= 20 {
		t.Fatalf("upgraded node applied schema %d while peers are older; "+
			"that is the mixed-schema window that corrupts replicated changesets",
			upgraded.Cluster.AppliedSchemaVersion)
	}

	// Make the old leader replicate a DeleteDynamicLeasesByNode changeset,
	// the command that halted both nodes in the field.
	victimID, err := nodeIDOf(ctx, clients[victimIdx])
	if err != nil {
		t.Fatalf("resolve victim identity: %v", err)
	}

	if _, err := leader.DrainClusterMember(ctx, victimID); err != nil {
		t.Fatalf("drain node %s: %v", victimID, err)
	}

	if err := waitForDrained(ctx, leader, victimID); err != nil {
		t.Fatalf("node %s never drained: %v", victimID, err)
	}

	if err := leader.RemoveClusterMember(ctx, victimID, false); err != nil {
		t.Fatalf("remove node %s: %v", victimID, err)
	}

	t.Logf("removed node %s from the old leader; checking the upgraded node survived", victimID)

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := clients[upgradeIdx].GetStatus(ctx); err != nil {
			t.Fatalf("upgraded node stopped answering after the lease-delete replicated: %v", err)
		}

		time.Sleep(5 * time.Second)
	}

	t.Log("upgraded node stayed up across a lease-delete replicated by an old leader")
}
