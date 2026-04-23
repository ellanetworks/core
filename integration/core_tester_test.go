package integration_test

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

func TestIntegrationEllaCoreTester(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ipFamily := DetectIPFamily()
	t.Logf("Running core-tester test in %s mode", ipFamily)

	ctx := context.Background()

	dockerClient, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	defer func() {
		err := dockerClient.Close()
		if err != nil {
			t.Fatalf("failed to close docker client: %v", err)
		}
	}()

	dockerClient.ComposeCleanup(ctx)

	composeFile := ComposeFile()

	err = dockerClient.ComposeUpWithFile(ctx, "compose/core-tester/", composeFile)
	if err != nil {
		t.Fatalf("failed to bring up compose with %s: %v", composeFile, err)
	}

	t.Cleanup(func() {
		logs, err := dockerClient.ComposeLogs(ctx, "compose/core-tester/", "ella-core")
		if err != nil {
			t.Logf("failed to collect ella-core logs: %v", err)
		} else {
			t.Logf("=== ella-core container logs ===\n%s", logs)
		}
	})

	t.Log("deployed ella core")

	clientConfig := &client.Config{
		BaseURL: APIAddress(),
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

	err = configureEllaCore(ctx, ellaClient, EllaCoreConfig{
		Networking: NetworkingConfig{
			NAT: true,
			Routes: []RouteConfig{
				{
					Destination: "8.8.8.8/32",
					Gateway:     N6Address(),
					Interface:   "n6",
					Metric:      0,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to configure Ella Core: %v", err)
	}

	t.Log("configured Ella Core")

	coreTesterContainerName, err := dockerClient.ResolveComposeContainer(ctx, "core-tester", "ella-core-tester")
	if err != nil {
		t.Fatalf("failed to resolve ella-core-tester container: %v", err)
	}

	t.Log("running Ella Core Tester `test` command")

	_, err = dockerClient.Exec(ctx, coreTesterContainerName, []string{
		"core-tester", "test",
		"--ella-core-api-address", APIAddress(),
		"--ella-core-api-token", ellaClient.GetToken(),
		"--ella-core-n2-address", net.JoinHostPort(N2Address(0), "38412"),
		"--gnb-n2-address", CoreTesterDefaultAddress(),
		"--gnb-n3-address", CoreTesterN3Address(),
		"--gnb-n3-address-secondary", CoreTesterN3AddressSecondary(),
		"--exclude", "ue/paging/downlink_data",
		"--verbose",
	}, false, 5*time.Minute, logWriter{t})
	if err != nil {
		t.Fatalf("failed to exec command in pod: %v", err)
	}
}
