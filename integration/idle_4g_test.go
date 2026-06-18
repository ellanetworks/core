// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestIntegration4GIdle attaches a real srsUE, then lets srsenb's inactivity
// timer release the UE. The MME must move the UE to ECM-IDLE and retain its EMM
// context (it stays EMM-REGISTERED), rather than deleting it as on a detach.
func TestIntegration4GIdle(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() != IPv4Only {
		t.Skip("4G integration runs in IPv4 mode only")
	}

	ctx := context.Background()

	// Short inactivity timer so srsenb releases the idle UE within a few seconds.
	dockerClient, _ := bring4GCoreUp(ctx, t, map[string]string{"ENB_INACTIVITY_TIMER": "5000"})

	defer dockerClient.ComposeCleanup(ctx)

	if err := dockerClient.ComposeUpServicesWithFile(ctx, "compose/srsenb/", ComposeFile(), "srsue"); err != nil {
		t.Fatalf("failed to start srsue: %v", err)
	}

	if !waitForAttach(ctx, t, dockerClient) {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsue", "srsenb")
		t.Fatal("UE did not reach EMM-REGISTERED")
	}

	// With no traffic, srsenb's inactivity timer fires and releases the UE.
	if !waitForLog(ctx, t, dockerClient, "UE moved to ECM-IDLE") {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsenb")
		t.Fatal("MME did not move the UE to ECM-IDLE on the inactivity release")
	}

	// The EMM context must be retained, not deleted.
	logs, err := dockerClient.ComposeLogs(ctx, "compose/srsenb/", "ella-core")
	if err == nil && strings.Contains(logs, "UE context released") {
		t.Fatal("EMM context deleted on inactivity release; expected ECM-IDLE retention")
	}

	t.Log("UE moved to ECM-IDLE with its EMM context retained")
}
