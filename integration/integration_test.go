package integration_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
	"gopkg.in/yaml.v2"
)

const (
	gnbsimNamespace              = "gnbsim"
	testProfileName              = "default"
	numIMSIS                     = 5
	testStartIMSI                = "001010100000001"
	testSubscriberKey            = "5122250214c33e723a5dd523fc145fc0"
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

func configureEllaCore(baseUrl string) (*client.Subscriber, error) {
	clientConfig := &client.Config{
		BaseURL: baseUrl,
	}
	ellaClient, err := client.New(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ella client: %v", err)
	}

	createUserOpts := &client.CreateUserOptions{
		Email:    "admin@ellanetworks.com",
		Password: "admin",
		Role:     "admin",
	}
	err = ellaClient.CreateUser(createUserOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %v", err)
	}

	loginOpts := &client.LoginOptions{
		Email:    "admin@ellanetworks.com",
		Password: "admin",
	}
	err = ellaClient.Login(loginOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to login: %v", err)
	}

	createProfileOpts := &client.CreateProfileOptions{
		Name:            testProfileName,
		UeIPPool:        "172.250.0.0/24",
		DNS:             "8.8.8.8",
		Mtu:             1460,
		BitrateUplink:   "200 Mbps",
		BitrateDownlink: "100 Mbps",
		Var5qi:          8,
		PriorityLevel:   1,
	}
	err = ellaClient.CreateProfile(createProfileOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile: %v", err)
	}

	updateOperatorIDOpts := &client.UpdateOperatorIDOptions{
		Mcc: "001",
		Mnc: "01",
	}
	err = ellaClient.UpdateOperatorID(updateOperatorIDOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to update operator ID: %v", err)
	}

	updateOperatorSliceOpts := &client.UpdateOperatorSliceOptions{
		Sst: 1,
		Sd:  1056816,
	}
	err = ellaClient.UpdateOperatorSlice(updateOperatorSliceOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to update operator slice: %v", err)
	}

	updateOperatorTrackingOpts := &client.UpdateOperatorTrackingOptions{
		SupportedTacs: []string{"001"},
	}
	err = ellaClient.UpdateOperatorTracking(updateOperatorTrackingOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to update operator tracking: %v", err)
	}

	for i := 0; i < numIMSIS; i++ {
		imsi, err := computeIMSI(testStartIMSI, i)
		if err != nil {
			return nil, fmt.Errorf("failed to compute IMSI: %v", err)
		}
		createSubscriberOpts := &client.CreateSubscriberOptions{
			Imsi:           imsi,
			Key:            testSubscriberKey,
			SequenceNumber: testSubscriberSequenceNumber,
			ProfileName:    testProfileName,
		}
		err = ellaClient.CreateSubscriber(createSubscriberOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to create subscriber: %v", err)
		}
	}

	getSubscriberOpts := &client.GetSubscriberOptions{
		ID: testStartIMSI,
	}
	subscriber0, err := ellaClient.GetSubscriber(getSubscriberOpts)
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

	err = k.ApplyKustomize("../k8s/core/base")
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

	// Assert the "data" field as map[interface{}]interface{}
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
	// Update the necessary fields with values from the subscriber
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

func TestIntegrationGnbsim(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	k := &K8s{Namespace: gnbsimNamespace}

	ellaCoreURL, err := deploy(k)
	if err != nil {
		t.Fatalf("failed to deploy: %v", err)
	}
	t.Log("deployed ella core")

	subscriber0, err := configureEllaCore(ellaCoreURL)
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

	err = k.DeleteNamespace()
	if err != nil {
		t.Fatalf("failed to delete namespace %s: %v", gnbsimNamespace, err)
	}
	t.Logf("kubectl delete namespace succeeded: %s", gnbsimNamespace)
}
