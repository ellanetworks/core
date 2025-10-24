package integration_test

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

func deployGnbSim(ctx context.Context, dockerClient *DockerClient) error {
	err := dockerClient.CreateGnbsimContainer(ctx)
	if err != nil {
		return fmt.Errorf("failed to create gnbsim container: %v", err)
	}

	err = dockerClient.ConnectContainerToNetwork(ctx, "n3", "gnbsim", netip.MustParseAddr("10.3.0.3"), "n3")
	if err != nil {
		return fmt.Errorf("failed to connect gnbsim to n3: %v", err)
	}

	err = dockerClient.StartContainer(ctx, "gnbsim")
	if err != nil {
		return fmt.Errorf("failed to start gnbsim container: %v", err)
	}

	return nil
}

func deployEllaCoreWithN3(ctx context.Context, dockerClient *DockerClient) error {
	subnet := netip.PrefixFrom(netip.MustParseAddr("10.3.0.0"), 24)
	err := dockerClient.CreateNetwork(ctx, "n3", subnet)
	if err != nil {
		return fmt.Errorf("failed to create n3 network: %v", err)
	}

	err = dockerClient.CreateEllaCoreContainerWithConfig(ctx, "config_2_int.yaml")
	if err != nil {
		return fmt.Errorf("failed to create ella-core container: %v", err)
	}

	err = dockerClient.ConnectContainerToNetwork(ctx, "n3", "ella-core", netip.MustParseAddr("10.3.0.2"), "n3")
	if err != nil {
		return fmt.Errorf("failed to connect ella-core to n3: %v", err)
	}

	err = dockerClient.StartContainer(ctx, "ella-core")
	if err != nil {
		return fmt.Errorf("failed to start ella-core container: %v", err)
	}

	return nil
}

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

	dockerClient.CleanUpDockerSpace(ctx)

	err = deployEllaCoreWithN3(ctx, dockerClient)
	if err != nil {
		t.Fatalf("failed to deploy ella core with N3: %v", err)
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

	err = deployGnbSim(ctx, dockerClient)
	if err != nil {
		t.Fatalf("failed to deploy GNBSim: %v", err)
	}

	t.Log("deployed GNBSim")

	t.Log("running GNBSim simulation")

	out, err := dockerClient.Exec(ctx, "gnbsim", "gnbsim --cfg /config.yaml", false, 5*time.Minute, logWriter{t})
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
