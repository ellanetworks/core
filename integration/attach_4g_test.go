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

// TestIntegration4GAttach brings up Ella Core, a real srsRAN eNB, and a real
// srsRAN UE (all gradiant/srsran-4g over the ZMQ soft radio) and verifies the UE
// completes an EPS attach — authentication, NAS security mode, and default
// bearer — reaching EMM-REGISTERED. The user plane (GTP-U) is out of scope.
//
// The UE's USIM (compose/srsenb/compose.yaml) matches the MME's hard-coded
// subscriber (internal/mme/subscriber.go).
//
// Isolation: named TestIntegration4G* (see TestIntegration4GS1Setup) and gated
// on the INTEGRATION environment variable.
func TestIntegration4GAttach(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() != IPv4Only {
		t.Skip("4G attach integration runs in IPv4 mode only")
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

	t.Log("eNB completed S1 Setup; starting srsue")

	if err := dockerClient.ComposeUpServicesWithFile(ctx, "compose/srsenb/", ComposeFile(), "srsue"); err != nil {
		t.Fatalf("failed to start srsue: %v", err)
	}

	if !waitForAttach(ctx, t, dockerClient) {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsue", "srsenb")
		t.Fatal("UE did not reach EMM-REGISTERED within the timeout")
	}

	t.Log("UE completed EPS attach (EMM-REGISTERED)")
}

// waitForAttach polls Ella Core's logs for the MME's EMM-REGISTERED marker,
// which is logged only after the UE's Attach Complete.
func waitForAttach(ctx context.Context, t *testing.T, dc *DockerClient) bool {
	t.Helper()

	deadline := time.Now().Add(150 * time.Second)

	for time.Now().Before(deadline) {
		logs, err := dc.ComposeLogs(ctx, "compose/srsenb/", "ella-core")
		if err == nil && strings.Contains(logs, "EMM-REGISTERED") {
			return true
		}

		time.Sleep(3 * time.Second)
	}

	return false
}

func dumpLogs(ctx context.Context, t *testing.T, dc *DockerClient, services ...string) {
	t.Helper()

	for _, svc := range services {
		logs, err := dc.ComposeLogs(ctx, "compose/srsenb/", svc)
		if err != nil {
			t.Logf("%s logs unavailable: %v", svc, err)
			continue
		}

		t.Logf("=== %s logs ===\n%s", svc, logs)
	}
}
