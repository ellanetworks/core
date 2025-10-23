package integration_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

const (
	testPolicyName               = "default"
	numIMSIS                     = 5
	testStartIMSI                = "001010100000001"
	testUERANSIMIMSI             = "001019756139935"
	testSubscriberKey            = "0eefb0893e6f1c2855a3a244c6db1277"
	testSubscriberCustomOPc      = "98da19bbc55e2a5b53857d10557b1d26"
	testSubscriberSequenceNumber = "000000000022"
	numProfiles                  = 5
)

func computeIMSI(baseIMSI string, increment int) (string, error) {
	intBaseImsi, err := strconv.Atoi(baseIMSI)
	if err != nil {
		return "", fmt.Errorf("failed to convert base IMSI to int: %v", err)
	}
	newIMSI := intBaseImsi + increment
	return fmt.Sprintf("%015d", newIMSI), nil
}

func configureEllaCore(ctx context.Context, cl *client.Client, nat bool) error {
	initializeOpts := &client.InitializeOptions{
		Email:    "admin@ellanetworks.com",
		Password: "admin",
	}

	err := cl.Initialize(ctx, initializeOpts)
	if err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}

	err = cl.Refresh(ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %v", err)
	}

	createAPITokenOpts := &client.CreateAPITokenOptions{
		Name:   "integration-test-token",
		Expiry: "",
	}
	resp, err := cl.CreateMyAPIToken(ctx, createAPITokenOpts)
	if err != nil {
		return fmt.Errorf("failed to create API token: %v", err)
	}

	cl.SetToken(resp.Token)

	err = cl.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{
		Enabled: nat,
	})
	if err != nil {
		return fmt.Errorf("failed to configure NAT: %v", err)
	}

	for i := range numIMSIS {
		imsi, err := computeIMSI(testStartIMSI, i)
		if err != nil {
			return fmt.Errorf("failed to compute IMSI: %v", err)
		}

		createSubscriberOpts := &client.CreateSubscriberOptions{
			Imsi:           imsi,
			Key:            testSubscriberKey,
			SequenceNumber: testSubscriberSequenceNumber,
			PolicyName:     testPolicyName,
			OPc:            testSubscriberCustomOPc,
		}
		err = cl.CreateSubscriber(ctx, createSubscriberOpts)
		if err != nil {
			return fmt.Errorf("failed to create subscriber: %v", err)
		}
	}

	createUEransimSubscriberOpts := &client.CreateSubscriberOptions{
		Imsi:           testUERANSIMIMSI,
		Key:            testSubscriberKey,
		SequenceNumber: testSubscriberSequenceNumber,
		PolicyName:     testPolicyName,
		OPc:            testSubscriberCustomOPc,
	}
	err = cl.CreateSubscriber(ctx, createUEransimSubscriberOpts)
	if err != nil {
		return fmt.Errorf("failed to create UERANSIM subscriber: %v", err)
	}

	return nil
}

func deployEllaCore() error {
	err := createN3Network()
	if err != nil {
		return fmt.Errorf("failed to create n3 network: %v", err)
	}

	err = createEllaCoreContainer()
	if err != nil {
		return fmt.Errorf("failed to create ella-core container: %v", err)
	}

	err = connectEllaCoreToN3()
	if err != nil {
		return fmt.Errorf("failed to connect ella-core to n3: %v", err)
	}

	err = startEllaCoreContainer()
	if err != nil {
		return fmt.Errorf("failed to start ella-core container: %v", err)
	}

	return nil
}

func deployGnbSim() error {
	err := createGnbsimContainer()
	if err != nil {
		return fmt.Errorf("failed to create gnbsim container: %v", err)
	}

	err = connectGnbsimToN3()
	if err != nil {
		return fmt.Errorf("failed to connect gnbsim to n3: %v", err)
	}

	err = startGnbsimContainer()
	if err != nil {
		return fmt.Errorf("failed to start gnbsim container: %v", err)
	}

	return nil
}

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

func waitForEllaCoreReady(ctx context.Context, cl *client.Client) error {
	timer := time.After(2 * time.Minute)

	for {
		select {
		case <-timer:
			return fmt.Errorf("timeout waiting for ella core to be ready")
		default:
			_, err := cl.GetStatus(ctx)
			if err != nil {
				time.Sleep(2 * time.Second)
				continue
			}
			return nil
		}
	}
}

func TestIntegrationGnbsim(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx := context.Background()

	cleanUpDockerSpace()

	err := deployEllaCore()
	if err != nil {
		t.Fatalf("failed to deploy: %v", err)
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

	err = deployGnbSim()
	if err != nil {
		t.Fatalf("failed to deploy GNBSim: %v", err)
	}

	t.Log("deployed GNBSim")

	t.Logf("Running GNBSim simulation in gnbsim container")

	result, err := dockerExec("gnbsim", "pebble exec gnbsim --cfg /config.yaml", false)
	if err != nil {
		t.Fatalf("failed to exec command in pod: %v", err)
	}

	passCount := strings.Count(result, "Profile Status: PASS")
	if passCount != numProfiles {
		t.Fatalf("expected 'Profile Status: PASS' to appear %d times, but found %d times", numProfiles, passCount)
	}
	t.Logf("Verified that 'Profile Status: PASS' appears %d times", passCount)

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

	cleanUpDockerSpace()

	t.Log("GNBSIM test completed successfully")
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

			err := deployEllaCore()
			if err != nil {
				t.Fatalf("failed to deploy: %v", err)
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

			time.Sleep(2 * time.Second)

			result, err = dockerExec("ueransim", "ping -I uesimtun0 8.8.8.8 -c 3", false)
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
				"python3 /network_test.py --dev uesimtun0 --dest 8.8.8.8",
				false,
			)
			if err != nil {
				t.Fatalf("networking test suite failed: %v", err)
			}

			t.Logf("Network tester results: %s", result)

			cleanUpDockerSpace()
		})
	}
}
