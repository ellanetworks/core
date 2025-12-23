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
		{
			name: "Nat disabled",
			nat:  false,
		},
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

			dockerClient.ComposeDown(ctx, "compose/ueransim/")
			dockerClient.ComposeDown(ctx, "compose/core-tester/")

			err = dockerClient.ComposeUp(ctx, "compose/ueransim/")
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

			err = configureEllaCore(ctx, ellaClient, EllaCoreConfig{
				Networking: NetworkingConfig{
					NAT: tc.nat,
					Routes: []RouteConfig{
						{
							Destination: "8.8.8.8/32",
							Gateway:     "10.6.0.3",
							Interface:   "n6",
							Metric:      0,
						},
					},
				},
				Subscribers: []SubscriberConfig{
					{
						Imsi:           "001019756139935",
						Key:            "0eefb0893e6f1c2855a3a244c6db1277",
						OPc:            "98da19bbc55e2a5b53857d10557b1d26",
						SequenceNumber: "000000000022",
						PolicyName:     "default",
					},
				},
			})
			if err != nil {
				t.Fatalf("failed to configure Ella Core: %v", err)
			}

			t.Log("configured Ella Core")

			ueransimContainerName, err := dockerClient.ResolveComposeContainer(ctx, "ueransim", "ueransim")
			if err != nil {
				t.Fatalf("failed to resolve ueransim container: %v", err)
			}

			_, err = dockerClient.Exec(
				ctx, ueransimContainerName,
				[]string{
					"sh", "-c",
					`nohup bin/nr-gnb --config /gnb.yaml >/tmp/gnb.log 2>&1 </dev/null & echo $! >/tmp/gnb.pid`,
				},
				true,
				5*time.Second,
				logWriter{t},
			)
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			t.Log("started pebble gnb")

			if err := waitForPatternInContainer(ctx, dockerClient, ueransimContainerName,
				`/tmp/gnb.log`, `NG Setup procedure is successful`, 15*time.Second, 200*time.Millisecond); err != nil {
				t.Fatalf("Gnb never became ready: %v", err)
			}

			t.Log("GNB reported ready")

			_, err = dockerClient.Exec(
				ctx, ueransimContainerName,
				[]string{
					"sh", "-c",
					`nohup bin/nr-ue --config /ue.yaml >/tmp/ue.log 2>&1 </dev/null & echo $! >/tmp/ue.pid`,
				},
				true,
				5*time.Second,
				logWriter{t},
			)
			if err != nil {
				t.Fatalf("failed to start UE: %v", err)
			}

			t.Log("started UERANSIM UE")

			if err := waitForPatternInContainer(ctx, dockerClient, ueransimContainerName,
				`/tmp/ue.log`, `TUN interface\[uesimtun0, 10\.45\.0\.1\] is up`, 15*time.Second, 200*time.Millisecond); err != nil {
				t.Fatalf("UE never became ready: %v", err)
			}

			t.Log("UE reported ready")

			result, err := dockerClient.Exec(ctx, ueransimContainerName, []string{"ip", "a"}, false, 5*time.Second, logWriter{t})
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			t.Logf("UERANSIM result: %s", result)

			if !strings.Contains(result, "uesimtun0") {
				t.Fatalf("expected 'uesimtun0' to be in the result, but it was not found")
			}

			t.Logf("Verified that 'uesimtun0' is in the result")

			result, err = dockerClient.Exec(ctx, ueransimContainerName, []string{"ping", "-I", "uesimtun0", "10.6.0.3", "-c", "3"}, false, 10*time.Second, logWriter{t})
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
				[]string{"python3", "/network_test.py", "--dev", "uesimtun0", "--dest", "10.6.0.3"},
				false,
				5*time.Second,
				logWriter{t},
			)
			if err != nil {
				t.Fatalf("networking test suite failed: %v", err)
			}

			t.Logf("Network tester results: %s", result)

			dockerClient.ComposeDown(ctx, "compose/ueransim/compose.yaml")
		})
	}
}
