// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

const joinTokenComposeDir = "compose/ha-scaleup/"

const joinTokenProject = "ha-scaleup"

func TestIntegrationHAJoinTokenRejection(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	beginHATest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
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

	dc.ComposeCleanup(ctx)

	t.Cleanup(func() {
		dc.ComposeDownWithFile(context.Background(), joinTokenComposeDir, composeFile)
	})

	fullPeers := []string{
		ClusterAddressWithPort(1, 7000),
		ClusterAddressWithPort(2, 7000),
		ClusterAddressWithPort(3, 7000),
		ClusterAddressWithPort(4, 7000),
	}

	clients, err := bringUpHAClusterAt(t, ctx, dc, joinTokenComposeDir, haNodeServices,
		[]string{ClusterAddressWithPort(4, 7000)})
	if err != nil {
		t.Fatalf("bring up 3-node cluster: %v", err)
	}

	t.Cleanup(func() {
		diagCtx, diagCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer diagCancel()

		dumpClusterDiagnostics(t, diagCtx, dc, joinTokenComposeDir, haNodeServices, clients)
	})

	_, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("nodes not ready: %v", err)
	}

	valid, err := leader.MintClusterJoinToken(ctx, &client.MintJoinTokenOptions{
		NodeID:     4,
		TTLSeconds: 600,
	})
	if err != nil {
		t.Fatalf("mint token for node 4: %v", err)
	}

	forOtherNode, err := leader.MintClusterJoinToken(ctx, &client.MintJoinTokenOptions{
		NodeID:     5,
		TTLSeconds: 600,
	})
	if err != nil {
		t.Fatalf("mint token for node 5: %v", err)
	}

	cases := []struct {
		name         string
		token        string
		wantFragment string
	}{
		{
			name:         "tampered_hmac",
			token:        tamperJoinToken(t, valid.Token),
			wantFragment: "join token",
		},
		{
			name:         "minted_for_another_node",
			token:        forOtherNode.Token,
			wantFragment: "but this node is",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertJoinRejected(t, ctx, dc, leader, composeFile, tc.token, tc.wantFragment)
		})
	}

	HALog(t, "both bad tokens were rejected; confirming a valid token still admits node 4")

	if err := stageAndStartJoiner(ctx, dc, leader, joinTokenComposeDir,
		"ella-core-4", 4, fullPeers, "nonvoter"); err != nil {
		t.Fatalf("stage + start node 4 with a freshly minted token: %v", err)
	}

	if err := waitForMemberSuffrage(ctx, leader, 4, "nonvoter"); err != nil {
		t.Fatalf("node 4 did not join with a valid token, so the rejections above prove nothing: %v", err)
	}

	HALog(t, "valid token admitted node 4; join-token fence verified")
}

func assertJoinRejected(t *testing.T, ctx context.Context, dc *DockerClient, leader *client.Client, composeFile, token, wantFragment string) {
	t.Helper()

	if err := writeNodeConfig(joinTokenComposeDir, 4, []string{
		ClusterAddressWithPort(1, 7000),
		ClusterAddressWithPort(2, 7000),
		ClusterAddressWithPort(3, 7000),
		ClusterAddressWithPort(4, 7000),
	}, token, "nonvoter"); err != nil {
		t.Fatalf("write node 4 config: %v", err)
	}

	if err := dc.ComposeUpServicesWithFile(ctx, joinTokenComposeDir, composeFile, "ella-core-4"); err != nil {
		t.Fatalf("start node 4: %v", err)
	}

	t.Cleanup(func() {
		if err := composeRemoveService(context.Background(), joinTokenComposeDir, composeFile, "ella-core-4"); err != nil {
			HALogf(t, "cleanup node 4: %v", err)
		}
	})

	if err := dc.DisableRestart(ctx, joinTokenProject, "ella-core-4"); err != nil {
		HALogf(t, "disable restart on node 4: %v", err)
	}

	const window = 30 * time.Second

	deadline := time.Now().Add(window)

	for time.Now().Before(deadline) {
		members, err := leader.ListClusterMembers(ctx)
		if err == nil {
			for _, m := range members {
				if m.NodeID == 4 {
					t.Fatalf("node 4 joined the cluster with a rejected token (suffrage=%s)", m.Suffrage)
				}
			}
		}

		select {
		case <-ctx.Done():
			t.Fatalf("context ended while watching for a bad-token join: %v", ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}

	logs, err := dc.ComposeLogs(ctx, joinTokenComposeDir, "ella-core-4")
	if err != nil {
		t.Fatalf("collect node 4 logs: %v", err)
	}

	if !strings.Contains(strings.ToLower(logs), strings.ToLower(wantFragment)) {
		t.Errorf("node 4 stayed out of the cluster, but its logs never mention %q, so the rejection is unexplained; logs:\n%s",
			wantFragment, tailLines(logs, 40))
	}
}

func tamperJoinToken(t *testing.T, token string) string {
	t.Helper()

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("decode join token: %v", err)
	}

	if len(raw) == 0 {
		t.Fatal("join token decoded to zero bytes")
	}

	raw[len(raw)-1] ^= 0x01

	tampered := base64.RawURLEncoding.EncodeToString(raw)
	if tampered == token {
		t.Fatal("tampering did not change the join token")
	}

	return tampered
}

func composeRemoveService(ctx context.Context, composeDir, composeFile, service string) error {
	rm := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "rm", "-sf", service)
	rm.Dir = composeDir

	if out, err := rm.CombinedOutput(); err != nil {
		return fmt.Errorf("compose rm %s: %w (%s)", service, err, out)
	}

	vol := exec.CommandContext(ctx, "docker", "volume", "rm", "-f", joinTokenProject+"_data4")
	if out, err := vol.CombinedOutput(); err != nil {
		return fmt.Errorf("volume rm for %s: %w (%s)", service, err, out)
	}

	return nil
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return s
	}

	return strings.Join(lines[len(lines)-n:], "\n")
}
