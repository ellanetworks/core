// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/ellanetworks/core/client"
)

// TestIntegration4GNetworkDetach attaches a real srsUE, then deletes its
// subscriber via the API. DeleteSubscriber drives a network-initiated detach
// (the 4G mirror of the AMF's deregistration on subscriber removal): the MME
// sends a Detach Request to the UE and releases its S1 context.
//
// Isolation: named TestIntegration4G* and gated on INTEGRATION.
func TestIntegration4GNetworkDetach(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() != IPv4Only {
		t.Skip("4G integration runs in IPv4 mode only")
	}

	ctx := context.Background()
	dockerClient, ellaClient := bring4GCoreUp(ctx, t, nil)

	defer dockerClient.ComposeCleanup(ctx)

	if err := dockerClient.ComposeUpServicesWithFile(ctx, "compose/srsenb/", ComposeFile(), "srsue"); err != nil {
		t.Fatalf("failed to start srsue: %v", err)
	}

	if !waitForAttach(ctx, t, dockerClient) {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsue", "srsenb")
		t.Fatal("UE did not reach EMM-REGISTERED")
	}

	t.Log("UE attached; deleting subscriber to trigger a network-initiated detach")

	if err := ellaClient.DeleteSubscriber(ctx, &client.DeleteSubscriberOptions{ID: "001010000000001"}); err != nil {
		t.Fatalf("failed to delete subscriber: %v", err)
	}

	if !waitForLog(ctx, t, dockerClient, "network-initiated detach") {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsenb")
		t.Fatal("MME did not send a network-initiated detach on subscriber deletion")
	}

	t.Log("subscriber deletion triggered a network-initiated detach")
}
