package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const (
	PolicyName      = "test-policy"
	BitrateUplink   = "100 Mbps"
	BitrateDownlink = "200 Mbps"
	Var5qi          = 9
	PriorityLevel   = 1
)

type CreatePolicyResponseResult struct {
	Message string `json:"message"`
}

type GetPolicyResponseResult struct {
	Name string `json:"name"`

	BitrateUplink   string `json:"bitrate-uplink,omitempty"`
	BitrateDownlink string `json:"bitrate-downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	PriorityLevel   int32  `json:"priority-level,omitempty"`
	DataNetworkName string `json:"data-network-name,omitempty"`
}

type GetPolicyResponse struct {
	Result GetPolicyResponseResult `json:"result"`
	Error  string                  `json:"error,omitempty"`
}

type CreatePolicyParams struct {
	Name string `json:"name"`

	BitrateUplink   string `json:"bitrate-uplink,omitempty"`
	BitrateDownlink string `json:"bitrate-downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	PriorityLevel   int32  `json:"priority-level,omitempty"`
	DataNetworkName string `json:"data-network-name,omitempty"`
}

type CreatePolicyResponse struct {
	Result CreatePolicyResponseResult `json:"result"`
	Error  string                     `json:"error,omitempty"`
}

type DeletePolicyResponseResult struct {
	Message string `json:"message"`
}

type DeletePolicyResponse struct {
	Result DeletePolicyResponseResult `json:"result"`
	Error  string                     `json:"error,omitempty"`
}

type ListPolicyResponse struct {
	Result []GetPolicyResponse `json:"result"`
	Error  string              `json:"error,omitempty"`
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
	req, err := http.NewRequest("GET", url+"/api/v1/policies/"+name, nil)
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

func editPolicy(url string, client *http.Client, name string, token string, data *CreatePolicyParams) (int, *CreatePolicyResponse, error) {
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
	req, err := http.NewRequest("DELETE", url+"/api/v1/policies/"+name, nil)
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
	ts, _, err := setupServer(dbPath, ReqsPerSec)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. List policies - 0", func(t *testing.T) {
		statusCode, response, err := listPolicies(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if len(response.Result) != 0 {
			t.Fatalf("expected 0 policies, got %d", len(response.Result))
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("2. Create data network", func(t *testing.T) {
		createDataNetworkParams := &CreateDataNetworkParams{
			Name:   DataNetworkName,
			MTU:    MTU,
			IPPool: IPPool,
			DNS:    DNS,
		}
		statusCode, response, err := createDataNetwork(ts.URL, client, token, createDataNetworkParams)
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

	t.Run("3. Create policy", func(t *testing.T) {
		createPolicyParams := &CreatePolicyParams{
			Name:            PolicyName,
			BitrateUplink:   "100 Mbps",
			BitrateDownlink: "200 Mbps",
			Var5qi:          9,
			PriorityLevel:   1,
			DataNetworkName: DataNetworkName,
		}
		statusCode, response, err := createPolicy(ts.URL, client, token, createPolicyParams)
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

	t.Run("4. List policies - 1", func(t *testing.T) {
		statusCode, response, err := listPolicies(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if len(response.Result) != 1 {
			t.Fatalf("expected 1 policy, got %d", len(response.Result))
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("5. Get policy", func(t *testing.T) {
		statusCode, response, err := getPolicy(ts.URL, client, token, PolicyName)
		if err != nil {
			t.Fatalf("couldn't get policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Name != PolicyName {
			t.Fatalf("expected name %s, got %s", PolicyName, response.Result.Name)
		}

		if response.Result.BitrateUplink != "100 Mbps" {
			t.Fatalf("expected bitrate-uplink 100 Mbps got %s", response.Result.BitrateUplink)
		}
		if response.Result.BitrateDownlink != "200 Mbps" {
			t.Fatalf("expected bitrate-downlink 200 Mbps got %s", response.Result.BitrateDownlink)
		}
		if response.Result.Var5qi != 9 {
			t.Fatalf("expected var5qi 9 got %d", response.Result.Var5qi)
		}
		if response.Result.PriorityLevel != 1 {
			t.Fatalf("expected priority-level 1 got %d", response.Result.PriorityLevel)
		}
		if response.Result.DataNetworkName != "internet" {
			t.Fatalf("expected data-network-name 'internet', got %s", response.Result.DataNetworkName)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("6. Get policy - id not found", func(t *testing.T) {
		statusCode, response, err := getPolicy(ts.URL, client, token, "policy-002")
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
		statusCode, response, err := createPolicy(ts.URL, client, token, createPolicyParams)
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
		createPolicyParams := &CreatePolicyParams{
			Name:            PolicyName,
			BitrateUplink:   "100 Mbps",
			BitrateDownlink: "200 Mbps",
			Var5qi:          2,
			PriorityLevel:   3,
			DataNetworkName: DataNetworkName,
		}
		statusCode, response, err := editPolicy(ts.URL, client, PolicyName, token, createPolicyParams)
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
			PolicyName:     PolicyName,
		}
		statusCode, response, err := createSubscriber(ts.URL, client, token, createSubscriberParams)
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

	t.Run("10. Delete policy - failure", func(t *testing.T) {
		statusCode, response, err := deletePolicy(ts.URL, client, token, PolicyName)
		if err != nil {
			t.Fatalf("couldn't delete policy: %s", err)
		}
		if statusCode != http.StatusConflict {
			t.Fatalf("expected status %d, got %d", http.StatusConflict, statusCode)
		}
		if response.Error != "Policy has subscribers" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("11. Delete subscriber", func(t *testing.T) {
		statusCode, response, err := deleteSubscriber(ts.URL, client, token, Imsi)
		if err != nil {
			t.Fatalf("couldn't edit policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("12. Delete policy - success", func(t *testing.T) {
		statusCode, response, err := deletePolicy(ts.URL, client, token, PolicyName)
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

	t.Run("13. Delete policy - no policy", func(t *testing.T) {
		statusCode, response, err := deletePolicy(ts.URL, client, token, PolicyName)
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

func TestCreatePolicyInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath, ReqsPerSec)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	tests := []struct {
		testName        string
		name            string
		bitrateUplink   string
		bitrateDownlink string
		var5qi          int32
		priorityLevel   int32
		DataNetworkName string
		error           string
	}{
		{
			testName:        "Invalid Name",
			name:            strings.Repeat("a", 257),
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			DataNetworkName: "internet",
			error:           "Invalid name format. Must be less than 256 characters",
		},

		{
			testName:        "Invalid Uplink Bitrate - Missing unit",
			name:            PolicyName,
			bitrateUplink:   "200",
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			DataNetworkName: "internet",
			error:           "Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Uplink Bitrate - Invalid unit",
			name:            PolicyName,
			bitrateUplink:   "200 Tbps",
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			DataNetworkName: "internet",
			error:           "Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Uplink Bitrate - Zero value",
			name:            PolicyName,
			bitrateUplink:   "0 Mbps",
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			DataNetworkName: "internet",
			error:           "Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Uplink Bitrate - Negative value",
			name:            PolicyName,
			bitrateUplink:   "-1 Mbps",
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			DataNetworkName: "internet",
			error:           "Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Uplink Bitrate - Too large value",
			name:            PolicyName,
			bitrateUplink:   "1001 Mbps",
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			DataNetworkName: "internet",
			error:           "Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Downlink Bitrate - Missing unit",
			name:            PolicyName,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: "200",
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			DataNetworkName: "internet",
			error:           "Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Downlink Bitrate - Invalid unit",
			name:            PolicyName,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: "200 Tbps",
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			DataNetworkName: "internet",
			error:           "Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Downlink Bitrate - Zero value",
			name:            PolicyName,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: "0 Mbps",
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			DataNetworkName: "internet",
			error:           "Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Downlink Bitrate - Negative value",
			name:            PolicyName,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: "-1 Mbps",
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			DataNetworkName: "internet",
			error:           "Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Downlink Bitrate - Too large value",
			name:            PolicyName,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: "1001 Mbps",
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			DataNetworkName: "internet",
			error:           "Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid 5QI - Too large value",
			name:            PolicyName,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: BitrateDownlink,
			var5qi:          256,
			priorityLevel:   PriorityLevel,
			DataNetworkName: "internet",
			error:           "Invalid Var5qi format. Must be an integer between 1 and 255",
		},
		{
			testName:        "Invalid Priority Level - Too large value",
			name:            PolicyName,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   256,
			DataNetworkName: "internet",
			error:           "Invalid priority-level format. Must be an integer between 1 and 255",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			createPolicyParams := &CreatePolicyParams{
				Name:            tt.name,
				BitrateUplink:   tt.bitrateUplink,
				BitrateDownlink: tt.bitrateDownlink,
				Var5qi:          tt.var5qi,
				PriorityLevel:   tt.priorityLevel,
				DataNetworkName: tt.DataNetworkName,
			}
			statusCode, response, err := createPolicy(ts.URL, client, token, createPolicyParams)
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
