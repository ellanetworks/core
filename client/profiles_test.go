package client_test

import (
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

	createProfileOpts := &client.CreateProfileOptions{
		Name:            "testProfile",
		UeIPPool:        "10.45.0.0/16",
		DNS:             "8.8.8.8",
		Mtu:             1400,
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "100 Mbps",
		Var5qi:          9,
		PriorityLevel:   1,
	}

	err := clientObj.CreateProfile(createProfileOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCreateProfile_Failure(t *testing.T) {
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
	createProfileOpts := &client.CreateProfileOptions{
		Name:            "testProfile",
		UeIPPool:        "12312312312",
		DNS:             "8.8.8.8",
		Mtu:             1400,
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "100 Mbps",
		Var5qi:          9,
		PriorityLevel:   1,
	}

	err := clientObj.CreateProfile(createProfileOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetProfile_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"name": "my-profile", "ue-ip-pool": "1.2.3.0/24"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	name := "my-profile"

	getRouteOpts := &client.GetProfileOptions{
		Name: name,
	}

	profile, err := clientObj.GetProfile(getRouteOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if profile.Name != name {
		t.Fatalf("expected ID %v, got %v", name, profile.Name)
	}

	if profile.UeIPPool != "1.2.3.0/24" {
		t.Fatalf("expected ID %v, got %v", "1.2.3.0/24", profile.UeIPPool)
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

	name := "non-existent-profile"
	getProfileOpts := &client.GetProfileOptions{
		Name: name,
	}
	_, err := clientObj.GetProfile(getProfileOpts)
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
	name := "testProfile"

	deleteProfileOpts := &client.DeleteProfileOptions{
		Name: name,
	}
	err := clientObj.DeleteProfile(deleteProfileOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDeleteProfile_Failure(t *testing.T) {
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

	name := "non-existent-profile"

	deleteProfileOpts := &client.DeleteProfileOptions{
		Name: name,
	}
	err := clientObj.DeleteProfile(deleteProfileOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListProfiles_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"imsi": "001010100000022", "profileName": "default"}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	profiles, err := clientObj.ListProfiles()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
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

	_, err := clientObj.ListProfiles()
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
