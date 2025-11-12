package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

func TestIntegrationGnbsim(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx := context.Background()

	dockerClient, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}
	defer dockerClient.Close()

	dockerClient.ComposeDown("compose/ueransim/")
	dockerClient.ComposeDown("compose/gnbsim/")
	dockerClient.ComposeDown("compose/core-tester/")

	err = dockerClient.ComposeUp("compose/gnbsim/")
	if err != nil {
		t.Fatalf("failed to bring up compose: %v", err)
	}

	t.Log("deployed ella core")

	clientConfig := &client.Config{
		BaseURL: "http://127.0.0.1:5002",
	}

	ellaClient, err := client.New(clientConfig)
	if err != nil {
		t.Fatalf("failed to create ella client: %v", err)
	}

	err = waitForEllaCoreReady(ctx, ellaClient)
	if err != nil {
		t.Fatalf("failed to wait for ella core to be ready: %v", err)
	}

	t.Log("ella core is ready")

	err = configureEllaCore(ctx, ellaClient, true)
	if err != nil {
		t.Fatalf("failed to configure Ella Core: %v", err)
	}

	t.Log("configured Ella Core")

	t.Log("running GNBSim simulation")

	gnbsimContainerName, err := dockerClient.ResolveComposeContainer(ctx, "gnbsim", "gnbsim")
	if err != nil {
		t.Fatalf("failed to resolve gnbsim container: %v", err)
	}

	out, err := dockerClient.Exec(ctx, gnbsimContainerName, []string{"gnbsim", "--cfg", "/config.yaml"}, false, 5*time.Minute, logWriter{t})
	if err != nil {
		t.Fatalf("failed to exec command in pod: %v", err)
	}

	t.Logf("gnbsim output:\n%s", out)

	passCount := strings.Count(out, "Profile Status: PASS")
	if passCount != numProfiles {
		t.Fatalf("expected 'Profile Status: PASS' %d times, found %d\nfull output:\n%s",
			numProfiles, passCount, out)
	}

	t.Logf("verified that 'Profile Status: PASS' appears %d times", passCount)

	metrics, err := ellaClient.GetMetrics(ctx)
	if err != nil {
		t.Fatalf("failed to get metrics: %v", err)
	}

	appUplinkBytes := metrics["app_uplink_bytes"]
	appDownlinkBytes := metrics["app_downlink_bytes"]

	if appUplinkBytes < 9000 {
		t.Fatalf("expected app_uplink_bytes to be at least 9000, but got %v", appUplinkBytes)
	}

	if appDownlinkBytes < 9000 {
		t.Fatalf("expected app_downlink_bytes to be at least 9000, but got %v", appDownlinkBytes)
	}
}
