// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/suites"
)

const haVRFComposeDir = "compose/ha-vrf/"

const haVRFComposeProject = "ha-vrf"

func assertVRFTopology(t *testing.T, ctx context.Context, dc *DockerClient) {
	t.Helper()

	want := map[string]string{"eth0": "cp-vrf", "n3": "up-vrf", "n6": "up-vrf"}

	for _, service := range haNodeServices {
		container, err := dc.ResolveComposeContainer(ctx, haVRFComposeProject, service)
		if err != nil {
			t.Fatalf("resolve container for %s: %v", service, err)
		}

		for iface, wantVRF := range want {
			got, err := vrfMasterOf(ctx, dc, container, iface)
			if err != nil {
				t.Fatalf("%s: read VRF master of %s: %v", service, iface, err)
			}

			if got != wantVRF {
				t.Fatalf("%s: %s is enslaved to %q, expected %q", service, iface, got, wantVRF)
			}
		}

		iface, err := mainTableDefaultIface(ctx, dc, container)
		if err != nil {
			t.Fatalf("%s: read main-table default route: %v", service, err)
		}

		if iface != "mgmt0" {
			t.Fatalf("%s: main-table default route is on %q, expected mgmt0", service, iface)
		}

		HALogf(t, "%s: eth0 in cp-vrf, n3+n6 in up-vrf, main-table default via mgmt0", service)
	}
}

func vrfMasterOf(ctx context.Context, dc *DockerClient, container, iface string) (string, error) {
	argv := []string{"/bin/busybox", "sh", "-c", "readlink /sys/class/net/" + iface + "/master || true"}

	out, err := dc.Exec(ctx, container, argv, false, 15*time.Second, nil)
	if err != nil {
		return "", err
	}

	out = strings.TrimSpace(out)
	if out == "" {
		return "", nil
	}

	return path.Base(out), nil
}

func mainTableDefaultIface(ctx context.Context, dc *DockerClient, container string) (string, error) {
	out, err := dc.Exec(ctx, container, []string{"/bin/busybox", "cat", "/proc/net/route"}, false, 15*time.Second, nil)
	if err != nil {
		return "", err
	}

	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[1] != "00000000" {
			continue
		}

		return fields[0], nil
	}

	return "", fmt.Errorf("no default route in table main:\n%s", out)
}

func TestIntegrationHAVRFClusterFormation(t *testing.T) {
	suites.RequireAll(t, suites.HAVRF)

	beginHATest(t)

	ctx := context.Background()

	dockerClient, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	defer func() {
		if err := dockerClient.Close(); err != nil {
			HALogf(t, "failed to close docker client: %v", err)
		}
	}()

	if err := buildVRFImage(ctx); err != nil {
		t.Fatalf("build ella-core-vrf image: %v", err)
	}

	HALog(t, "bringing up staged HA cluster with VRF-enslaved cluster interfaces")

	clients, err := bringUpHAClusterAt(t, ctx, dockerClient, haVRFComposeDir, haNodeServices, nil)
	if err != nil {
		t.Fatalf("bring up HA cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(t, ctx, dockerClient, haVRFComposeDir, haNodeServices, clients)
	})

	assertVRFTopology(t, ctx, dockerClient)

	HALog(t, "cluster is ready, verifying roles")

	leaderCount := 0
	followerCount := 0

	for i, c := range clients {
		status, err := c.GetStatus(ctx)
		if err != nil {
			t.Fatalf("failed to get status from node %d: %v", i+1, err)
		}

		switch status.Cluster.Role {
		case "Leader":
			leaderCount++
		case "Follower":
			followerCount++
		default:
			t.Fatalf("node %d has unexpected role %q", i+1, status.Cluster.Role)
		}
	}

	if leaderCount != 1 {
		t.Fatalf("expected 1 leader, got %d", leaderCount)
	}

	if followerCount != 2 {
		t.Fatalf("expected 2 followers, got %d", followerCount)
	}

	HALog(t, "roles verified: 1 leader, 2 followers")

	_, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("failed to find leader: %v", err)
	}

	HALog(t, "creating subscriber on leader")

	err = leader.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
		Imsi:           "001019756139935",
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	})
	if err != nil {
		t.Fatalf("failed to create subscriber on leader: %v", err)
	}

	HALog(t, "subscriber created, waiting for follower convergence")

	idx, err := leaderAppliedIndex(ctx, leader)
	if err != nil {
		t.Fatalf("failed to get leader applied index: %v", err)
	}

	err = waitForFollowerConvergence(ctx, clients, idx)
	if err != nil {
		t.Fatalf("followers did not converge: %v", err)
	}

	HALog(t, "followers converged, reading subscriber from each follower")

	for i, c := range clients {
		status, err := c.GetStatus(ctx)
		if err != nil {
			t.Fatalf("failed to get status from node %d: %v", i+1, err)
		}

		if status.Cluster.Role != "Follower" {
			continue
		}

		sub, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{
			ID: "001019756139935",
		})
		if err != nil {
			t.Fatalf("failed to read subscriber from follower node %d: %v", i+1, err)
		}

		if sub.Imsi != "001019756139935" {
			t.Fatalf("follower node %d returned subscriber with IMSI %q, expected %q",
				i+1, sub.Imsi, "001019756139935")
		}

		HALogf(t, "follower node %d returned subscriber correctly", i+1)
	}

	assertMembershipConsistent(t, ctx, clients)
}
