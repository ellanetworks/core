package client_test

import (
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
		UeIPPool:        "10.45.0.0/16",
		DNS:             "8.8.8.8",
		Mtu:             1400,
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "100 Mbps",
		Var5qi:          9,
		PriorityLevel:   1,
	}

	err := clientObj.CreatePolicy(createPolicyOpts)
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
		UeIPPool:        "12312312312",
		DNS:             "8.8.8.8",
		Mtu:             1400,
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "100 Mbps",
		Var5qi:          9,
		PriorityLevel:   1,
	}

	err := clientObj.CreatePolicy(createPolicyOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetPolicy_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"name": "my-policy", "ue-ip-pool": "1.2.3.0/24"}`),
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

	policy, err := clientObj.GetPolicy(getRouteOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if policy.Name != name {
		t.Fatalf("expected ID %v, got %v", name, policy.Name)
	}

	if policy.UeIPPool != "1.2.3.0/24" {
		t.Fatalf("expected ID %v, got %v", "1.2.3.0/24", policy.UeIPPool)
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
	_, err := clientObj.GetPolicy(getPolicyOpts)
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
	err := clientObj.DeletePolicy(deletePolicyOpts)
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
	err := clientObj.DeletePolicy(deletePolicyOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListPolicies_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"imsi": "001010100000022", "policyName": "default"}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	policies, err := clientObj.ListPolicies()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
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

	_, err := clientObj.ListPolicies()
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
