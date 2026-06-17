// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

// TestIntegrationHARemoveLeader drains and removes the current leader,
// then asserts that the cluster shrinks to 2 members, stays writable
// throughout, and that the removed node is fenced from accepting
// writes against itself.
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
			HALogf(t, "failed to close docker client: %v", err)
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

	// The background writer cycles only over the surviving nodes: the
	// HA client retries on 503 but not on 502 (the removed-node fence),
	// so routing writes to the doomed leader would surface a permanent
	// error that operators would have steered around with their own
	// load-balancer.
	survivors := make([]*client.Client, 0, len(clients)-1)

	for i, c := range clients {
		if i != leaderIdx {
			survivors = append(survivors, c)
		}
	}

	writer := startSubscriberWriter(t, ctx, survivors, "001019756160000")

	const observationWindow = 3 * time.Second
	time.Sleep(observationWindow)

	drainResp, err := leader.DrainClusterMember(ctx, leaderNodeID)
	if err != nil {
		writer.stop()
		t.Fatalf("drain leader: %v", err)
	}

	if drainResp.DrainState != "drained" {
		writer.stop()
		t.Fatalf("drainState = %q, want drained", drainResp.DrainState)
	}

	newLeader, err := waitForNewLeader(ctx, survivors)
	if err != nil {
		writer.stop()
		t.Fatalf("no new leader after drain: %v", err)
	}

	if err := newLeader.RemoveClusterMember(ctx, leaderNodeID, false); err != nil {
		writer.stop()
		t.Fatalf("RemoveClusterMember(%d): %v", leaderNodeID, err)
	}

	if err := waitForMemberCount(ctx, newLeader, 2, 60*time.Second); err != nil {
		writer.stop()
		t.Fatalf("cluster did not shrink to 2 members: %v", err)
	}

	time.Sleep(observationWindow)

	report, werr := writer.stopAndReport()
	if werr != nil {
		t.Fatalf("background writer reported a permanent failure: %v", werr)
	}

	HALogf(t, "background writer: success=%d transient=%d attempts=%d",
		report.success, report.transient, report.attempts)

	const minAttempts = 20
	if report.attempts < minAttempts {
		t.Fatalf("writer attempts=%d (< %d); steady-state property not exercised",
			report.attempts, minAttempts)
	}

	if report.success == 0 {
		t.Fatal("zero successful writes; cluster was unwriteable")
	}

	if report.transient > report.attempts/2 {
		t.Fatalf("transient error rate too high: %d/%d", report.transient, report.attempts)
	}

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
	}

	if _, err := newLeader.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: fencedIMSI}); err == nil {
		t.Fatal("subscriber written via removed leader landed on the cluster; fence is broken")
	} else if !isExpectedNotFound(err) {
		HALogf(t, "unexpected error reading fenced IMSI: %v", err)
	}

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
		t.Fatalf("surviving follower did not converge: %v", err)
	}
}

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

	return fmt.Errorf("expected %d members, got %d after %s", want, last, timeout)
}

func isExpectedNotFound(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	return strings.Contains(msg, "not found") || strings.Contains(msg, "404")
}
