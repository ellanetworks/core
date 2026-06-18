// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestIntegration4GServiceRequest attaches a real srsUE, lets srsenb's inactivity
// timer move it to ECM-IDLE, then injects uplink traffic on the UE's TUN. srsUE
// emits a mobile-originated SERVICE REQUEST; the MME resolves it by S-TMSI,
// verifies the short MAC, and re-establishes the S1 context (ECM-IDLE →
// ECM-CONNECTED) — no Paging and no user plane required (the GW buffers the
// packet while the bearer re-establishes).
//
// Isolation: named TestIntegration4G* and gated on INTEGRATION.
func TestIntegration4GServiceRequest(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() != IPv4Only {
		t.Skip("4G integration runs in IPv4 mode only")
	}

	ctx := context.Background()

	// Short inactivity timer so srsenb releases the UE to ECM-IDLE within seconds.
	dockerClient, _ := bring4GCoreUp(ctx, t, map[string]string{"ENB_INACTIVITY_TIMER": "5000"})

	defer dockerClient.ComposeCleanup(ctx)

	if err := dockerClient.ComposeUpServicesWithFile(ctx, "compose/srsenb/", ComposeFile(), "srsue"); err != nil {
		t.Fatalf("failed to start srsue: %v", err)
	}

	if !waitForAttach(ctx, t, dockerClient) {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsue", "srsenb")
		t.Fatal("UE did not reach EMM-REGISTERED")
	}

	if !waitForLog(ctx, t, dockerClient, "UE moved to ECM-IDLE") {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsenb")
		t.Fatal("UE did not move to ECM-IDLE")
	}

	t.Log("UE idle; injecting uplink traffic to trigger a Service Request")

	srsue, err := dockerClient.ResolveComposeContainer(ctx, "srsenb", "srsue")
	if err != nil {
		t.Fatalf("failed to resolve srsue container: %v", err)
	}

	// An uplink packet egressing the UE's TUN makes srsUE emit a Service Request.
	// Send it via the TUN (the interface carrying the UE's 10.45.x address) to an
	// off-PDN address: with no user plane there are no replies — the side effect
	// (the uplink packet) is the point; ignore the exit status.
	_, _ = dockerClient.Exec(ctx, srsue,
		[]string{"sh", "-c", `DEV=$(ip -o -4 addr show | awk '/inet 10\.45\./{print $2; exit}'); ping -I "$DEV" -c 5 -W 1 8.8.8.8`},
		false, 15*time.Second, logWriter{t})

	if !waitForLog(ctx, t, dockerClient, "Service Request accepted") {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsue", "srsenb")
		t.Fatal("MME did not accept a Service Request after uplink traffic")
	}

	t.Log("UE reconnected via a mobile-originated Service Request")
}
