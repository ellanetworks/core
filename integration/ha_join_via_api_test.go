// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/suites"
)

func writeUnprovisionedNodeConfig(composeDir string, nodeID int) error {
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
datapath:
  attach-mode: "xdp-generic"
cluster:
  bind-address: "%s:7000"
`, addr, addr, ClusterAddressWithBrackets(nodeID))

	return os.WriteFile(filepath.Join(cfgDir, "core.yaml"), []byte(body), 0o644)
}

func postClusterJoin(ctx context.Context, baseURL, token string, seeds []string) (int, string, error) {
	payload, err := json.Marshal(map[string]any{
		"token":         token,
		"seedAddresses": seeds,
	})
	if err != nil {
		return 0, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v1/cluster/join", bytes.NewReader(payload))
	if err != nil {
		return 0, "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 90 * time.Second}).Do(req)
	if err != nil {
		return 0, "", err
	}

	defer func() { _ = resp.Body.Close() }()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return resp.StatusCode, "", err
	}

	return resp.StatusCode, buf.String(), nil
}

func getJoinState(ctx context.Context, baseURL string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/cluster/join", nil)
	if err != nil {
		return "", "", err
	}

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "", "", err
	}

	defer func() { _ = resp.Body.Close() }()

	var envelope struct {
		Result struct {
			State string `json:"state"`
			Error string `json:"error"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return "", "", err
	}

	return envelope.Result.State, envelope.Result.Error, nil
}

func waitForJoinState(ctx context.Context, baseURL, want string) error {
	deadline := time.Now().Add(2 * time.Minute)

	var lastState, lastErr string

	for time.Now().Before(deadline) {
		state, joinErr, err := getJoinState(ctx, baseURL)
		if err == nil {
			lastState, lastErr = state, joinErr
			if state == want {
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("join state is %q (last error %q), want %q", lastState, lastErr, want)
}

func waitForAPIReachable(ctx context.Context, c *client.Client) error {
	deadline := time.Now().Add(2 * time.Minute)

	for time.Now().Before(deadline) {
		if _, err := c.GetStatus(ctx); err == nil {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("API never became reachable")
}

func TestIntegrationHAJoinViaAPI(t *testing.T) {
	suites.Require(t, suites.HA)

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

	dockerClient.ComposeCleanup(ctx)

	composeFile := ComposeFile()
	seeds := []string{ClusterAddressWithPort(1, 7000)}

	if err := writeNodeConfig(haComposeDir, 1, []string{ClusterAddressWithPort(1, 7000)}, "", ""); err != nil {
		t.Fatalf("write node 1 config: %v", err)
	}

	if err := dockerClient.ComposeUpServicesWithFile(ctx, haComposeDir, composeFile, haNodeServices[0]); err != nil {
		t.Fatalf("start node 1: %v", err)
	}

	node1, err := newInsecureClient(getHANodeURLs()[0])
	if err != nil {
		t.Fatalf("client for node 1: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(t, ctx, dockerClient, haComposeDir, haNodeServices[:2], []*client.Client{node1})
	})

	if err := foundCluster(ctx, getHANodeURLs()[0]); err != nil {
		t.Fatalf("found cluster on node 1: %v", err)
	}

	if err := waitForNodeReady(ctx, node1); err != nil {
		t.Fatalf("node 1 never became ready: %v", err)
	}

	adminToken, err := initializeAndGetAdminToken(ctx, node1)
	if err != nil {
		t.Fatalf("initialize node 1: %v", err)
	}

	node1.SetToken(adminToken)

	if err := writeUnprovisionedNodeConfig(haComposeDir, 2); err != nil {
		t.Fatalf("write node 2 config: %v", err)
	}

	if err := dockerClient.ComposeUpServicesWithFile(ctx, haComposeDir, composeFile, haNodeServices[1]); err != nil {
		t.Fatalf("start node 2: %v", err)
	}

	node2, err := newInsecureClient(getHANodeURLs()[1])
	if err != nil {
		t.Fatalf("client for node 2: %v", err)
	}

	if err := waitForAPIReachable(ctx, node2); err != nil {
		t.Fatalf("unprovisioned node 2 must still serve its API: %v", err)
	}

	status, err := node2.GetStatus(ctx)
	if err != nil {
		t.Fatalf("status of waiting node: %v", err)
	}

	if status.Ready {
		t.Fatal("a node with nothing to join must not report itself ready")
	}

	tok, err := node1.MintClusterJoinToken(ctx, &client.MintJoinTokenOptions{NodeID: 2, TTLSeconds: 600})
	if err != nil {
		t.Fatalf("mint join token: %v", err)
	}

	code, body, err := postClusterJoin(ctx, getHANodeURLs()[1], tok.Token, seeds)
	if err != nil {
		t.Fatalf("POST cluster join: %v", err)
	}

	if code != http.StatusAccepted {
		t.Fatalf("join request rejected: status %d body %s", code, body)
	}

	if err := waitForJoinState(ctx, getHANodeURLs()[1], "joined"); err != nil {
		t.Fatalf("join never completed: %v", err)
	}

	clients := []*client.Client{node1, node2}
	for _, c := range clients {
		c.SetToken(adminToken)
	}

	if err := waitForClusterReady(ctx, clients); err != nil {
		t.Fatalf("cluster not ready after API-driven join: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("nodes not ready: %v", err)
	}

	members, err := node1.ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("expected 2 members after the API-driven join, got %d", len(members))
	}

	code, body, err = postClusterJoin(ctx, getHANodeURLs()[1], tok.Token, seeds)
	if err != nil {
		t.Fatalf("second join request: %v", err)
	}

	if code != http.StatusConflict {
		t.Fatalf("a node that already joined must refuse further join requests: status %d body %s", code, body)
	}

	state, _, err := getJoinState(ctx, getHANodeURLs()[1])
	if err != nil {
		t.Fatalf("read join state: %v", err)
	}

	if state != "joined" {
		t.Fatalf("a joined node must keep reporting joined, got %q", state)
	}
}
