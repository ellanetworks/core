// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

// TestIntegration4GDetach brings a real srsUE to EMM-REGISTERED, then stops it
// (SIGTERM → srsUE switch-off → Detach Request) and verifies the MME runs the
// detach: Detach Request → S1 UE Context Release with srsenb → context deletion.
func TestIntegration4GDetach(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() != IPv4Only {
		t.Skip("4G detach integration runs in IPv4 mode only")
	}

	ctx := context.Background()

	dockerClient, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	defer func() {
		if err := dockerClient.Close(); err != nil {
			t.Fatalf("failed to close docker client: %v", err)
		}
	}()

	dockerClient.ComposeCleanup(ctx)
	defer dockerClient.ComposeCleanup(ctx)

	if err := dockerClient.ComposeUpServicesWithFile(ctx, "compose/srsenb/", ComposeFile(), "ella-core"); err != nil {
		t.Fatalf("failed to start ella core: %v", err)
	}

	ellaClient, err := client.New(&client.Config{BaseURL: APIAddress()})
	if err != nil {
		t.Fatalf("failed to create ella client: %v", err)
	}

	if err := waitForEllaCoreReady(ctx, ellaClient); err != nil {
		t.Fatalf("failed to wait for ella core to be ready: %v", err)
	}

	provision4GCore(ctx, t, ellaClient)

	if err := dockerClient.ComposeUpServicesWithFile(ctx, "compose/srsenb/", ComposeFile(), "srsenb"); err != nil {
		t.Fatalf("failed to start srsenb: %v", err)
	}

	if !waitForENB(ctx, t, ellaClient) {
		t.Fatal("eNB did not complete S1 Setup")
	}

	if err := dockerClient.ComposeUpServicesWithFile(ctx, "compose/srsenb/", ComposeFile(), "srsue"); err != nil {
		t.Fatalf("failed to start srsue: %v", err)
	}

	if !waitForAttach(ctx, t, dockerClient) {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsue", "srsenb")
		t.Fatal("UE did not reach EMM-REGISTERED")
	}

	t.Log("UE attached; signalling srsue to trigger a switch-off detach")

	// `docker stop` sends SIGTERM to the entrypoint shell (PID 1), which does not
	// forward it; send SIGTERM straight to the srsue process so it runs
	// switch_off (the UE-initiated detach) before exiting.
	if _, err := dockerClient.Exec(ctx, "srsenb-srsue-1", []string{"pkill", "-TERM", "srsue"}, false, 30*time.Second, nil); err != nil {
		t.Fatalf("failed to signal srsue: %v", err)
	}

	if !waitForRelease(ctx, t, dockerClient) {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsenb")
		t.Fatal("MME did not release the UE context after detach")
	}

	t.Log("UE detached and S1 context released")
}

// waitForRelease polls Ella Core's logs for the MME's context-release marker.
func waitForRelease(ctx context.Context, t *testing.T, dc *DockerClient) bool {
	t.Helper()

	deadline := time.Now().Add(60 * time.Second)

	for time.Now().Before(deadline) {
		logs, err := dc.ComposeLogs(ctx, "compose/srsenb/", "ella-core")
		if err == nil && strings.Contains(logs, "UE context released") {
			return true
		}

		time.Sleep(3 * time.Second)
	}

	return false
}
