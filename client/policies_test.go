// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestCreatePolicy_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Policy created successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	createPolicyOpts := &client.CreatePolicyOptions{
		Name:                "testPolicy",
		ProfileName:         "testProfile",
		SliceName:           "default",
		DataNetworkName:     "internet",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "100 Mbps",
		Var5qi:              9,
		Arp:                 1,
	}

	ctx := context.Background()

	err := clientObj.CreatePolicy(ctx, createPolicyOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestGetPolicy_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"name": "my-policy", "profile_name": "my-profile", "slice_name": "default", "data_network_name": "internet", "session_ambr_uplink": "100 Mbps", "session_ambr_downlink": "100 Mbps", "var5qi": 9, "arp": 1}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	name := "my-policy"

	getRouteOpts := &client.GetPolicyOptions{
		Name: name,
	}

	ctx := context.Background()

	policy, err := clientObj.GetPolicy(ctx, getRouteOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if policy.Name != name {
		t.Fatalf("expected ID %v, got %v", name, policy.Name)
	}
}

func TestDeletePolicy_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Policy deleted successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	name := "testPolicy"

	deletePolicyOpts := &client.DeletePolicyOptions{
		Name: name,
	}

	ctx := context.Background()

	err := clientObj.DeletePolicy(ctx, deletePolicyOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestListPolicies_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"items": [{"name": "policy1"}, {"name": "policy2"}], "page": 1, "per_page": 10, "total_count": 2}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	params := &client.ListParams{
		Page:    1,
		PerPage: 10,
	}

	policies, err := clientObj.ListPolicies(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(policies.Items) != 2 {
		t.Fatalf("expected 2 policies, got: %d", len(policies.Items))
	}
}

func TestCreatePolicy_WithRules_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Policy created successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	remotePrefix := "10.0.0.0/24"
	rules := &client.PolicyRules{
		Uplink: []client.PolicyRule{
			{
				Description:  "Allow HTTP/HTTPS",
				RemotePrefix: &remotePrefix,
				Protocol:     6,
				PortLow:      80,
				PortHigh:     443,
				Action:       "allow",
			},
		},
		Downlink: []client.PolicyRule{
			{
				Description:  "Allow DNS",
				RemotePrefix: nil,
				Protocol:     17,
				PortLow:      53,
				PortHigh:     53,
				Action:       "allow",
			},
		},
	}

	createPolicyOpts := &client.CreatePolicyOptions{
		Name:                "policy-with-rules",
		ProfileName:         "policy-with-rules",
		SliceName:           "default",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkName:     "internet",
		Rules:               rules,
	}

	ctx := context.Background()

	err := clientObj.CreatePolicy(ctx, createPolicyOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "POST" {
		t.Fatalf("expected POST method, got: %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/policies" {
		t.Fatalf("expected path api/v1/policies, got: %s", fake.lastOpts.Path)
	}
}

func TestUpdatePolicy_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Policy updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	updatePolicyOpts := &client.UpdatePolicyOptions{
		SessionAmbrUplink:   "150 Mbps",
		SessionAmbrDownlink: "250 Mbps",
		Var5qi:              8,
		Arp:                 2,
		DataNetworkName:     "internet",
		Rules:               nil,
	}

	ctx := context.Background()

	err := clientObj.UpdatePolicy(ctx, "test-policy", updatePolicyOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "PUT" {
		t.Fatalf("expected PUT method, got: %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/policies/test-policy" {
		t.Fatalf("expected path api/v1/policies/test-policy, got: %s", fake.lastOpts.Path)
	}
}

func TestUpdatePolicy_WithRules_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Policy updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	remotePrefix := "192.168.0.0/16"
	rules := &client.PolicyRules{
		Uplink: []client.PolicyRule{
			{
				Description:  "Allow SSH",
				RemotePrefix: &remotePrefix,
				Protocol:     6,
				PortLow:      22,
				PortHigh:     22,
				Action:       "allow",
			},
		},
		Downlink: []client.PolicyRule{},
	}

	updatePolicyOpts := &client.UpdatePolicyOptions{
		SessionAmbrUplink:   "200 Mbps",
		SessionAmbrDownlink: "300 Mbps",
		Var5qi:              7,
		Arp:                 3,
		DataNetworkName:     "internet",
		Rules:               rules,
	}

	ctx := context.Background()

	err := clientObj.UpdatePolicy(ctx, "test-policy", updatePolicyOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "PUT" {
		t.Fatalf("expected PUT method, got: %s", fake.lastOpts.Method)
	}
}

func TestGetPolicy_WithRules_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result: []byte(`{
				"name": "policy-with-rules",
				"profile_name": "policy-with-rules",
				"slice_name": "default",
				"data_network_name": "internet",
				"session_ambr_uplink": "100 Mbps",
				"session_ambr_downlink": "200 Mbps",
				"var5qi": 9,
				"arp": 1,
				"rules": {
					"uplink": [
						{
							"description": "Allow HTTP/HTTPS",
							"remote_prefix": "10.0.0.0/24",
							"protocol": 6,
							"port_low": 80,
							"port_high": 443,
							"action": "allow"
						}
					],
					"downlink": [
						{
							"description": "Allow DNS",
							"remote_prefix": null,
							"protocol": 17,
							"port_low": 53,
							"port_high": 53,
							"action": "allow"
						}
					]
				}
			}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	policy, err := clientObj.GetPolicy(ctx, &client.GetPolicyOptions{Name: "policy-with-rules"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if policy.Name != "policy-with-rules" {
		t.Fatalf("expected name 'policy-with-rules', got: %s", policy.Name)
	}

	if policy.Rules == nil {
		t.Fatalf("expected rules to be present, got nil")
	}

	if len(policy.Rules.Uplink) != 1 {
		t.Fatalf("expected 1 uplink rule, got: %d", len(policy.Rules.Uplink))
	}

	if len(policy.Rules.Downlink) != 1 {
		t.Fatalf("expected 1 downlink rule, got: %d", len(policy.Rules.Downlink))
	}

	if policy.Rules.Uplink[0].Description != "Allow HTTP/HTTPS" {
		t.Fatalf("expected description 'Allow HTTP/HTTPS', got: %s", policy.Rules.Uplink[0].Description)
	}

	if policy.Rules.Uplink[0].Protocol != 6 {
		t.Fatalf("expected protocol 6, got: %d", policy.Rules.Uplink[0].Protocol)
	}

	if policy.Rules.Downlink[0].RemotePrefix != nil {
		t.Fatalf("expected null remote_prefix, got: %v", *policy.Rules.Downlink[0].RemotePrefix)
	}
}

func TestGetPolicy_WithoutRules_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result: []byte(`{
				"name": "simple-policy",
				"profile_name": "simple-policy",
				"slice_name": "default",
				"data_network_name": "internet",
				"session_ambr_uplink": "100 Mbps",
				"session_ambr_downlink": "200 Mbps",
				"var5qi": 9,
				"arp": 1
			}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	policy, err := clientObj.GetPolicy(ctx, &client.GetPolicyOptions{Name: "simple-policy"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if policy.Name != "simple-policy" {
		t.Fatalf("expected name 'simple-policy', got: %s", policy.Name)
	}

	if policy.Rules != nil {
		t.Fatalf("expected rules to be nil, got: %v", policy.Rules)
	}
}
