// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

func TestIntegrationHALeaderCrash(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	beginHATest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	defer func() {
		if err := dc.Close(); err != nil {
			HALogf(t, "failed to close docker client: %v", err)
		}
	}()

	composeFile := ComposeFile()

	clients, err := bringUpHACluster(t, ctx, dc)
	if err != nil {
		t.Fatalf("bring up HA cluster: %v", err)
	}

	t.Cleanup(func() {
		diagCtx, diagCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer diagCancel()

		dumpClusterDiagnostics(t, diagCtx, dc, haComposeDir, haNodeServices, clients)
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
		t.Fatalf("read leader status pre-crash: %v", err)
	}

	crashedNodeID := leaderStatus.Cluster.NodeID
	leaderService := haNodeServices[leaderIdx]

	if err := dc.DisableRestart(ctx, haComposeProject, leaderService); err != nil {
		t.Fatalf("disable restart on %s: %v", leaderService, err)
	}

	survivors := make([]*client.Client, 0, len(clients)-1)

	for i, c := range clients {
		if i != leaderIdx {
			survivors = append(survivors, c)
		}
	}

	writer := startSubscriberWriter(t, ctx, survivors, "001019756180000")

	writerStopped := false

	t.Cleanup(func() {
		if !writerStopped {
			writer.stop()
		}
	})

	time.Sleep(2 * time.Second)

	HALogf(t, "SIGKILLing leader %s (node %d)", leaderService, crashedNodeID)

	if err := composeKill(ctx, haComposeDir, composeFile, leaderService); err != nil {
		t.Fatalf("kill %s: %v", leaderService, err)
	}

	newLeader, err := waitForNewLeader(ctx, survivors)
	if err != nil {
		t.Fatalf("survivors did not elect a new leader after the crash: %v", err)
	}

	time.Sleep(2 * time.Second)

	report, werr := writer.stopAndReport()
	writerStopped = true

	if werr != nil {
		t.Fatalf("background writer reported a permanent failure: %v", werr)
	}

	HALogf(t, "writer: %d ok, %d transient, %d attempts, longest gap %s",
		report.success, report.transient, report.attempts, report.maxGap)

	if report.success < 2 {
		t.Fatalf("writer landed %d successful writes; too few to prove anything about the crash",
			report.success)
	}

	const crashWriteGapCeiling = 30 * time.Second

	if report.maxGap > crashWriteGapCeiling {
		t.Errorf("writes were unavailable for %s across the leader crash, ceiling is %s",
			report.maxGap, crashWriteGapCeiling)
	}

	newLeaderIdx, err := leaderAppliedIndex(ctx, newLeader)
	if err != nil {
		t.Fatalf("new leader applied index: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, survivors, newLeaderIdx); err != nil {
		t.Fatalf("survivors did not converge after the crash: %v", err)
	}

	HALog(t, "survivors converged; checking acknowledged writes survived on the majority side")

	assertAckedWritesDurable(t, ctx, survivors, report)

	HALogf(t, "restarting crashed node %s so it recovers from an uncleanly closed raft log", leaderService)

	if err := dc.ComposeStartWithFile(ctx, haComposeDir, composeFile, leaderService); err != nil {
		t.Fatalf("restart %s: %v", leaderService, err)
	}

	crashedClient := clients[leaderIdx]

	if err := waitForNodeReady(ctx, crashedClient); err != nil {
		t.Fatalf("crashed node did not become ready after restart: %v", err)
	}

	postRestartIdx, err := leaderAppliedIndex(ctx, newLeader)
	if err != nil {
		t.Fatalf("leader applied index after restart: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, []*client.Client{crashedClient}, postRestartIdx); err != nil {
		t.Fatalf("crashed node did not converge after restart: %v", err)
	}

	HALog(t, "crashed node recovered and converged; checking it carries every acknowledged write")

	assertAckedWritesDurable(t, ctx, clients, report)

	assertMembershipConsistent(t, ctx, clients)
}
