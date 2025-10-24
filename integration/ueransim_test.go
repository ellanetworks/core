package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

func TestIntegrationUERANSIM(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	testCases := []struct {
		name string
		nat  bool
	}{
		// {
		// 	name: "Nat disabled",
		// 	nat:  false,
		// },
		{
			name: "Nat enabled",
			nat:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			dockerClient, err := NewDockerClient()
			if err != nil {
				t.Fatalf("failed to create docker client: %v", err)
			}
			defer dockerClient.Close()

			dockerClient.ComposeDown("compose/ueransim/")
			dockerClient.ComposeDown("compose/gnbsim/")

			err = dockerClient.ComposeUp("compose/ueransim/")
			if err != nil {
				t.Fatalf("failed to bring up compose: %v", err)
			}

			t.Log("deployed ella core")

			// err = configureRouter(t, ctx, dockerClient)
			// if err != nil {
			// 	t.Fatalf("failed to configure router: %v", err)
			// }

			// t.Log("configured router")

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

			err = configureEllaCore(ctx, ellaClient, tc.nat)
			if err != nil {
				t.Fatalf("failed to configure Ella Core: %v", err)
			}

			t.Log("configured Ella Core")

			ueransimContainerName, err := dockerClient.ResolveComposeContainer(ctx, "ueransim", "ueransim")
			if err != nil {
				t.Fatalf("failed to resolve ueransim container: %v", err)
			}

			_, err = dockerClient.Exec(ctx, ueransimContainerName, "bin/nr-gnb --config /gnb.yaml", true, 5*time.Second, logWriter{t})
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			t.Log("started pebble gnb")

			time.Sleep(3 * time.Second)

			_, err = dockerClient.Exec(ctx, ueransimContainerName, "bin/nr-ue --config /ue.yaml", true, 5*time.Second, logWriter{t})
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			t.Log("started pebble ue")

			time.Sleep(3 * time.Second)

			result, err := dockerClient.Exec(ctx, ueransimContainerName, "ip a", false, 5*time.Second, logWriter{t})
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			t.Logf("UERANSIM result: %s", result)

			if !strings.Contains(result, "uesimtun0") {
				t.Fatalf("expected 'uesimtun0' to be in the result, but it was not found")
			}

			t.Logf("Verified that 'uesimtun0' is in the result")

			ellaCoreContainerName, err := dockerClient.ResolveComposeContainer(ctx, "ueransim", "ella-core")
			if err != nil {
				t.Fatalf("failed to resolve ella core container: %v", err)
			}

			// nolint:godox TODO: this block is currently necessary to warm up the connectivity,
			// otherwise pings are lost. It should be removed once the issue is identified and fixed.
			_, err = dockerClient.Exec(ctx, ellaCoreContainerName, "ping 10.6.0.3 -c 1", false, 5*time.Second, logWriter{t})
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			result, err = dockerClient.Exec(ctx, ueransimContainerName, "ping -I uesimtun0 10.6.0.3 -c 3", false, 10*time.Second, logWriter{t})
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			t.Logf("UERANSIM ping result: %s", result)

			if !strings.Contains(result, "3 packets transmitted, 3 received") {
				t.Fatalf("expected '3 packets transmitted, 3 received' to be in the result, but it was not found")
			}

			if !strings.Contains(result, "0% packet loss") {
				t.Fatalf("expected '0 packet loss' to be in the result, but it was not found")
			}

			t.Logf("Verified that '3 packets transmitted, 3 received' and '0 packet loss' are in the result")

			err = dockerClient.CopyFileToContainer(ctx, ueransimContainerName, "network_test.py", "/network_test.py")
			if err != nil {
				t.Fatalf("failed to copy testing script: %v", err)
			}

			t.Logf("testing script copied")

			result, err = dockerClient.Exec(
				ctx,
				ueransimContainerName,
				"python3 /network_test.py --dev uesimtun0 --dest 10.6.0.3",
				false,
				5*time.Second,
				logWriter{t},
			)
			if err != nil {
				t.Fatalf("networking test suite failed: %v", err)
			}

			t.Logf("Network tester results: %s", result)

			dockerClient.ComposeDown("compose/ueransim/compose.yaml")
		})
	}
}
