package integration_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

func deployUeransim() error {
	err := createUeransimContainer()
	if err != nil {
		return fmt.Errorf("failed to create ueransim container: %v", err)
	}

	err = connectUeransimToN3()
	if err != nil {
		return fmt.Errorf("failed to connect ueransim to n3: %v", err)
	}

	err = startUeransimContainer()
	if err != nil {
		return fmt.Errorf("failed to start ueransim container: %v", err)
	}

	return nil
}

func deployRouter() error {
	err := createRouterContainer()
	if err != nil {
		return fmt.Errorf("failed to create router container: %v", err)
	}

	err = connectRouterToN6()
	if err != nil {
		return fmt.Errorf("failed to connect router to n6 network: %v", err)
	}

	err = startRouterContainer()
	if err != nil {
		return fmt.Errorf("failed to start router container: %v", err)
	}

	_, err = dockerExec(
		"router",
		"ip route add 10.45.0.0/16 via 10.6.0.2",
		false,
	)
	if err != nil {
		return fmt.Errorf("failed to enable ip forwarding in router: %v", err)
	}

	_, err = dockerExec(
		"router",
		"sysctl -w net.ipv4.ip_forward=1",
		false,
	)
	if err != nil {
		return fmt.Errorf("failed to enable ip forwarding in router: %v", err)
	}

	_, err = dockerExec(
		"router",
		"iptables-legacy -t nat -A POSTROUTING -o eth0 -j MASQUERADE",
		false,
	)
	if err != nil {
		return fmt.Errorf("failed to configure NAT in router: %v", err)
	}

	_, err = dockerExec(
		"router",
		"python3 /responder.py 34242",
		true,
	)
	if err != nil {
		return fmt.Errorf("failed to start responder in router: %v", err)
	}

	return nil
}

func deployEllaCoreWithN3AndN6() error {
	err := createN3Network()
	if err != nil {
		return fmt.Errorf("failed to create n3 network: %v", err)
	}

	err = createN6Network()
	if err != nil {
		return fmt.Errorf("failed to create n6 network: %v", err)
	}

	err = createEllaCoreContainerWithConfig("config_3_int.yaml")
	if err != nil {
		return fmt.Errorf("failed to create ella-core container: %v", err)
	}

	err = connectEllaCoreToN3()
	if err != nil {
		return fmt.Errorf("failed to connect ella-core to n3: %v", err)
	}

	err = connectEllaCoreToN6()
	if err != nil {
		return fmt.Errorf("failed to connect ella-core to n6: %v", err)
	}

	err = startEllaCoreContainer()
	if err != nil {
		return fmt.Errorf("failed to start ella-core container: %v", err)
	}

	return nil
}

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

			cleanUpDockerSpace()

			err := deployEllaCoreWithN3AndN6()
			if err != nil {
				t.Fatalf("failed to deploy ella core with N3 and N6: %v", err)
			}

			t.Log("deployed ella core")

			err = deployRouter()
			if err != nil {
				t.Fatalf("failed to deploy router: %v", err)
			}

			t.Log("deployed router")

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

			err = deployUeransim()
			if err != nil {
				t.Fatalf("failed to deploy UERANSIM: %v", err)
			}

			t.Log("deployed UERANSIM")

			_, err = dockerExec("ueransim", "bin/nr-gnb --config /gnb.yaml", true)
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			t.Log("started pebble gnb")

			time.Sleep(3 * time.Second)

			_, err = dockerExec("ueransim", "bin/nr-ue --config /ue.yaml", true)
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			t.Log("started pebble ue")

			time.Sleep(3 * time.Second)

			result, err := dockerExec("ueransim", "ip a", false)
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			t.Logf("UERANSIM result: %s", result)

			if !strings.Contains(result, "uesimtun0") {
				t.Fatalf("expected 'uesimtun0' to be in the result, but it was not found")
			}

			t.Logf("Verified that 'uesimtun0' is in the result")

			// nolint:godox TODO: this block is currently necessary to warm up the connectivity,
			// otherwise pings are lost. It should be removed once the issue is identified and fixed.
			_, err = dockerExec("ella-core", "ping 10.6.0.3 -c 1", false)
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			result, err = dockerExec("ueransim", "ping -I uesimtun0 10.6.0.3 -c 3", false)
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

			err = copyTestingScript()
			if err != nil {
				t.Fatalf("failed to copy testing script: %v", err)
			}

			t.Logf("testing script copied")

			result, err = dockerExec(
				"ueransim",
				"python3 /network_test.py --dev uesimtun0 --dest 10.6.0.3",
				false,
			)
			if err != nil {
				t.Fatalf("networking test suite failed: %v", err)
			}

			t.Logf("Network tester results: %s", result)
		})
	}
}
