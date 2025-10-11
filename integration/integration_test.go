package integration_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
	"gopkg.in/yaml.v2"
)

const (
	gnbsimNamespace              = "gnbsim"
	ueransimNamespace            = "ueransim"
	testPolicyName               = "tests"
	numIMSIS                     = 5
	testStartIMSI                = "001010100000001"
	testSubscriberKey            = "5122250214c33e723a5dd523fc145fc0"
	testSubscriberCustomOPc      = "981d464c7c52eb6e5036234984ad0bcf"
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

type ConfigureEllaCoreOpts struct {
	client    *client.Client
	customOPc bool
	nat       bool
}

func configureEllaCore(ctx context.Context, opts *ConfigureEllaCoreOpts) (*client.Subscriber, error) {
	initializeOpts := &client.InitializeOptions{
		Email:    "admin@ellanetworks.com",
		Password: "admin",
	}

	err := opts.client.Initialize(ctx, initializeOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %v", err)
	}

	err = opts.client.Refresh(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %v", err)
	}

	createAPITokenOpts := &client.CreateAPITokenOptions{
		Name:   "integration-test-token",
		Expiry: "",
	}
	resp, err := opts.client.CreateMyAPIToken(ctx, createAPITokenOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create API token: %v", err)
	}

	opts.client.SetToken(resp.Token)

	createDataNetworkOpts := &client.CreateDataNetworkOptions{
		Name:   "not-internet",
		IPPool: "172.250.0.0/24",
		DNS:    "8.8.8.8",
		Mtu:    1460,
	}
	err = opts.client.CreateDataNetwork(ctx, createDataNetworkOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create data network: %v", err)
	}

	err = opts.client.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{
		Enabled: opts.nat,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to disable NAT: %v", err)
	}

	createPolicyOpts := &client.CreatePolicyOptions{
		Name:            testPolicyName,
		BitrateUplink:   "200 Mbps",
		BitrateDownlink: "100 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkName: "not-internet",
	}
	err = opts.client.CreatePolicy(ctx, createPolicyOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy: %v", err)
	}

	updateOperatorIDOpts := &client.UpdateOperatorIDOptions{
		Mcc: "001",
		Mnc: "01",
	}
	err = opts.client.UpdateOperatorID(ctx, updateOperatorIDOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to update operator ID: %v", err)
	}

	updateOperatorSliceOpts := &client.UpdateOperatorSliceOptions{
		Sst: 1,
	}
	err = opts.client.UpdateOperatorSlice(ctx, updateOperatorSliceOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to update operator slice: %v", err)
	}

	updateOperatorTrackingOpts := &client.UpdateOperatorTrackingOptions{
		SupportedTacs: []string{"001"},
	}
	err = opts.client.UpdateOperatorTracking(ctx, updateOperatorTrackingOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to update operator tracking: %v", err)
	}

	for i := 0; i < numIMSIS; i++ {
		imsi, err := computeIMSI(testStartIMSI, i)
		if err != nil {
			return nil, fmt.Errorf("failed to compute IMSI: %v", err)
		}
		var opc string
		if opts.customOPc {
			opc = testSubscriberCustomOPc
		}
		createSubscriberOpts := &client.CreateSubscriberOptions{
			Imsi:           imsi,
			Key:            testSubscriberKey,
			SequenceNumber: testSubscriberSequenceNumber,
			PolicyName:     testPolicyName,
			OPc:            opc,
		}
		err = opts.client.CreateSubscriber(ctx, createSubscriberOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to create subscriber: %v", err)
		}
	}

	getSubscriberOpts := &client.GetSubscriberOptions{
		ID: testStartIMSI,
	}
	subscriber0, err := opts.client.GetSubscriber(ctx, getSubscriberOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber: %v", err)
	}
	return subscriber0, nil
}

func deploy(k *K8s) (string, error) {
	err := k.CreateNamespace()
	if err != nil {
		return "", fmt.Errorf("failed to create namespace %s: %v", gnbsimNamespace, err)
	}

	err = k.ApplyKustomize("../k8s/core/overlays/test")
	if err != nil {
		return "", fmt.Errorf("kubectl apply failed: %v", err)
	}

	err = k.ApplyKustomize("../k8s/router")
	if err != nil {
		return "", fmt.Errorf("kubectl apply failed: %v", err)
	}

	err = k.WaitForAppReady("ella-core")
	if err != nil {
		return "", fmt.Errorf("kubectl wait failed: %v", err)
	}

	err = k.WaitForAppReady("router")
	if err != nil {
		return "", fmt.Errorf("kubectl wait failed: %v", err)
	}

	nodePort, err := k.GetNodePort("ella-core")
	if err != nil {
		return "", fmt.Errorf("failed to get node port: %v", err)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", nodePort), nil
}

func patchGnbsimConfigmap(k *K8s, subscriber *client.Subscriber) error {
	configmapName := "gnbsim-config"
	configmap, err := k.GetConfigMap(configmapName)
	if err != nil {
		return fmt.Errorf("failed to get configmap %s: %v", configmapName, err)
	}

	data, ok := configmap["data"].(map[interface{}]interface{})
	if !ok {
		return fmt.Errorf("the 'data' field in the ConfigMap is not of the expected type")
	}
	configYamlStr, ok := data["configuration.yaml"].(string)
	if !ok {
		return fmt.Errorf("the 'configuration.yaml' key is missing in the ConfigMap data")
	}

	var config map[interface{}]interface{}
	err = yaml.Unmarshal([]byte(configYamlStr), &config)
	if err != nil {
		return fmt.Errorf("failed to unmarshal configmap data: %v", err)
	}
	configuration, ok := config["configuration"].(map[interface{}]interface{})
	if !ok {
		return fmt.Errorf("the 'configuration' key is missing or not a map in the config")
	}
	profiles, ok := configuration["profiles"].([]interface{})
	if !ok {
		return fmt.Errorf("the 'profiles' key is missing or not a list in the config")
	}
	for _, profile := range profiles {
		profileMap, ok := profile.(map[interface{}]interface{})
		if !ok {
			return fmt.Errorf("profile is not a valid map")
		}
		profileMap["startImsi"] = subscriber.Imsi
		profileMap["opc"] = subscriber.Opc
		profileMap["key"] = subscriber.Key
		profileMap["sequenceNumber"] = subscriber.SequenceNumber
	}
	// Create the updated YAML string
	updatedConfigYamlStr, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal updated config: %v", err)
	}
	// Construct a patch to update the 'configuration.yaml' field.
	// This patch uses a map with string keys.
	patch := map[string]interface{}{
		"data": map[string]interface{}{
			"configuration.yaml": string(updatedConfigYamlStr),
		},
	}
	err = k.PatchConfigMap(configmapName, patch)
	if err != nil {
		return fmt.Errorf("failed to patch configmap %s: %v", configmapName, err)
	}
	err = k.RolloutRestart("gnbsim")
	if err != nil {
		return fmt.Errorf("failed to rollout restart gnbsim: %v", err)
	}
	err = k.WaitForRollout("gnbsim")
	if err != nil {
		return fmt.Errorf("failed to wait for rollout gnbsim: %v", err)
	}
	return nil
}

func patchUERANSIMConfigmap(k *K8s, subscriber *client.Subscriber) error {
	configmapName := "ueransim-config"
	configmap, err := k.GetConfigMap(configmapName)
	if err != nil {
		return fmt.Errorf("failed to get configmap %s: %v", configmapName, err)
	}
	data, ok := configmap["data"].(map[interface{}]interface{})
	if !ok {
		return fmt.Errorf("the 'data' field in the ConfigMap is not of the expected type")
	}
	configYamlStr, ok := data["ue.yaml"].(string)
	if !ok {
		return fmt.Errorf("the 'ue.yaml' key is missing in the ConfigMap data")
	}
	var config map[interface{}]interface{}
	err = yaml.Unmarshal([]byte(configYamlStr), &config)
	if err != nil {
		return fmt.Errorf("failed to unmarshal configmap data: %v", err)
	}
	config["supi"] = fmt.Sprintf("imsi-%s", subscriber.Imsi)
	config["key"] = subscriber.Key
	config["op"] = subscriber.Opc
	// Create the updated YAML string
	updatedConfigYamlStr, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal updated config: %v", err)
	}
	// Construct a patch to update the 'ue.yaml' field.
	// This patch uses a map with string keys.
	patch := map[string]interface{}{
		"data": map[string]interface{}{
			"ue.yaml": string(updatedConfigYamlStr),
		},
	}
	err = k.PatchConfigMap(configmapName, patch)
	if err != nil {
		return fmt.Errorf("failed to patch configmap %s: %v", configmapName, err)
	}
	err = k.RolloutRestart("ueransim")
	if err != nil {
		return fmt.Errorf("failed to rollout restart ueransim: %v", err)
	}
	err = k.WaitForRollout("ueransim")
	if err != nil {
		return fmt.Errorf("failed to wait for rollout ueransim: %v", err)
	}
	return nil
}

func TestIntegrationGnbsim(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx := context.Background()

	k := &K8s{Namespace: gnbsimNamespace}

	ellaCoreURL, err := deploy(k)
	if err != nil {
		t.Fatalf("failed to deploy: %v", err)
	}
	t.Log("deployed ella core")

	clientConfig := &client.Config{
		BaseURL: ellaCoreURL,
	}
	ellaClient, err := client.New(clientConfig)
	if err != nil {
		t.Fatalf("failed to create ella client: %v", err)
	}

	configureOpts := &ConfigureEllaCoreOpts{
		client:    ellaClient,
		customOPc: true,
	}
	subscriber0, err := configureEllaCore(ctx, configureOpts)
	if err != nil {
		t.Fatalf("failed to configure Ella Core: %v", err)
	}
	t.Log("configured Ella Core")

	err = k.ApplyKustomize("../k8s/gnbsim")
	if err != nil {
		t.Fatalf("failed to apply kustomize: %v", err)
	}
	t.Log("applied kustomize for gnbsim")
	err = k.WaitForAppReady("gnbsim")
	if err != nil {
		t.Fatalf("failed to wait for gnbsim app to be ready: %v", err)
	}

	err = patchGnbsimConfigmap(k, subscriber0)
	if err != nil {
		t.Fatalf("failed to patch gnbsim configmap: %v", err)
	}
	t.Log("patched gnbsim configmap")

	err = k.WaitForAppReady("gnbsim")
	if err != nil {
		t.Fatalf("failed to wait for gnbsim app to be ready: %v", err)
	}
	t.Log("gnbsim app is ready")

	podName, err := k.GetPodName("gnbsim")
	if err != nil {
		t.Fatalf("failed to get pod name: %v", err)
	}

	t.Logf("Running GNBSim simulation in pod %s", podName)

	result, err := k.Exec(podName, "pebble exec gnbsim --cfg /etc/gnbsim/configuration.yaml", "gnbsim")
	if err != nil {
		t.Fatalf("failed to exec command in pod: %v", err)
	}
	t.Logf("GNBSim simulation result: %s", result)

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

	err = k.DeleteNamespace()
	if err != nil {
		t.Fatalf("failed to delete namespace %s: %v", gnbsimNamespace, err)
	}
	t.Logf("deleted namespace %s", gnbsimNamespace)
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

			k := &K8s{Namespace: ueransimNamespace}

			ellaCoreURL, err := deploy(k)
			if err != nil {
				t.Fatalf("failed to deploy: %v", err)
			}
			t.Log("deployed ella core")

			clientConfig := &client.Config{
				BaseURL: ellaCoreURL,
			}
			ellaClient, err := client.New(clientConfig)
			if err != nil {
				t.Fatalf("failed to create ella client: %v", err)
			}

			configureOpts := &ConfigureEllaCoreOpts{
				client:    ellaClient,
				customOPc: false,
				nat:       tc.nat,
			}
			subscriber0, err := configureEllaCore(ctx, configureOpts)
			if err != nil {
				t.Fatalf("failed to configure Ella Core: %v", err)
			}
			t.Log("configured Ella Core")

			err = k.ApplyKustomize("../k8s/ueransim")
			if err != nil {
				t.Fatalf("failed to apply kustomize: %v", err)
			}
			t.Log("applied kustomize for ueransim")
			err = k.WaitForAppReady("ueransim")
			if err != nil {
				t.Fatalf("failed to wait for ueransim app to be ready: %v", err)
			}

			err = patchUERANSIMConfigmap(k, subscriber0)
			if err != nil {
				t.Fatalf("failed to patch ueransim configmap: %v", err)
			}
			t.Log("patched ueransim configmap")

			ueransimPodName, err := k.GetPodName("ueransim")
			if err != nil {
				t.Fatalf("failed to get pod name: %v", err)
			}

			_, err = k.Exec(ueransimPodName, "pebble add gnb /etc/ueransim/pebble_gnb.yaml", "ueransim")
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			t.Log("added pebble gnb.yaml")

			_, err = k.Exec(ueransimPodName, "pebble start gnb", "ueransim")
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}
			t.Log("started pebble gnb")

			_, err = k.Exec(ueransimPodName, "pebble add ue /etc/ueransim/pebble_ue.yaml", "ueransim")
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}
			t.Log("added pebble ue.yaml")

			_, err = k.Exec(ueransimPodName, "pebble start ue", "ueransim")
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}
			t.Log("started pebble ue")

			result, err := k.Exec(ueransimPodName, "ip a", "ueransim")
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}
			t.Logf("UERANSIM result: %s", result)
			if !strings.Contains(result, "uesimtun0") {
				t.Fatalf("expected 'uesimtun0' to be in the result, but it was not found")
			}
			t.Logf("Verified that 'uesimtun0' is in the result")

			// nolint:godox TODO: this block is currently necessary to warm up the connectivity when NAT is enabled,
			// otherwise some pings are lost. It should be removed once the issue is identified and fixed.
			_, err = k.Exec(ueransimPodName, "ping -I uesimtun0 192.168.250.1 -c 10", "ueransim")
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			result, err = k.Exec(ueransimPodName, "ping -I uesimtun0 192.168.250.1 -c 3", "ueransim")
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

			result, err = k.Exec(
				ueransimPodName,
				"python3 /opt/network-tester/network_test.py --dev uesimtun0 --dest 192.168.250.1",
				"ueransim",
			)
			if err != nil {
				t.Fatalf("networking test suite failed: %v", err)
			}
			t.Logf("Network tester results: %s", result)

			err = k.DeleteNamespace()
			if err != nil {
				t.Fatalf("failed to delete namespace %s: %v", ueransimNamespace, err)
			}
			t.Logf("deleted namespace %s", ueransimNamespace)
			t.Log("UERANSIM test completed successfully")
		})
	}
}
