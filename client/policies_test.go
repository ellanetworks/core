package client_test

import (
	"context"
	"errors"
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
		Name:            "testPolicy",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "100 Mbps",
		Var5qi:          9,
		Arp:             1,
	}

	ctx := context.Background()

	err := clientObj.CreatePolicy(ctx, createPolicyOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCreatePolicy_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid UE IP Pool"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	createPolicyOpts := &client.CreatePolicyOptions{
		Name:            "testPolicy",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "100 Mbps",
		Var5qi:          9,
		Arp:             1,
	}

	ctx := context.Background()

	err := clientObj.CreatePolicy(ctx, createPolicyOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetPolicy_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"name": "my-policy", "ip_pool": "1.2.3.0/24"}`),
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

func TestGetPolicy_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Policy not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	name := "non-existent-policy"
	getPolicyOpts := &client.GetPolicyOptions{
		Name: name,
	}

	ctx := context.Background()

	_, err := clientObj.GetPolicy(ctx, getPolicyOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
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

func TestDeletePolicy_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Policy not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	name := "non-existent-policy"

	deletePolicyOpts := &client.DeletePolicyOptions{
		Name: name,
	}

	ctx := context.Background()

	err := clientObj.DeletePolicy(ctx, deletePolicyOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
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

func TestListPolicies_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 500,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Internal server error"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	params := &client.ListParams{
		Page:    1,
		PerPage: 10,
	}

	_, err := clientObj.ListPolicies(ctx, params)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
