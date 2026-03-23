// Copyright 2026 Ella Networks

package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const (
	InitialDataNetworkName = "internet"
)

type CreateNetworkRuleRequest struct {
	Description  string `json:"description"`
	Direction    string `json:"direction"`
	RemotePrefix string `json:"remote_prefix"`
	Protocol     int32  `json:"protocol"`
	PortLow      int32  `json:"port_low"`
	PortHigh     int32  `json:"port_high"`
	Action       string `json:"action"`
	Precedence   int32  `json:"precedence"`
}

type NetworkRule struct {
	ID           int64  `json:"id"`
	PolicyID     int64  `json:"policy_id"`
	Description  string `json:"description"`
	Direction    string `json:"direction"`
	RemotePrefix string `json:"remote_prefix"`
	Protocol     int32  `json:"protocol"`
	PortLow      int32  `json:"port_low"`
	PortHigh     int32  `json:"port_high"`
	Action       string `json:"action"`
	Precedence   int32  `json:"precedence"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type CreateNetworkRuleResponse struct {
	Result struct {
		Message string `json:"message"`
		ID      int64  `json:"id"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

type GetNetworkRuleResponse struct {
	Result NetworkRule `json:"result"`
	Error  string      `json:"error,omitempty"`
}

type ListNetworkRulesResponse struct {
	Result struct {
		Items []NetworkRule `json:"items"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

type UpdateNetworkRuleResponse struct {
	Result struct {
		Message string `json:"message"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

type DeleteNetworkRuleResponse struct {
	Result struct {
		Message string `json:"message"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

func TestNetworkRulesCreateGetUpdateDelete(t *testing.T) {
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

	createPolicyParams := &CreatePolicyParams{
		Name:            "test-policy",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkName: InitialDataNetworkName,
	}

	statusCode, _, err := createPolicy(env.Server.URL, client, token, createPolicyParams)
	if err != nil {
		t.Fatalf("couldn't create policy: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", statusCode)
	}

	policyID := createPolicyParams.Name

	prefix := "10.0.0.0/24"
	ruleRequest := CreateNetworkRuleRequest{
		Description:  "test-rule",
		Direction:    "uplink",
		RemotePrefix: prefix,
		Protocol:     6,
		PortLow:      80,
		PortHigh:     443,
		Action:       "allow",
		Precedence:   1,
	}

	statusCode, createResp, err := createNetworkRule(env.Server.URL, client, token, policyID, &ruleRequest)
	if err != nil {
		t.Fatalf("Failed to create rule: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Error: %s", statusCode, createResp.Error)
	}

	ruleID := fmt.Sprintf("%d", createResp.Result.ID)

	statusCode, getResp, err := getNetworkRule(env.Server.URL, client, token, ruleID)
	if err != nil {
		t.Fatalf("Failed to get rule: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", statusCode)
	}

	if getResp.Result.Description != ruleRequest.Description {
		t.Fatalf("Expected description %s, got %s", ruleRequest.Description, getResp.Result.Description)
	}

	if getResp.Result.Direction != ruleRequest.Direction {
		t.Fatalf("Expected direction %s, got %s", ruleRequest.Direction, getResp.Result.Direction)
	}

	updateRequest := CreateNetworkRuleRequest{
		Description:  "test-rule",
		Direction:    "downlink",
		RemotePrefix: prefix,
		Protocol:     17,
		PortLow:      53,
		PortHigh:     53,
		Action:       "deny",
		Precedence:   2,
	}

	statusCode, updateResp, err := updateNetworkRule(env.Server.URL, client, token, ruleID, &updateRequest)
	if err != nil {
		t.Fatalf("Failed to update rule: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Error: %s", statusCode, updateResp.Error)
	}

	statusCode, getResp, err = getNetworkRule(env.Server.URL, client, token, ruleID)
	if err != nil {
		t.Fatalf("Failed to get updated rule: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("Expected status 200 after update, got %d", statusCode)
	}

	if getResp.Result.Direction != "downlink" {
		t.Fatalf("Expected direction downlink, got %s", getResp.Result.Direction)
	}

	if getResp.Result.Action != "deny" {
		t.Fatalf("Expected action deny, got %s", getResp.Result.Action)
	}

	statusCode, deleteResp, err := deleteNetworkRule(env.Server.URL, client, token, ruleID)
	if err != nil {
		t.Fatalf("Failed to delete rule: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Error: %s", statusCode, deleteResp.Error)
	}

	statusCode, _, err = getNetworkRule(env.Server.URL, client, token, ruleID)
	if err != nil {
		t.Fatalf("Failed to check deleted rule: %s", err)
	}

	if statusCode != http.StatusNotFound {
		t.Fatalf("Expected status 404 after delete, got %d", statusCode)
	}
}

func TestNetworkRulesListForPolicy(t *testing.T) {
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

	createPolicyParams := &CreatePolicyParams{
		Name:            "test-policy",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkName: InitialDataNetworkName,
	}

	statusCode, _, err := createPolicy(env.Server.URL, client, token, createPolicyParams)
	if err != nil {
		t.Fatalf("couldn't create policy: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", statusCode)
	}

	policyID := createPolicyParams.Name

	prefix := "10.0.0.0/24"
	for i := 1; i <= 3; i++ {
		ruleRequest := CreateNetworkRuleRequest{
			Description:  "rule-" + string(rune('0'+i)),
			Direction:    "uplink",
			RemotePrefix: prefix,
			Protocol:     6,
			PortLow:      80,
			PortHigh:     443,
			Action:       "allow",
			Precedence:   int32(i),
		}

		statusCode, createResp, err := createNetworkRule(env.Server.URL, client, token, policyID, &ruleRequest)
		if err != nil {
			t.Fatalf("Failed to create rule %d: %s", i, err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("Failed to create rule %d: expected status 201, got %d", i, statusCode)
		}

		_ = createResp
	}

	statusCode, listResp, err := listNetworkRulesForPolicy(env.Server.URL, client, token, policyID)
	if err != nil {
		t.Fatalf("Failed to list rules: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", statusCode)
	}

	if len(listResp.Result.Items) != 3 {
		t.Fatalf("Expected 3 rules, got %d", len(listResp.Result.Items))
	}

	var expectedPolicyID int64
	for i, rule := range listResp.Result.Items {
		if i == 0 {
			expectedPolicyID = rule.PolicyID
		} else if rule.PolicyID != expectedPolicyID {
			t.Fatalf("Expected all rules to have same policy_id, got %d and %d", expectedPolicyID, rule.PolicyID)
		}
	}
}

func TestNetworkRulesValidation(t *testing.T) {
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

	createPolicyParams := &CreatePolicyParams{
		Name:            "test-policy",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkName: InitialDataNetworkName,
	}

	statusCode, _, err := createPolicy(env.Server.URL, client, token, createPolicyParams)
	if err != nil {
		t.Fatalf("couldn't create policy: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", statusCode)
	}

	policyID := createPolicyParams.Name

	tests := []struct {
		name           string
		request        CreateNetworkRuleRequest
		expectedStatus int
	}{
		{
			name: "invalid CIDR",
			request: CreateNetworkRuleRequest{
				Description:  "rule",
				Direction:    "uplink",
				RemotePrefix: "invalid-cidr",
				Protocol:     6,
				PortLow:      80,
				PortHigh:     443,
				Action:       "allow",
				Precedence:   1,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "port_low > port_high",
			request: CreateNetworkRuleRequest{
				Description:  "rule",
				Direction:    "uplink",
				RemotePrefix: "10.0.0.0/24",
				Protocol:     6,
				PortLow:      443,
				PortHigh:     80,
				Action:       "allow",
				Precedence:   1,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid protocol",
			request: CreateNetworkRuleRequest{
				Description:  "rule",
				Direction:    "uplink",
				RemotePrefix: "10.0.0.0/24",
				Protocol:     256,
				PortLow:      80,
				PortHigh:     443,
				Action:       "allow",
				Precedence:   1,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid action",
			request: CreateNetworkRuleRequest{
				Description:  "rule",
				Direction:    "uplink",
				RemotePrefix: "10.0.0.0/24",
				Protocol:     6,
				PortLow:      80,
				PortHigh:     443,
				Action:       "invalid",
				Precedence:   1,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid direction",
			request: CreateNetworkRuleRequest{
				Description:  "rule",
				Direction:    "sideways",
				RemotePrefix: "10.0.0.0/24",
				Protocol:     6,
				PortLow:      80,
				PortHigh:     443,
				Action:       "allow",
				Precedence:   1,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid precedence",
			request: CreateNetworkRuleRequest{
				Description:  "rule",
				Direction:    "uplink",
				RemotePrefix: "10.0.0.0/24",
				Protocol:     6,
				PortLow:      80,
				PortHigh:     443,
				Action:       "allow",
				Precedence:   0,
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statusCode, _, err := createNetworkRule(env.Server.URL, client, token, policyID, &tt.request)
			if err != nil {
				t.Fatalf("Request failed: %s", err)
			}

			if statusCode != tt.expectedStatus {
				t.Fatalf("Expected status %d, got %d", tt.expectedStatus, statusCode)
			}
		})
	}
}

func TestNetworkRulesCannotChangePolicyID(t *testing.T) {
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

	createPolicyParams1 := &CreatePolicyParams{
		Name:            "test-policy-1",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkName: InitialDataNetworkName,
	}

	statusCode, _, err := createPolicy(env.Server.URL, client, token, createPolicyParams1)
	if err != nil {
		t.Fatalf("couldn't create policy 1: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", statusCode)
	}

	createPolicyParams2 := &CreatePolicyParams{
		Name:            "test-policy-2",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkName: InitialDataNetworkName,
	}

	statusCode, _, err = createPolicy(env.Server.URL, client, token, createPolicyParams2)
	if err != nil {
		t.Fatalf("couldn't create policy 2: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", statusCode)
	}

	policyID1 := createPolicyParams1.Name
	policyID2 := createPolicyParams2.Name

	prefix := "10.0.0.0/24"
	ruleRequest := CreateNetworkRuleRequest{
		Description:  "test-rule",
		Direction:    "uplink",
		RemotePrefix: prefix,
		Protocol:     6,
		PortLow:      80,
		PortHigh:     443,
		Action:       "allow",
		Precedence:   1,
	}

	statusCode, createResp, err := createNetworkRule(env.Server.URL, client, token, policyID1, &ruleRequest)
	if err != nil {
		t.Fatalf("Failed to create rule: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", statusCode)
	}

	ruleID := fmt.Sprintf("%d", createResp.Result.ID)

	type updateReqWithPolicyID struct {
		Name         string `json:"name"`
		Direction    string `json:"direction"`
		RemotePrefix string `json:"remote_prefix"`
		Protocol     int32  `json:"protocol"`
		PortLow      int32  `json:"port_low"`
		PortHigh     int32  `json:"port_high"`
		Action       string `json:"action"`
		Precedence   int32  `json:"precedence"`
		PolicyID     string `json:"policy_id"`
	}

	req := updateReqWithPolicyID{
		Name:         ruleRequest.Description,
		Direction:    ruleRequest.Direction,
		RemotePrefix: ruleRequest.RemotePrefix,
		Protocol:     ruleRequest.Protocol,
		PortLow:      ruleRequest.PortLow,
		PortHigh:     ruleRequest.PortHigh,
		Action:       ruleRequest.Action,
		Precedence:   ruleRequest.Precedence,
		PolicyID:     policyID2,
	}

	statusCode, err = updateNetworkRuleWithPolicy(env.Server.URL, client, token, ruleID, &req)
	if err != nil {
		t.Fatalf("Request failed: %s", err)
	}

	if statusCode != http.StatusBadRequest {
		t.Fatalf("Expected status 400 when trying to change policy_id, got %d", statusCode)
	}
}

func createNetworkRule(baseURL string, client *http.Client, token string, policyID string, rule *CreateNetworkRuleRequest) (int, *CreateNetworkRuleResponse, error) {
	body, err := json.Marshal(rule)
	if err != nil {
		return 0, nil, err
	}

	url := baseURL + "/api/v1/policies/" + policyID + "/rules"

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, strings.NewReader(string(body)))
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

	var resp CreateNetworkRuleResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

func getNetworkRule(baseURL string, client *http.Client, token string, ruleID string) (int, *GetNetworkRuleResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", baseURL+"/api/v1/network-rules/"+ruleID, nil)
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

	var resp GetNetworkRuleResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

func listNetworkRulesForPolicy(baseURL string, client *http.Client, token string, policyID string) (int, *ListNetworkRulesResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", baseURL+"/api/v1/policies/"+policyID+"/rules", nil)
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

	var resp ListNetworkRulesResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

func updateNetworkRule(baseURL string, client *http.Client, token string, ruleID string, rule *CreateNetworkRuleRequest) (int, *UpdateNetworkRuleResponse, error) {
	body, err := json.Marshal(rule)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "PUT", baseURL+"/api/v1/network-rules/"+ruleID, strings.NewReader(string(body)))
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

	var resp UpdateNetworkRuleResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

func updateNetworkRuleWithPolicy(baseURL string, client *http.Client, token string, ruleID string, rule any) (int, error) {
	body, err := json.Marshal(rule)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "PUT", baseURL+"/api/v1/network-rules/"+ruleID, strings.NewReader(string(body)))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	if err := res.Body.Close(); err != nil {
		panic(err)
	}

	return res.StatusCode, nil
}

func deleteNetworkRule(baseURL string, client *http.Client, token string, ruleID string) (int, *DeleteNetworkRuleResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", baseURL+"/api/v1/network-rules/"+ruleID, nil)
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

	var resp DeleteNetworkRuleResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}
