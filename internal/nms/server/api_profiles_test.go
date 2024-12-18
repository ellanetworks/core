package server_test

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const ProfileName = "test-profile"

type CreateProfileSuccessResponse struct {
	Message string `json:"message"`
}

type GetProfileResponse struct {
	Name  string   `json:"name"`
	Imsis []string `json:"imsis"`

	Dnn             string `json:"dnn,omitempty"`
	UeIpPool        string `json:"ue-ip-pool,omitempty"`
	DnsPrimary      string `json:"dns-primary,omitempty"`
	DnsSecondary    string `json:"dns-secondary,omitempty"`
	Mtu             int32  `json:"mtu,omitempty"`
	BitrateUplink   int64  `json:"bitrate-uplink,omitempty"`
	BitrateDownlink int64  `json:"bitrate-downlink,omitempty"`
	BitrateUnit     string `json:"bitrate-unit,omitempty"`
	Qci             int32  `json:"qci,omitempty"`
	Arp             int32  `json:"arp,omitempty"`
	Pdb             int32  `json:"pdb,omitempty"`
	Pelr            int32  `json:"pelr,omitempty"`
}

type CreateProfileParams struct {
	Name  string   `json:"name"`
	Imsis []string `json:"imsis"`

	Dnn             string `json:"dnn,omitempty"`
	UeIpPool        string `json:"ue-ip-pool,omitempty"`
	DnsPrimary      string `json:"dns-primary,omitempty"`
	DnsSecondary    string `json:"dns-secondary,omitempty"`
	Mtu             int32  `json:"mtu,omitempty"`
	BitrateUplink   int64  `json:"bitrate-uplink,omitempty"`
	BitrateDownlink int64  `json:"bitrate-downlink,omitempty"`
	BitrateUnit     string `json:"bitrate-unit,omitempty"`
	Qci             int32  `json:"qci,omitempty"`
	Arp             int32  `json:"arp,omitempty"`
	Pdb             int32  `json:"pdb,omitempty"`
	Pelr            int32  `json:"pelr,omitempty"`
}

type CreateProfileResponseResult struct {
	ID int `json:"id"`
}

type CreateProfileResponse struct {
	Result CreateProfileSuccessResponse `json:"result"`
	Error  string                       `json:"error,omitempty"`
}

type DeleteProfileResponseResult struct {
	ID int `json:"id"`
}

func listProfiles(url string, client *http.Client) (int, []*GetProfileResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/profiles", nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var profileResponse []*GetProfileResponse
	if err := json.NewDecoder(res.Body).Decode(&profileResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, profileResponse, nil
}

func getProfile(url string, client *http.Client, name string) (int, *GetProfileResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/profiles/"+name, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var profileResponse GetProfileResponse
	if err := json.NewDecoder(res.Body).Decode(&profileResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &profileResponse, nil
}

func createProfile(url string, client *http.Client, data *CreateProfileParams) (int, *CreateProfileResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest("POST", url+"/api/v1/profiles", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var createResponse CreateProfileResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteProfile(url string, client *http.Client, name string) (int, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/profiles/"+name, nil)
	if err != nil {
		return 0, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	return res.StatusCode, nil
}

// This is an end-to-end test for the profiles handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestProfilesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, err := setupServer(db_path)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("1. List profiles - 0", func(t *testing.T) {
		statusCode, profiles, err := listProfiles(ts.URL, client)
		if err != nil {
			t.Fatalf("couldn't list profile: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if len(profiles) != 0 {
			t.Fatalf("expected 0 profiles, got %d", len(profiles))
		}
	})

	t.Run("2. Create profile", func(t *testing.T) {
		createProfileParams := &CreateProfileParams{
			Name:            ProfileName,
			Dnn:             "internet",
			UeIpPool:        "0.0.0.0/24",
			DnsPrimary:      "8.8.8.8",
			DnsSecondary:    "2.2.2.2",
			Mtu:             1500,
			BitrateUplink:   1000000,
			BitrateDownlink: 2000000,
			BitrateUnit:     "bps",
			Qci:             9,
			Arp:             1,
			Pdb:             1,
			Pelr:            1,
		}
		statusCode, response, err := createProfile(ts.URL, client, createProfileParams)
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

	t.Run("3. List profiles - 1", func(t *testing.T) {
		statusCode, profiles, err := listProfiles(ts.URL, client)
		if err != nil {
			t.Fatalf("couldn't list profile: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if len(profiles) != 1 {
			t.Fatalf("expected 1 profile, got %d", len(profiles))
		}
	})

	t.Run("4. Get profile", func(t *testing.T) {
		statusCode, response, err := getProfile(ts.URL, client, ProfileName)
		if err != nil {
			t.Fatalf("couldn't get profile: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Name != ProfileName {
			t.Fatalf("expected name %s, got %s", ProfileName, response.Name)
		}
		if response.UeIpPool != "0.0.0.0/24" {
			t.Fatalf("expected ue-ip-pool 0.0.0.0/24 got %s", response.UeIpPool)
		}
		if response.DnsPrimary != "8.8.8.8" {
			t.Fatalf("expected dns-primary 8.8.8.8 got %s", response.DnsPrimary)
		}
		if response.DnsSecondary != "2.2.2.2" {
			t.Fatalf("expected dns-secondary 2.2.2.2 got %s", response.DnsSecondary)
		}
		if response.Mtu != 1500 {
			t.Fatalf("expected mtu 1500 got %d", response.Mtu)
		}
		if response.BitrateUplink != 1000000 {
			t.Fatalf("expected bitrate-uplink 1000000 got %d", response.BitrateUplink)
		}
		if response.BitrateDownlink != 2000000 {
			t.Fatalf("expected bitrate-downlink 2000000 got %d", response.BitrateDownlink)
		}
		if response.BitrateUnit != "bps" {
			t.Fatalf("expected bitrate-unit bps got %s", response.BitrateUnit)
		}
		if response.Qci != 9 {
			t.Fatalf("expected qci 9 got %d", response.Qci)
		}
		if response.Arp != 1 {
			t.Fatalf("expected arp 1 got %d", response.Arp)
		}
		if response.Pdb != 1 {
			t.Fatalf("expected pdb 1 got %d", response.Pdb)
		}
		if response.Pelr != 1 {
			t.Fatalf("expected pelr 1 got %d", response.Pelr)
		}
	})

	t.Run("5. Get profile - id not found", func(t *testing.T) {
		statusCode, _, err := getProfile(ts.URL, client, "device-group-002")
		if err != nil {
			t.Fatalf("couldn't get profile: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
	})

	t.Run("5. Create profile - no name", func(t *testing.T) {
		createProfileParams := &CreateProfileParams{}
		statusCode, _, err := createProfile(ts.URL, client, createProfileParams)
		if err != nil {
			t.Fatalf("couldn't create profile: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})

	t.Run("6. Delete profile - success", func(t *testing.T) {
		statusCode, err := deleteProfile(ts.URL, client, ProfileName)
		if err != nil {
			t.Fatalf("couldn't delete profile: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
	})

	t.Run("7. Delete profile - no device group", func(t *testing.T) {
		statusCode, err := deleteProfile(ts.URL, client, ProfileName)
		if err != nil {
			t.Fatalf("couldn't delete profile: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
	})
}
