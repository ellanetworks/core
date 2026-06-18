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

// TestIntegration4GUnknownIMSI brings up srsUE with an IMSI that is not the
// MME's provisioned subscriber (a different MSISDN in the same PLMN) and checks
// the MME rejects the attach with ATTACH REJECT #2 ("IMSI unknown in HSS")
// rather than letting it register.
//
// Isolation: named TestIntegration4G* and gated on INTEGRATION.
func TestIntegration4GUnknownIMSI(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() != IPv4Only {
		t.Skip("4G integration runs in IPv4 mode only")
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

	// Start srsUE with a non-matching IMSI (different MSISDN, same PLMN so it
	// still camps on the cell).
	if err := dockerClient.ComposeRecreateService(ctx, "compose/srsenb/", ComposeFile(), "srsue",
		map[string]string{"UE_MSISDN": "0000000002"}); err != nil {
		t.Fatalf("failed to start srsue: %v", err)
	}

	if !waitForLog(ctx, t, dockerClient, "attach rejected: cannot authenticate subscriber") {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsue", "srsenb")
		t.Fatal("MME did not reject the unknown IMSI")
	}

	logs, err := dockerClient.ComposeLogs(ctx, "compose/srsenb/", "ella-core")
	if err == nil && strings.Contains(logs, "EMM-REGISTERED") {
		t.Fatal("unknown IMSI unexpectedly reached EMM-REGISTERED")
	}

	t.Log("unknown IMSI rejected with Attach Reject #2")
}

// TestIntegration4GAuthMACFailure brings up srsUE with a key that does not match
// the MME's hard-coded subscriber. The UE fails the AUTN MAC check and sends
// AUTHENTICATION FAILURE #20; the MME aborts with AUTHENTICATION REJECT
// (TS 24.301 §5.4.2.5) and the UE never registers.
func TestIntegration4GAuthMACFailure(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() != IPv4Only {
		t.Skip("4G integration runs in IPv4 mode only")
	}

	ctx := context.Background()
	dockerClient, _ := bring4GCoreUp(ctx, t, nil)

	defer dockerClient.ComposeCleanup(ctx)

	// Start srsUE with a wrong key (correct IMSI, so the IMSI check passes and the
	// failure occurs at authentication).
	if err := dockerClient.ComposeRecreateService(ctx, "compose/srsenb/", ComposeFile(), "srsue",
		map[string]string{"UE_KEY": "00112233445566778899aabbccddeeff"}); err != nil {
		t.Fatalf("failed to start srsue: %v", err)
	}

	if !waitForLog(ctx, t, dockerClient, "authentication rejected") {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsue", "srsenb")
		t.Fatal("MME did not reject authentication for the wrong key")
	}

	logs, err := dockerClient.ComposeLogs(ctx, "compose/srsenb/", "ella-core")
	if err == nil && strings.Contains(logs, "EMM-REGISTERED") {
		t.Fatal("wrong-key UE unexpectedly reached EMM-REGISTERED")
	}

	t.Log("wrong-key UE rejected with Authentication Reject")
}

// provision4GCore initializes Ella Core, sets an API token on cl, and creates
// the hard-coded srsUE subscriber (IMSI 001010000000001, TS 35.208 set-1
// credentials matching the compose USIM). The MME authenticates it from the DB.
func provision4GCore(ctx context.Context, t *testing.T, cl *client.Client) {
	t.Helper()

	if err := configureEllaCore(ctx, cl, EllaCoreConfig{
		Subscribers: []SubscriberConfig{{
			Imsi:           "001010000000001",
			Key:            "465b5ce8b199b49faa5f0a2ee238a6bc",
			OPc:            "cd63cb71954a9f4e48a5994e37a02baf",
			SequenceNumber: "000000000001",
			ProfileName:    "default",
		}},
	}); err != nil {
		t.Fatalf("failed to provision Ella Core: %v", err)
	}
}

// bring4GCoreUp starts ella-core and srsenb (through S1 Setup) and returns the
// docker client; the caller starts srsue with the env it needs. enbEnv, when
// non-nil, overrides srsenb environment (e.g. a short inactivity timer).
func bring4GCoreUp(ctx context.Context, t *testing.T, enbEnv map[string]string) (*DockerClient, *client.Client) {
	t.Helper()

	return bring4GCoreUpWithFile(ctx, t, ComposeFile(), enbEnv)
}

// bring4GCoreUpWithFile is bring4GCoreUp parameterized by compose file, letting
// the IPv6 user-plane test bring the stack up with the dual-stack N6 wiring
// (compose-dualstack.yaml) while the S1-U transport stays IPv4.
func bring4GCoreUpWithFile(ctx context.Context, t *testing.T, composeFile string, enbEnv map[string]string) (*DockerClient, *client.Client) {
	t.Helper()

	dockerClient, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	t.Cleanup(func() {
		if err := dockerClient.Close(); err != nil {
			t.Errorf("failed to close docker client: %v", err)
		}
	})

	dockerClient.ComposeCleanup(ctx)

	if err := dockerClient.ComposeUpServicesWithFile(ctx, "compose/srsenb/", composeFile, "ella-core"); err != nil {
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

	if enbEnv == nil {
		if err := dockerClient.ComposeUpServicesWithFile(ctx, "compose/srsenb/", composeFile, "srsenb"); err != nil {
			t.Fatalf("failed to start srsenb: %v", err)
		}
	} else if err := dockerClient.ComposeRecreateService(ctx, "compose/srsenb/", composeFile, "srsenb", enbEnv); err != nil {
		t.Fatalf("failed to start srsenb: %v", err)
	}

	if !waitForENB(ctx, t, ellaClient) {
		t.Fatal("eNB did not complete S1 Setup")
	}

	return dockerClient, ellaClient
}

// waitForLog polls the ella-core service's logs until marker appears or it times out.
func waitForLog(ctx context.Context, t *testing.T, dc *DockerClient, marker string) bool {
	t.Helper()

	deadline := time.Now().Add(90 * time.Second)

	for time.Now().Before(deadline) {
		logs, err := dc.ComposeLogs(ctx, "compose/srsenb/", "ella-core")
		if err == nil && strings.Contains(logs, marker) {
			return true
		}

		time.Sleep(3 * time.Second)
	}

	return false
}
