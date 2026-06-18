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

// TestIntegration4GS1Setup brings up Ella Core and a real srsRAN eNB
// (gradiant/srsran-4g) and verifies the eNB completes S1 Setup against the MME.
//
// The eNB appears in the radios API with ran_node_type "eNB" only after the MME
// has accepted its S1 Setup Request and sent an S1 Setup Response, so its
// presence is a positive proof of the request/response exchange.
//
// Isolation: named TestIntegration4G* so it runs on its own via
//
//	INTEGRATION=1 go test ./integration/... -run TestIntegration4G
//
// and is excluded from a full local run with `-skip TestIntegration4G`. Like the
// rest of the suite, it also gates on the INTEGRATION environment variable.
func TestIntegration4GS1Setup(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	// The srsenb compose stack is IPv4-only for now.
	if DetectIPFamily() != IPv4Only {
		t.Skip("4G S1 Setup integration runs in IPv4 mode only")
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

	// Start Ella Core first and wait until it is ready, then start srsenb, so
	// the eNB's S1 connection attempt lands after the MME's S1-MME listener is
	// up. srsenb attempts S1 once at startup and does not reliably retry a
	// refused connection, so ordering matters.
	if err := dockerClient.ComposeUpServicesWithFile(ctx, "compose/srsenb/", ComposeFile(), "ella-core"); err != nil {
		t.Fatalf("failed to start ella core: %v", err)
	}

	t.Log("deployed ella core")

	ellaClient, err := client.New(&client.Config{BaseURL: APIAddress()})
	if err != nil {
		t.Fatalf("failed to create ella client: %v", err)
	}

	if err := waitForEllaCoreReady(ctx, ellaClient); err != nil {
		t.Fatalf("failed to wait for ella core to be ready: %v", err)
	}

	// Minimal bootstrap: create the first user and an API token so the radios
	// API is queryable. No subscribers/NAT/routes are needed for S1 Setup.
	if err := ellaClient.Initialize(ctx, &client.InitializeOptions{
		Email:    "admin@ellanetworks.com",
		Password: "admin",
	}); err != nil {
		t.Fatalf("failed to initialize ella core: %v", err)
	}

	tok, err := ellaClient.CreateMyAPIToken(ctx, &client.CreateAPITokenOptions{Name: "4g-integration-token"})
	if err != nil {
		t.Fatalf("failed to create API token: %v", err)
	}

	ellaClient.SetToken(tok.Token)

	if err := dockerClient.ComposeUpServicesWithFile(ctx, "compose/srsenb/", ComposeFile(), "srsenb"); err != nil {
		t.Fatalf("failed to start srsenb: %v", err)
	}

	t.Log("ella core ready and srsenb started; waiting for the eNB to complete S1 Setup")

	if !waitForENB(ctx, t, ellaClient) {
		logs, logErr := dockerClient.ComposeLogs(ctx, "compose/srsenb/", "srsenb")
		if logErr != nil {
			t.Fatalf("eNB did not complete S1 Setup, and srsenb logs unavailable: %v", logErr)
		}

		t.Fatalf("eNB did not complete S1 Setup within the timeout.\nsrsenb logs:\n%s", logs)
	}

	t.Log("eNB completed S1 Setup")
}

// waitForENB polls the radios API until a radio with ran_node_type "eNB"
// appears, or the timeout elapses.
func waitForENB(ctx context.Context, t *testing.T, cl *client.Client) bool {
	t.Helper()

	deadline := time.Now().Add(90 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := cl.ListRadios(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err == nil {
			for _, radio := range resp.Items {
				if radio.RanNodeType == "eNB" {
					return true
				}
			}
		}

		time.Sleep(2 * time.Second)
	}

	return false
}
