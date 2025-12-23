package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

func TestIntegrationEllaCoreTester(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx := context.Background()

	dockerClient, err := NewDockerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}
	defer dockerClient.Close()

	dockerClient.ComposeDown(ctx, "compose/ueransim/")
	dockerClient.ComposeDown(ctx, "compose/core-tester/")

	err = dockerClient.ComposeUp(ctx, "compose/core-tester/")
	if err != nil {
		t.Fatalf("failed to bring up compose: %v", err)
	}

	t.Log("deployed ella core")

	clientConfig := &client.Config{
		BaseURL: "http://10.3.0.2:5002",
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
					Gateway:     "10.6.0.3",
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
		"--ella-core-api-address", "http://10.3.0.2:5002",
		"--ella-core-api-token", ellaClient.GetToken(),
		"--ella-core-n2-address", "10.3.0.2:38412",
		"--gnb-n2-address", "10.3.0.3",
		"--gnb-n3-address", "10.3.0.3",
		"--verbose",
	}, false, 5*time.Minute, logWriter{t})
	if err != nil {
		t.Fatalf("failed to exec command in pod: %v", err)
	}
}
