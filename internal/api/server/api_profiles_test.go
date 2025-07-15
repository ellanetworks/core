package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/api/server"
)

const (
	ProfileName     = "test-profile"
	DNS             = "8.8.8.8"
	UeIPPool        = "0.0.0.0/24"
	Mtu             = 1500
	BitrateUplink   = "100 Mbps"
	BitrateDownlink = "200 Mbps"
	Var5qi          = 9
	PriorityLevel   = 1
)

type CreateProfileResponseResult struct {
	Message string `json:"message"`
}

type GetProfileResponseResult struct {
	Name string `json:"name"`

	UeIPPool        string `json:"ue-ip-pool,omitempty"`
	DNS             string `json:"dns,omitempty"`
	Mtu             int32  `json:"mtu,omitempty"`
	BitrateUplink   string `json:"bitrate-uplink,omitempty"`
	BitrateDownlink string `json:"bitrate-downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	PriorityLevel   int32  `json:"priority-level,omitempty"`
}

type GetProfileResponse struct {
	Result GetProfileResponseResult `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

type CreateProfileParams struct {
	Name string `json:"name"`

	UeIPPool        string `json:"ue-ip-pool,omitempty"`
	DNS             string `json:"dns,omitempty"`
	Mtu             int32  `json:"mtu,omitempty"`
	BitrateUplink   string `json:"bitrate-uplink,omitempty"`
	BitrateDownlink string `json:"bitrate-downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	PriorityLevel   int32  `json:"priority-level,omitempty"`
}

type CreateProfileResponse struct {
	Result CreateProfileResponseResult `json:"result"`
	Error  string                      `json:"error,omitempty"`
}

type DeleteProfileResponseResult struct {
	Message string `json:"message"`
}

type DeleteProfileResponse struct {
	Result DeleteProfileResponseResult `json:"result"`
	Error  string                      `json:"error,omitempty"`
}

type ListProfileResponse struct {
	Result []GetProfileResponse `json:"result"`
	Error  string               `json:"error,omitempty"`
}

func listProfiles(url string, client *http.Client, token string) (int, *ListProfileResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/profiles", nil)
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
	var profileResponse ListProfileResponse
	if err := json.NewDecoder(res.Body).Decode(&profileResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &profileResponse, nil
}

func getProfile(url string, client *http.Client, token string, name string) (int, *GetProfileResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/profiles/"+name, nil)
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
	var profileResponse GetProfileResponse
	if err := json.NewDecoder(res.Body).Decode(&profileResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &profileResponse, nil
}

func createProfile(url string, client *http.Client, token string, data *CreateProfileParams) (int, *CreateProfileResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/profiles", strings.NewReader(string(body)))
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
	var createResponse CreateProfileResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func editProfile(url string, client *http.Client, name string, token string, data *CreateProfileParams) (int, *CreateProfileResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/profiles/"+name, strings.NewReader(string(body)))
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
	var createResponse CreateProfileResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteProfile(url string, client *http.Client, token, name string) (int, *DeleteProfileResponse, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/profiles/"+name, nil)
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
	var deleteProfileResponse DeleteProfileResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteProfileResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &deleteProfileResponse, nil
}

// This is an end-to-end test for the profiles handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestAPIProfilesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath, server.TestMode)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. List profiles - 0", func(t *testing.T) {
		statusCode, response, err := listProfiles(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list profile: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if len(response.Result) != 0 {
			t.Fatalf("expected 0 profiles, got %d", len(response.Result))
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("2. Create profile", func(t *testing.T) {
		createProfileParams := &CreateProfileParams{
			Name:            ProfileName,
			UeIPPool:        "0.0.0.0/24",
			DNS:             "8.8.8.8",
			Mtu:             1500,
			BitrateUplink:   "100 Mbps",
			BitrateDownlink: "200 Mbps",
			Var5qi:          9,
			PriorityLevel:   1,
		}
		statusCode, response, err := createProfile(ts.URL, client, token, createProfileParams)
		if err != nil {
			t.Fatalf("couldn't create profile: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "Profile created successfully" {
			t.Fatalf("expected message 'Profile created successfully', got %q", response.Result.Message)
		}
	})

	t.Run("3. List profiles - 1", func(t *testing.T) {
		statusCode, response, err := listProfiles(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list profile: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if len(response.Result) != 1 {
			t.Fatalf("expected 1 profile, got %d", len(response.Result))
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("4. Get profile", func(t *testing.T) {
		statusCode, response, err := getProfile(ts.URL, client, token, ProfileName)
		if err != nil {
			t.Fatalf("couldn't get profile: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Name != ProfileName {
			t.Fatalf("expected name %s, got %s", ProfileName, response.Result.Name)
		}
		if response.Result.UeIPPool != "0.0.0.0/24" {
			t.Fatalf("expected ue-ip-pool 0.0.0.0/24 got %s", response.Result.UeIPPool)
		}
		if response.Result.DNS != "8.8.8.8" {
			t.Fatalf("expected dns 8.8.8.8 got %s", response.Result.DNS)
		}
		if response.Result.Mtu != 1500 {
			t.Fatalf("expected mtu 1500 got %d", response.Result.Mtu)
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
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("5. Get profile - id not found", func(t *testing.T) {
		statusCode, response, err := getProfile(ts.URL, client, token, "profile-002")
		if err != nil {
			t.Fatalf("couldn't get profile: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
		if response.Error != "Profile not found" {
			t.Fatalf("expected error %q, got %q", "Profile not found", response.Error)
		}
	})

	t.Run("5. Create profile - no name", func(t *testing.T) {
		createProfileParams := &CreateProfileParams{}
		statusCode, response, err := createProfile(ts.URL, client, token, createProfileParams)
		if err != nil {
			t.Fatalf("couldn't create profile: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
		if response.Error != "name is missing" {
			t.Fatalf("expected error %q, got %q", "name is missing", response.Error)
		}
	})

	t.Run("6. Edit profile - success", func(t *testing.T) {
		createProfileParams := &CreateProfileParams{
			Name:            ProfileName,
			UeIPPool:        "2.2.2.2/24",
			DNS:             "1.1.1.1",
			Mtu:             1500,
			BitrateUplink:   "100 Mbps",
			BitrateDownlink: "200 Mbps",
			Var5qi:          9,
			PriorityLevel:   1,
		}
		statusCode, response, err := editProfile(ts.URL, client, ProfileName, token, createProfileParams)
		if err != nil {
			t.Fatalf("couldn't edit profile: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("7. Add subscriber to profile", func(t *testing.T) {
		createSubscriberParams := &CreateSubscriberParams{
			Imsi:           Imsi,
			Key:            Key,
			SequenceNumber: SequenceNumber,
			ProfileName:    ProfileName,
		}
		statusCode, response, err := createSubscriber(ts.URL, client, token, createSubscriberParams)
		if err != nil {
			t.Fatalf("couldn't edit profile: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("8. Delete profile - failure", func(t *testing.T) {
		statusCode, response, err := deleteProfile(ts.URL, client, token, ProfileName)
		if err != nil {
			t.Fatalf("couldn't delete profile: %s", err)
		}
		if statusCode != http.StatusConflict {
			t.Fatalf("expected status %d, got %d", http.StatusConflict, statusCode)
		}
		if response.Error != "Profile has subscribers" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("7. Delete subscriber", func(t *testing.T) {
		statusCode, response, err := deleteSubscriber(ts.URL, client, token, Imsi)
		if err != nil {
			t.Fatalf("couldn't edit profile: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("8. Delete profile - success", func(t *testing.T) {
		statusCode, response, err := deleteProfile(ts.URL, client, token, ProfileName)
		if err != nil {
			t.Fatalf("couldn't delete profile: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("9. Delete profile - no profile", func(t *testing.T) {
		statusCode, response, err := deleteProfile(ts.URL, client, token, ProfileName)
		if err != nil {
			t.Fatalf("couldn't delete profile: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
		if response.Error != "Profile not found" {
			t.Fatalf("expected error %q, got %q", "Profile not found", response.Error)
		}
	})
}

func TestCreateProfileInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath, server.TestMode)
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
		ueIPPool        string
		dns             string
		mtu             int32
		bitrateUplink   string
		bitrateDownlink string
		var5qi          int32
		priorityLevel   int32
		error           string
	}{
		{
			testName:        "Invalid ueIPPool - missing subnet",
			name:            ProfileName,
			ueIPPool:        "0.0.0.0",
			dns:             DNS,
			mtu:             Mtu,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid ue-ip-pool format. Must be in CIDR format",
		},
		{
			testName:        "Invalid ueIPPool - Too many bits",
			name:            ProfileName,
			ueIPPool:        "0.0.0.0/2555",
			dns:             DNS,
			mtu:             Mtu,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid ue-ip-pool format. Must be in CIDR format",
		},
		{
			testName:        "Invalid Name",
			name:            strings.Repeat("a", 257),
			ueIPPool:        UeIPPool,
			dns:             DNS,
			mtu:             Mtu,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid name format. Must be less than 256 characters",
		},
		{
			testName:        "Invalid DNS Primary",
			name:            ProfileName,
			ueIPPool:        UeIPPool,
			dns:             "not a valid ip",
			mtu:             Mtu,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid dns format. Must be a valid IP address",
		},
		{
			testName:        "Invalid MTU - Negative",
			name:            ProfileName,
			ueIPPool:        UeIPPool,
			dns:             DNS,
			mtu:             -1,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid mtu format. Must be an integer between 0 and 65535",
		},
		{
			testName:        "Invalid MTU - Too large",
			name:            ProfileName,
			ueIPPool:        UeIPPool,
			dns:             DNS,
			mtu:             65536,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid mtu format. Must be an integer between 0 and 65535",
		},
		{
			testName:        "Invalid Uplink Bitrate - Missing unit",
			name:            ProfileName,
			ueIPPool:        UeIPPool,
			dns:             DNS,
			mtu:             Mtu,
			bitrateUplink:   "200",
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Uplink Bitrate - Invalid unit",
			name:            ProfileName,
			ueIPPool:        UeIPPool,
			dns:             DNS,
			mtu:             Mtu,
			bitrateUplink:   "200 Tbps",
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Uplink Bitrate - Zero value",
			name:            ProfileName,
			ueIPPool:        UeIPPool,
			dns:             DNS,
			mtu:             Mtu,
			bitrateUplink:   "0 Mbps",
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Uplink Bitrate - Negative value",
			name:            ProfileName,
			ueIPPool:        UeIPPool,
			dns:             DNS,
			mtu:             Mtu,
			bitrateUplink:   "-1 Mbps",
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Uplink Bitrate - Too large value",
			name:            ProfileName,
			ueIPPool:        UeIPPool,
			dns:             DNS,
			mtu:             Mtu,
			bitrateUplink:   "1001 Mbps",
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Downlink Bitrate - Missing unit",
			name:            ProfileName,
			ueIPPool:        UeIPPool,
			dns:             DNS,
			mtu:             Mtu,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: "200",
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Downlink Bitrate - Invalid unit",
			name:            ProfileName,
			ueIPPool:        UeIPPool,
			dns:             DNS,
			mtu:             Mtu,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: "200 Tbps",
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Downlink Bitrate - Zero value",
			name:            ProfileName,
			ueIPPool:        UeIPPool,
			dns:             DNS,
			mtu:             Mtu,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: "0 Mbps",
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Downlink Bitrate - Negative value",
			name:            ProfileName,
			ueIPPool:        UeIPPool,
			dns:             DNS,
			mtu:             Mtu,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: "-1 Mbps",
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid Downlink Bitrate - Too large value",
			name:            ProfileName,
			ueIPPool:        UeIPPool,
			dns:             DNS,
			mtu:             Mtu,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: "1001 Mbps",
			var5qi:          Var5qi,
			priorityLevel:   PriorityLevel,
			error:           "Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps",
		},
		{
			testName:        "Invalid 5QI - Too large value",
			name:            ProfileName,
			ueIPPool:        UeIPPool,
			dns:             DNS,
			mtu:             Mtu,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: BitrateDownlink,
			var5qi:          256,
			priorityLevel:   PriorityLevel,
			error:           "Invalid Var5qi format. Must be an integer between 1 and 255",
		},
		{
			testName:        "Invalid Priority Level - Too large value",
			name:            ProfileName,
			ueIPPool:        UeIPPool,
			dns:             DNS,
			mtu:             Mtu,
			bitrateUplink:   BitrateUplink,
			bitrateDownlink: BitrateDownlink,
			var5qi:          Var5qi,
			priorityLevel:   256,
			error:           "Invalid priority-level format. Must be an integer between 1 and 255",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			createProfileParams := &CreateProfileParams{
				Name:            tt.name,
				UeIPPool:        tt.ueIPPool,
				DNS:             tt.dns,
				Mtu:             tt.mtu,
				BitrateUplink:   tt.bitrateUplink,
				BitrateDownlink: tt.bitrateDownlink,
				Var5qi:          tt.var5qi,
				PriorityLevel:   tt.priorityLevel,
			}
			statusCode, response, err := createProfile(ts.URL, client, token, createProfileParams)
			if err != nil {
				t.Fatalf("couldn't create profile: %s", err)
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
