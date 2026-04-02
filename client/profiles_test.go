package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestCreateProfile_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Profile created successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.CreateProfileOptions{
		Name:           "enterprise",
		UeAmbrUplink:   "1 Gbps",
		UeAmbrDownlink: "1 Gbps",
	}

	ctx := context.Background()

	err := clientObj.CreateProfile(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "POST" {
		t.Fatalf("expected POST method, got: %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/profiles" {
		t.Fatalf("expected path api/v1/profiles, got: %s", fake.lastOpts.Path)
	}
}

func TestCreateProfile_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid profile"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.CreateProfileOptions{
		Name:           "enterprise",
		UeAmbrUplink:   "1 Gbps",
		UeAmbrDownlink: "1 Gbps",
	}

	ctx := context.Background()

	err := clientObj.CreateProfile(ctx, opts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetProfile_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"name": "enterprise", "ue_ambr_uplink": "1 Gbps", "ue_ambr_downlink": "1 Gbps"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	profile, err := clientObj.GetProfile(ctx, &client.GetProfileOptions{Name: "enterprise"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if profile.Name != "enterprise" {
		t.Fatalf("expected name 'enterprise', got: %s", profile.Name)
	}

	if profile.UeAmbrUplink != "1 Gbps" {
		t.Fatalf("expected ue_ambr_uplink '1 Gbps', got: %s", profile.UeAmbrUplink)
	}

	if profile.UeAmbrDownlink != "1 Gbps" {
		t.Fatalf("expected ue_ambr_downlink '1 Gbps', got: %s", profile.UeAmbrDownlink)
	}
}

func TestGetProfile_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Profile not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	_, err := clientObj.GetProfile(ctx, &client.GetProfileOptions{Name: "non-existent"})
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestUpdateProfile_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Profile updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.UpdateProfileOptions{
		UeAmbrUplink:   "2 Gbps",
		UeAmbrDownlink: "2 Gbps",
	}

	ctx := context.Background()

	err := clientObj.UpdateProfile(ctx, "enterprise", opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "PUT" {
		t.Fatalf("expected PUT method, got: %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/profiles/enterprise" {
		t.Fatalf("expected path api/v1/profiles/enterprise, got: %s", fake.lastOpts.Path)
	}
}

func TestUpdateProfile_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Profile not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.UpdateProfileOptions{
		UeAmbrUplink: "2 Gbps",
	}

	ctx := context.Background()

	err := clientObj.UpdateProfile(ctx, "non-existent", opts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestDeleteProfile_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Profile deleted successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	err := clientObj.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: "enterprise"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "DELETE" {
		t.Fatalf("expected DELETE method, got: %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/profiles/enterprise" {
		t.Fatalf("expected path api/v1/profiles/enterprise, got: %s", fake.lastOpts.Path)
	}
}

func TestDeleteProfile_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 409,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Profile has subscribers"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	err := clientObj.DeleteProfile(ctx, &client.DeleteProfileOptions{Name: "enterprise"})
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListProfiles_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"items": [{"name": "enterprise", "ue_ambr_uplink": "1 Gbps", "ue_ambr_downlink": "1 Gbps"}, {"name": "iot", "ue_ambr_uplink": "10 Mbps", "ue_ambr_downlink": "10 Mbps"}], "page": 1, "per_page": 10, "total_count": 2}`),
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

	resp, err := clientObj.ListProfiles(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 profiles, got: %d", len(resp.Items))
	}

	if resp.Items[0].Name != "enterprise" {
		t.Fatalf("expected first profile name 'enterprise', got: %s", resp.Items[0].Name)
	}
}

func TestListProfiles_Failure(t *testing.T) {
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

	_, err := clientObj.ListProfiles(ctx, params)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
