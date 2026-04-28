package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	PolicyName          = "test-policy"
	SessionAmbrUplink   = "100 Mbps"
	SessionAmbrDownlink = "200 Mbps"
	Var5qi              = 9
	Arp                 = 1
	TestProfileName     = "test-profile"
	DefaultSliceName    = "default"
	DefaultProfileName  = "default"
)

type CreatePolicyResponseResult struct {
	Message string `json:"message"`
}

type PolicyRule struct {
	Description  string  `json:"description"`
	RemotePrefix *string `json:"remote_prefix,omitempty"`
	Protocol     int32   `json:"protocol"`
	PortLow      int32   `json:"port_low"`
	PortHigh     int32   `json:"port_high"`
	Action       string  `json:"action"`
}

type PolicyRules struct {
	Uplink   []PolicyRule `json:"uplink,omitempty"`
	Downlink []PolicyRule `json:"downlink,omitempty"`
}

type Policy struct {
	Name                string       `json:"name"`
	ProfileName         string       `json:"profile_name,omitempty"`
	SliceName           string       `json:"slice_name,omitempty"`
	SessionAmbrUplink   string       `json:"session_ambr_uplink,omitempty"`
	SessionAmbrDownlink string       `json:"session_ambr_downlink,omitempty"`
	Var5qi              int32        `json:"var5qi,omitempty"`
	Arp                 int32        `json:"arp,omitempty"`
	DataNetworkName     string       `json:"data_network_name,omitempty"`
	Rules               *PolicyRules `json:"rules,omitempty"`
}

type GetPolicyResponse struct {
	Result Policy `json:"result"`
	Error  string `json:"error,omitempty"`
}

type CreatePolicyParams struct {
	Name                string       `json:"name"`
	ProfileName         string       `json:"profile_name"`
	SliceName           string       `json:"slice_name"`
	SessionAmbrUplink   string       `json:"session_ambr_uplink,omitempty"`
	SessionAmbrDownlink string       `json:"session_ambr_downlink,omitempty"`
	Var5qi              int32        `json:"var5qi,omitempty"`
	Arp                 int32        `json:"arp,omitempty"`
	DataNetworkName     string       `json:"data_network_name,omitempty"`
	Rules               *PolicyRules `json:"rules,omitempty"`
}

type CreatePolicyResponse struct {
	Result CreatePolicyResponseResult `json:"result"`
	Error  string                     `json:"error,omitempty"`
}

type UpdatePolicyParams struct {
	ProfileName         string       `json:"profile_name"`
	SliceName           string       `json:"slice_name"`
	SessionAmbrUplink   string       `json:"session_ambr_uplink,omitempty"`
	SessionAmbrDownlink string       `json:"session_ambr_downlink,omitempty"`
	Var5qi              int32        `json:"var5qi,omitempty"`
	Arp                 int32        `json:"arp,omitempty"`
	DataNetworkName     string       `json:"data_network_name,omitempty"`
	Rules               *PolicyRules `json:"rules,omitempty"`
}

type DeletePolicyResponseResult struct {
	Message string `json:"message"`
}

type DeletePolicyResponse struct {
	Result DeletePolicyResponseResult `json:"result"`
	Error  string                     `json:"error,omitempty"`
}

type ListPolicyResponseResult struct {
	Items      []Policy `json:"items"`
	Page       int      `json:"page"`
	PerPage    int      `json:"per_page"`
	TotalCount int      `json:"total_count"`
}

type ListPolicyResponse struct {
	Result ListPolicyResponseResult `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

func listPolicies(url string, client *http.Client, token string) (int, *ListPolicyResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/policies", nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var policyResponse ListPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&policyResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &policyResponse, nil
}

func getPolicy(url string, client *http.Client, token string, name string) (int, *GetPolicyResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/policies/"+name, nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var policyResponse GetPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&policyResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &policyResponse, nil
}

func createPolicy(url string, client *http.Client, token string, data *CreatePolicyParams) (int, *CreatePolicyResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/policies", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var createResponse CreatePolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &createResponse, nil
}

func editPolicy(url string, client *http.Client, name string, token string, data *UpdatePolicyParams) (int, *CreatePolicyResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/policies/"+name, strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var createResponse CreatePolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &createResponse, nil
}

func deletePolicy(url string, client *http.Client, token, name string) (int, *DeletePolicyResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", url+"/api/v1/policies/"+name, nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var deletePolicyResponse DeletePolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&deletePolicyResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &deletePolicyResponse, nil
}

// This is an end-to-end test for the policies handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestAPIPoliciesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. List policies - 1", func(t *testing.T) {
		statusCode, response, err := listPolicies(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list policy: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if len(response.Result.Items) != 1 {
			t.Fatalf("expected 1 policy, got %d", len(response.Result.Items))
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("2. Create new data network", func(t *testing.T) {
		createDataNetworkParams := &CreateDataNetworkParams{
			Name:   DataNetworkName,
			MTU:    MTU,
			IPPool: IPPool,
			DNS:    DNS,
		}

		statusCode, response, err := createDataNetwork(env.Server.URL, client, token, createDataNetworkParams)
		if err != nil {
			t.Fatalf("couldn't create subscriber: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("2b. Create profile for test policy", func(t *testing.T) {
		params := &CreateProfileParams{
			Name:           TestProfileName,
			UeAmbrUplink:   "100 Mbps",
			UeAmbrDownlink: "200 Mbps",
		}

		statusCode, response, err := createProfile(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't create profile: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("3. Create policy", func(t *testing.T) {
		createPolicyParams := &CreatePolicyParams{
			Name:                PolicyName,
			ProfileName:         TestProfileName,
			SliceName:           DefaultSliceName,
			SessionAmbrUplink:   "100 Mbps",
			SessionAmbrDownlink: "200 Mbps",
			Var5qi:              9,
			Arp:                 1,
			DataNetworkName:     DataNetworkName,
		}

		statusCode, response, err := createPolicy(env.Server.URL, client, token, createPolicyParams)
		if err != nil {
			t.Fatalf("couldn't create policy: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Message != "Policy created successfully" {
			t.Fatalf("expected message 'Policy created successfully', got %q", response.Result.Message)
		}
	})

	t.Run("4. List policies - 2", func(t *testing.T) {
		statusCode, response, err := listPolicies(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list policy: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if len(response.Result.Items) != 2 {
			t.Fatalf("expected 2 policy, got %d", len(response.Result.Items))
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("5. Get policy", func(t *testing.T) {
		statusCode, response, err := getPolicy(env.Server.URL, client, token, PolicyName)
		if err != nil {
			t.Fatalf("couldn't get policy: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.Name != PolicyName {
			t.Fatalf("expected name %s, got %s", PolicyName, response.Result.Name)
		}

		if response.Result.SessionAmbrUplink != "100 Mbps" {
			t.Fatalf("expected session_ambr_uplink 100 Mbps got %s", response.Result.SessionAmbrUplink)
		}

		if response.Result.SessionAmbrDownlink != "200 Mbps" {
			t.Fatalf("expected session_ambr_downlink 200 Mbps got %s", response.Result.SessionAmbrDownlink)
		}

		if response.Result.Var5qi != 9 {
			t.Fatalf("expected var5qi 9 got %d", response.Result.Var5qi)
		}

		if response.Result.Arp != 1 {
			t.Fatalf("expected arp 1 got %d", response.Result.Arp)
		}

		if response.Result.DataNetworkName != "not-internet" {
			t.Fatalf("expected data_network_name 'not-internet', got %s", response.Result.DataNetworkName)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("6. Get policy - id not found", func(t *testing.T) {
		statusCode, response, err := getPolicy(env.Server.URL, client, token, "policy-002")
		if err != nil {
			t.Fatalf("couldn't get policy: %s", err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}

		if response.Error != "Policy not found" {
			t.Fatalf("expected error %q, got %q", "Policy not found", response.Error)
		}
	})

	t.Run("7. Create policy - no name", func(t *testing.T) {
		createPolicyParams := &CreatePolicyParams{}

		statusCode, response, err := createPolicy(env.Server.URL, client, token, createPolicyParams)
		if err != nil {
			t.Fatalf("couldn't create policy: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if response.Error != "name is missing" {
			t.Fatalf("expected error %q, got %q", "name is missing", response.Error)
		}
	})

	t.Run("8. Edit policy - success", func(t *testing.T) {
		updatePolicyParams := &UpdatePolicyParams{
			ProfileName:         TestProfileName,
			SliceName:           DefaultSliceName,
			SessionAmbrUplink:   "100 Mbps",
			SessionAmbrDownlink: "200 Mbps",
			Var5qi:              6,
			Arp:                 3,
			DataNetworkName:     DataNetworkName,
		}

		statusCode, response, err := editPolicy(env.Server.URL, client, PolicyName, token, updatePolicyParams)
		if err != nil {
			t.Fatalf("couldn't edit policy: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("9. Add subscriber to policy", func(t *testing.T) {
		createSubscriberParams := &CreateSubscriberParams{
			Imsi:           Imsi,
			Key:            Key,
			SequenceNumber: SequenceNumber,
			ProfileName:    TestProfileName,
		}

		statusCode, response, err := createSubscriber(env.Server.URL, client, token, createSubscriberParams)
		if err != nil {
			t.Fatalf("couldn't edit policy: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("10. Delete policy - success", func(t *testing.T) {
		statusCode, response, err := deletePolicy(env.Server.URL, client, token, PolicyName)
		if err != nil {
			t.Fatalf("couldn't delete policy: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("11. Delete subscriber", func(t *testing.T) {
		statusCode, response, err := deleteSubscriber(env.Server.URL, client, token, Imsi)
		if err != nil {
			t.Fatalf("couldn't edit policy: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("12. Delete policy - no policy", func(t *testing.T) {
		statusCode, response, err := deletePolicy(env.Server.URL, client, token, PolicyName)
		if err != nil {
			t.Fatalf("couldn't delete policy: %s", err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}

		if response.Error != "Policy not found" {
			t.Fatalf("expected error %q, got %q", "Policy not found", response.Error)
		}
	})
}

// TestUpdatePolicyPathBodyMismatch verifies that the path name is used
// for the DB update instead of any name sent in the request body.
func TestUpdatePolicyPathBodyMismatch(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	// Create data network and policy.
	_, _, err = createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
		Name: DataNetworkName, MTU: MTU, IPPool: IPPool, DNS: DNS,
	})
	if err != nil {
		t.Fatalf("couldn't create data network: %s", err)
	}

	// Create a profile for the policy.
	_, _, err = createProfile(env.Server.URL, client, token, &CreateProfileParams{
		Name: "real-profile", UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps",
	})
	if err != nil {
		t.Fatalf("couldn't create profile: %s", err)
	}

	_, _, err = createPolicy(env.Server.URL, client, token, &CreatePolicyParams{
		Name: "real-policy", ProfileName: "real-profile", SliceName: DefaultSliceName,
		SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps",
		Var5qi: 9, Arp: 1, DataNetworkName: DataNetworkName,
	})
	if err != nil {
		t.Fatalf("couldn't create policy: %s", err)
	}

	// Update with a different name in the body than the path.
	updateParams := &UpdatePolicyParams{
		ProfileName:         "real-profile",
		SliceName:           DefaultSliceName,
		SessionAmbrUplink:   "50 Mbps",
		SessionAmbrDownlink: "50 Mbps",
		Var5qi:              6,
		Arp:                 2,
		DataNetworkName:     DataNetworkName,
	}

	statusCode, response, err := editPolicy(env.Server.URL, client, "real-policy", token, updateParams)
	if err != nil {
		t.Fatalf("couldn't edit policy: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d (error: %s)", http.StatusOK, statusCode, response.Error)
	}

	// Verify the real policy was updated (looked up by path name).
	getStatus, getResp, err := getPolicy(env.Server.URL, client, token, "real-policy")
	if err != nil {
		t.Fatalf("couldn't get policy: %s", err)
	}

	if getStatus != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, getStatus)
	}

	if getResp.Result.Arp != 2 {
		t.Fatalf("expected arp 2, got %d", getResp.Result.Arp)
	}

	// Verify the audit log references the path name, not the body name.
	_, auditResp, err := listAuditLogs(env.Server.URL, client, token, 1, 100)
	if err != nil {
		t.Fatalf("couldn't list audit logs: %s", err)
	}

	var found bool

	for _, entry := range auditResp.Result.Items {
		if entry.Action != "update_policy" {
			continue
		}

		found = true

		expected := "User updated policy: real-policy"
		if entry.Details != expected {
			t.Errorf("audit log records wrong name: got %q, want %q", entry.Details, expected)
		}

		break
	}

	if !found {
		t.Fatal("no update_policy audit entry found")
	}
}

func TestCreatePolicyInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	tests := []struct {
		testName            string
		name                string
		profileName         string
		sliceName           string
		sessionAmbrUplink   string
		sessionAmbrDownlink string
		var5qi              int32
		arp                 int32
		DataNetworkName     string
		error               string
	}{
		{
			testName:            "Invalid Name",
			name:                strings.Repeat("a", 257),
			profileName:         DefaultProfileName,
			sliceName:           DefaultSliceName,
			sessionAmbrUplink:   SessionAmbrUplink,
			sessionAmbrDownlink: SessionAmbrDownlink,
			var5qi:              Var5qi,
			arp:                 Arp,
			DataNetworkName:     "internet",
			error:               "invalid name format - must be less than 256 characters",
		},

		{
			testName:            "Invalid Uplink Bitrate - Missing unit",
			name:                PolicyName,
			profileName:         DefaultProfileName,
			sliceName:           DefaultSliceName,
			sessionAmbrUplink:   "200",
			sessionAmbrDownlink: SessionAmbrDownlink,
			var5qi:              Var5qi,
			arp:                 Arp,
			DataNetworkName:     "internet",
			error:               "invalid session_ambr_uplink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps",
		},
		{
			testName:            "Invalid Uplink Bitrate - Invalid unit",
			name:                PolicyName,
			profileName:         DefaultProfileName,
			sliceName:           DefaultSliceName,
			sessionAmbrUplink:   "200 Tbps",
			sessionAmbrDownlink: SessionAmbrDownlink,
			var5qi:              Var5qi,
			arp:                 Arp,
			DataNetworkName:     "internet",
			error:               "invalid session_ambr_uplink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps",
		},
		{
			testName:            "Invalid Uplink Bitrate - Zero value",
			name:                PolicyName,
			profileName:         DefaultProfileName,
			sliceName:           DefaultSliceName,
			sessionAmbrUplink:   "0 Mbps",
			sessionAmbrDownlink: SessionAmbrDownlink,
			var5qi:              Var5qi,
			arp:                 Arp,
			DataNetworkName:     "internet",
			error:               "invalid session_ambr_uplink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps",
		},
		{
			testName:            "Invalid Uplink Bitrate - Negative value",
			name:                PolicyName,
			profileName:         DefaultProfileName,
			sliceName:           DefaultSliceName,
			sessionAmbrUplink:   "-1 Mbps",
			sessionAmbrDownlink: SessionAmbrDownlink,
			var5qi:              Var5qi,
			arp:                 Arp,
			DataNetworkName:     "internet",
			error:               "invalid session_ambr_uplink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps",
		},
		{
			testName:            "Invalid Uplink Bitrate - Too large value",
			name:                PolicyName,
			profileName:         DefaultProfileName,
			sliceName:           DefaultSliceName,
			sessionAmbrUplink:   "1000001 Mbps",
			sessionAmbrDownlink: SessionAmbrDownlink,
			var5qi:              Var5qi,
			arp:                 Arp,
			DataNetworkName:     "internet",
			error:               "invalid session_ambr_uplink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps",
		},
		{
			testName:            "Invalid Downlink Bitrate - Missing unit",
			name:                PolicyName,
			profileName:         DefaultProfileName,
			sliceName:           DefaultSliceName,
			sessionAmbrUplink:   SessionAmbrUplink,
			sessionAmbrDownlink: "200",
			var5qi:              Var5qi,
			arp:                 Arp,
			DataNetworkName:     "internet",
			error:               "invalid session_ambr_downlink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps",
		},
		{
			testName:            "Invalid Downlink Bitrate - Invalid unit",
			name:                PolicyName,
			profileName:         DefaultProfileName,
			sliceName:           DefaultSliceName,
			sessionAmbrUplink:   SessionAmbrUplink,
			sessionAmbrDownlink: "200 Tbps",
			var5qi:              Var5qi,
			arp:                 Arp,
			DataNetworkName:     "internet",
			error:               "invalid session_ambr_downlink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps",
		},
		{
			testName:            "Invalid Downlink Bitrate - Zero value",
			name:                PolicyName,
			profileName:         DefaultProfileName,
			sliceName:           DefaultSliceName,
			sessionAmbrUplink:   SessionAmbrUplink,
			sessionAmbrDownlink: "0 Mbps",
			var5qi:              Var5qi,
			arp:                 Arp,
			DataNetworkName:     "internet",
			error:               "invalid session_ambr_downlink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps",
		},
		{
			testName:            "Invalid Downlink Bitrate - Negative value",
			name:                PolicyName,
			profileName:         DefaultProfileName,
			sliceName:           DefaultSliceName,
			sessionAmbrUplink:   SessionAmbrUplink,
			sessionAmbrDownlink: "-1 Mbps",
			var5qi:              Var5qi,
			arp:                 Arp,
			DataNetworkName:     "internet",
			error:               "invalid session_ambr_downlink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps",
		},
		{
			testName:            "Invalid Downlink Bitrate - Too large value",
			name:                PolicyName,
			profileName:         DefaultProfileName,
			sliceName:           DefaultSliceName,
			sessionAmbrUplink:   SessionAmbrUplink,
			sessionAmbrDownlink: "1000001 Mbps",
			var5qi:              Var5qi,
			arp:                 Arp,
			DataNetworkName:     "internet",
			error:               "invalid session_ambr_downlink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps",
		},
		{
			testName:            "Invalid 5QI - GBR",
			name:                PolicyName,
			profileName:         DefaultProfileName,
			sliceName:           DefaultSliceName,
			sessionAmbrUplink:   SessionAmbrUplink,
			sessionAmbrDownlink: SessionAmbrDownlink,
			var5qi:              1,
			arp:                 Arp,
			DataNetworkName:     "internet",
			error:               "invalid Var5qi format - must be an integer associated with a non-GBR 5QI",
		},
		{
			testName:            "Invalid 5QI - Delay Critical GBR",
			name:                PolicyName,
			profileName:         DefaultProfileName,
			sliceName:           DefaultSliceName,
			sessionAmbrUplink:   SessionAmbrUplink,
			sessionAmbrDownlink: SessionAmbrDownlink,
			var5qi:              82,
			arp:                 Arp,
			DataNetworkName:     "internet",
			error:               "invalid Var5qi format - must be an integer associated with a non-GBR 5QI",
		},
		{
			testName:            "Invalid Priority Level - Too large value",
			name:                PolicyName,
			profileName:         DefaultProfileName,
			sliceName:           DefaultSliceName,
			sessionAmbrUplink:   SessionAmbrUplink,
			sessionAmbrDownlink: SessionAmbrDownlink,
			var5qi:              Var5qi,
			arp:                 256,
			DataNetworkName:     "internet",
			error:               "invalid arp format - must be an integer between 1 and 15",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			createPolicyParams := &CreatePolicyParams{
				Name:                tt.name,
				ProfileName:         tt.profileName,
				SliceName:           tt.sliceName,
				SessionAmbrUplink:   tt.sessionAmbrUplink,
				SessionAmbrDownlink: tt.sessionAmbrDownlink,
				Var5qi:              tt.var5qi,
				Arp:                 tt.arp,
				DataNetworkName:     tt.DataNetworkName,
			}

			statusCode, response, err := createPolicy(env.Server.URL, client, token, createPolicyParams)
			if err != nil {
				t.Fatalf("couldn't create policy: %s", err)
			}

			if statusCode != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
			}

			if response.Error != tt.error {
				t.Fatalf("expected error %q, got %q", tt.error, response.Error)
			}
		})
	}
}

func TestCreateTooManyPoliciesPerProfile(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	// Create a profile to hold all policies.
	_, _, profErr := createProfile(env.Server.URL, client, token, &CreateProfileParams{
		Name:           "test-profile",
		UeAmbrUplink:   SessionAmbrUplink,
		UeAmbrDownlink: SessionAmbrDownlink,
	})
	if profErr != nil {
		t.Fatalf("couldn't create profile: %s", profErr)
	}

	// Create data networks so each policy can have a unique (profile, slice, dn) tuple.
	// The DB already has a default DN from migration, so we create 11 more (12 total).
	for i := 0; i < 11; i++ {
		dnName := "test-dn-" + strconv.Itoa(i)
		dnParams := &CreateDataNetworkParams{
			Name:   dnName,
			MTU:    1500,
			DNS:    "8.8.8.8",
			IPPool: fmt.Sprintf("10.%d.0.0/16", 50+i),
		}

		statusCode, _, dnErr := createDataNetwork(env.Server.URL, client, token, dnParams)
		if dnErr != nil {
			t.Fatalf("couldn't create data network %s: %s", dnName, dnErr)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d for DN creation, got %d", http.StatusCreated, statusCode)
		}
	}

	// Fill the profile with 12 policies (one per DN).
	for i := 0; i < 11; i++ {
		createPolicyParams := &CreatePolicyParams{
			Name:                PolicyName + strconv.Itoa(i),
			ProfileName:         "test-profile",
			SliceName:           DefaultSliceName,
			SessionAmbrUplink:   SessionAmbrUplink,
			SessionAmbrDownlink: SessionAmbrDownlink,
			Var5qi:              Var5qi,
			Arp:                 Arp,
			DataNetworkName:     "test-dn-" + strconv.Itoa(i),
		}

		statusCode, response, err := createPolicy(env.Server.URL, client, token, createPolicyParams)
		if err != nil {
			t.Fatalf("couldn't create policy: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error: %q", response.Error)
		}
	}

	// Create a 12th policy using a new slice to get a unique (profile, slice, dn) tuple.
	_, _, sliceErr := createSlice(env.Server.URL, client, token, &CreateSliceParams{
		Name: "extra-slice",
		Sst:  1,
		Sd:   "000002",
	})
	if sliceErr != nil {
		t.Fatalf("couldn't create extra slice: %s", sliceErr)
	}

	createPolicyParams := &CreatePolicyParams{
		Name:                PolicyName + "11",
		ProfileName:         "test-profile",
		SliceName:           "extra-slice",
		SessionAmbrUplink:   SessionAmbrUplink,
		SessionAmbrDownlink: SessionAmbrDownlink,
		Var5qi:              Var5qi,
		Arp:                 Arp,
		DataNetworkName:     "test-dn-0",
	}

	statusCode, response, err := createPolicy(env.Server.URL, client, token, createPolicyParams)
	if err != nil {
		t.Fatalf("couldn't create policy: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
	}

	if response.Error != "" {
		t.Fatalf("unexpected error: %q", response.Error)
	}

	// Now try the 13th policy on the same profile — should be rejected.
	excessParams := &CreatePolicyParams{
		Name:                "excess-policy",
		ProfileName:         "test-profile",
		SliceName:           "extra-slice",
		SessionAmbrUplink:   SessionAmbrUplink,
		SessionAmbrDownlink: SessionAmbrDownlink,
		Var5qi:              Var5qi,
		Arp:                 Arp,
		DataNetworkName:     "test-dn-1", // reuse DN; uniqueness doesn't matter — the limit check comes first
	}

	excessStatus, excessResp, excessErr := createPolicy(env.Server.URL, client, token, excessParams)
	if excessErr != nil {
		t.Fatalf("couldn't create policy: %s", excessErr)
	}

	if excessStatus != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, excessStatus)
	}

	expectedError := "Maximum number of policies per profile reached (12)"
	if excessResp.Error != expectedError {
		t.Fatalf("expected error %q, got %q", expectedError, excessResp.Error)
	}
}

// TestUpdatePolicyDeletesRulesWhenNotProvided verifies that omitting the rules
// field on an update request deletes all existing rules for the policy.
func TestUpdatePolicyDeletesRulesWhenNotProvided(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't initialize and login: %s", err)
	}

	// Create the data network first.
	_, _, err = createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
		Name: DataNetworkName, MTU: MTU, IPPool: IPPool, DNS: DNS,
	})
	if err != nil {
		t.Fatalf("couldn't create data network: %s", err)
	}

	// Create a profile for the policy.
	_, _, err = createProfile(env.Server.URL, client, token, &CreateProfileParams{
		Name: "rules-profile", UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps",
	})
	if err != nil {
		t.Fatalf("couldn't create profile: %s", err)
	}

	// Create a policy with rules.
	action := "allow"
	uplinkRule := PolicyRule{
		Description: "Allow HTTP",
		Protocol:    6,
		PortLow:     80,
		PortHigh:    80,
		Action:      action,
	}

	_, _, err = createPolicy(env.Server.URL, client, token, &CreatePolicyParams{
		Name:                "policy-with-rules",
		ProfileName:         "rules-profile",
		SliceName:           DefaultSliceName,
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "100 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkName:     DataNetworkName,
		Rules: &PolicyRules{
			Uplink: []PolicyRule{uplinkRule},
		},
	})
	if err != nil {
		t.Fatalf("couldn't create policy: %s", err)
	}

	// Verify the rules were created.
	_, getResp, err := getPolicy(env.Server.URL, client, token, "policy-with-rules")
	if err != nil {
		t.Fatalf("couldn't get policy: %s", err)
	}

	if getResp.Result.Rules == nil || len(getResp.Result.Rules.Uplink) == 0 {
		t.Fatal("expected uplink rules to exist after creation, got none")
	}

	// Update the policy without providing rules — this should delete them.
	statusCode, updateResp, err := editPolicy(env.Server.URL, client, "policy-with-rules", token, &UpdatePolicyParams{
		ProfileName:         "rules-profile",
		SliceName:           DefaultSliceName,
		SessionAmbrUplink:   "200 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkName:     DataNetworkName,
		// Rules intentionally omitted.
	})
	if err != nil {
		t.Fatalf("couldn't update policy: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d (error: %s)", http.StatusOK, statusCode, updateResp.Error)
	}

	// Verify the rules are now gone.
	_, getResp, err = getPolicy(env.Server.URL, client, token, "policy-with-rules")
	if err != nil {
		t.Fatalf("couldn't get policy after update: %s", err)
	}

	if getResp.Result.Rules != nil {
		t.Fatalf("expected rules to be deleted after update without rules, got: %+v", getResp.Result.Rules)
	}
}

func TestCreateMultiplePoliciesPerProfile(t *testing.T) {
	tempDir := t.TempDir()

	env, err := setupServer(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete setupServer: %s", err)
	}

	defer env.Server.Close()

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("Couldn't complete initializeAndRefresh: %s", err)
	}

	// Create a second data network for the second policy
	dn := &CreateDataNetworkParams{
		Name:   "second-dn",
		IPPool: "10.46.0.0/24",
		DNS:    "8.8.8.8",
		MTU:    1500,
	}

	statusCode, _, err := createDataNetwork(env.Server.URL, client, token, dn)
	if err != nil {
		t.Fatalf("Couldn't complete createDataNetwork: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d", http.StatusCreated, statusCode)
	}

	// The default profile already has the default policy.
	// Creating another policy for the same profile should now succeed.
	secondPolicy := &CreatePolicyParams{
		Name:                "second-policy-for-default",
		ProfileName:         DefaultProfileName,
		SliceName:           DefaultSliceName,
		SessionAmbrUplink:   "50 Mbps",
		SessionAmbrDownlink: "100 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkName:     "second-dn",
	}

	statusCode, resp, err := createPolicy(env.Server.URL, client, token, secondPolicy)
	if err != nil {
		t.Fatalf("Couldn't complete createPolicy: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusCreated, statusCode, resp.Error)
	}
}

// TestUpdatePolicyPersistsRules verifies UpdatePolicy commits rule
// changes to the database. Propagation of those rules to the local UPF
// is covered by the upf settings reconciler tests.
func TestUpdatePolicyPersistsRules(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't initialize and login: %s", err)
	}

	_, _, err = createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
		Name: DataNetworkName, MTU: MTU, IPPool: IPPool, DNS: DNS,
	})
	if err != nil {
		t.Fatalf("couldn't create data network: %s", err)
	}

	_, _, err = createProfile(env.Server.URL, client, token, &CreateProfileParams{
		Name: "update-rules-profile", UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps",
	})
	if err != nil {
		t.Fatalf("couldn't create profile: %s", err)
	}

	_, _, err = createPolicy(env.Server.URL, client, token, &CreatePolicyParams{
		Name:                "filter-test-policy",
		ProfileName:         "update-rules-profile",
		SliceName:           DefaultSliceName,
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "100 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkName:     DataNetworkName,
	})
	if err != nil {
		t.Fatalf("couldn't create policy: %s", err)
	}

	cidr := "10.0.0.0/8"
	uplinkRule := PolicyRule{
		Description:  "Allow traffic to 10.0.0.0/8",
		RemotePrefix: &cidr,
		Protocol:     6,
		PortLow:      443,
		PortHigh:     443,
		Action:       "allow",
	}

	downlinkRule := PolicyRule{
		Description:  "Allow traffic from 10.0.0.0/8",
		RemotePrefix: &cidr,
		Protocol:     6,
		PortLow:      443,
		PortHigh:     443,
		Action:       "deny",
	}

	statusCode, updateResp, err := editPolicy(env.Server.URL, client, "filter-test-policy", token, &UpdatePolicyParams{
		ProfileName:         "update-rules-profile",
		SliceName:           DefaultSliceName,
		SessionAmbrUplink:   "200 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkName:     DataNetworkName,
		Rules: &PolicyRules{
			Uplink:   []PolicyRule{uplinkRule},
			Downlink: []PolicyRule{downlinkRule},
		},
	})
	if err != nil {
		t.Fatalf("couldn't update policy: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d (error: %s)", http.StatusOK, statusCode, updateResp.Error)
	}

	_, getResp, err := getPolicy(env.Server.URL, client, token, "filter-test-policy")
	if err != nil {
		t.Fatalf("couldn't get policy after update: %s", err)
	}

	if getResp.Result.Rules == nil {
		t.Fatal("expected rules to exist after update, got none")
	}

	if len(getResp.Result.Rules.Uplink) != 1 {
		t.Fatalf("expected 1 uplink rule, got %d", len(getResp.Result.Rules.Uplink))
	}

	if len(getResp.Result.Rules.Downlink) != 1 {
		t.Fatalf("expected 1 downlink rule, got %d", len(getResp.Result.Rules.Downlink))
	}

	if getResp.Result.Rules.Uplink[0].Action != "allow" {
		t.Fatalf("expected uplink rule action 'allow', got %q", getResp.Result.Rules.Uplink[0].Action)
	}

	if getResp.Result.Rules.Downlink[0].Action != "deny" {
		t.Fatalf("expected downlink rule action 'deny', got %q", getResp.Result.Rules.Downlink[0].Action)
	}
}
