package server_test

import (
	"net/http"
	"path/filepath"
	"testing"
)

const (
	ProfileUeAmbrUplink   = "500 Mbps"
	ProfileUeAmbrDownlink = "1 Gbps"
)

func TestProfilesEndToEnd(t *testing.T) {
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

	// Step 1: List profiles (default profile should exist)
	t.Run("1. List default profiles", func(t *testing.T) {
		statusCode, response, err := listProfiles(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("Couldn't complete listProfiles: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.TotalCount != 1 {
			t.Fatalf("Expected 1 default profile, got %d", response.Result.TotalCount)
		}

		if response.Result.Items[0].Name != "default" {
			t.Fatalf("Expected default profile name, got %q", response.Result.Items[0].Name)
		}
	})

	// Step 2: Create a new profile
	t.Run("2. Create profile", func(t *testing.T) {
		profile := &CreateProfileParams{
			Name:           "my-profile",
			UeAmbrUplink:   ProfileUeAmbrUplink,
			UeAmbrDownlink: ProfileUeAmbrDownlink,
		}

		statusCode, _, err := createProfile(env.Server.URL, client, token, profile)
		if err != nil {
			t.Fatalf("Couldn't complete createProfile: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("Expected status %d, got %d", http.StatusCreated, statusCode)
		}
	})

	// Step 3: List profiles (should now have 2)
	t.Run("3. List profiles after creation", func(t *testing.T) {
		statusCode, response, err := listProfiles(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("Couldn't complete listProfiles: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.TotalCount != 2 {
			t.Fatalf("Expected 2 profiles, got %d", response.Result.TotalCount)
		}
	})

	// Step 4: Get the created profile
	t.Run("4. Get profile", func(t *testing.T) {
		statusCode, response, err := getProfile(env.Server.URL, client, token, "my-profile")
		if err != nil {
			t.Fatalf("Couldn't complete getProfile: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.Name != "my-profile" {
			t.Fatalf("Expected name 'my-profile', got %q", response.Result.Name)
		}

		if response.Result.UeAmbrUplink != ProfileUeAmbrUplink {
			t.Fatalf("Expected uplink %q, got %q", ProfileUeAmbrUplink, response.Result.UeAmbrUplink)
		}

		if response.Result.UeAmbrDownlink != ProfileUeAmbrDownlink {
			t.Fatalf("Expected downlink %q, got %q", ProfileUeAmbrDownlink, response.Result.UeAmbrDownlink)
		}
	})

	// Step 5: Update the profile
	t.Run("5. Update profile", func(t *testing.T) {
		updateParams := &UpdateProfileParams{
			UeAmbrUplink:   "1 Gbps",
			UeAmbrDownlink: "2 Gbps",
		}

		statusCode, updateResp, err := editProfile(env.Server.URL, client, "my-profile", token, updateParams)
		if err != nil {
			t.Fatalf("Couldn't complete editProfile: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d (error: %s)", http.StatusOK, statusCode, updateResp.Error)
		}
	})

	// Step 6: Verify the update
	t.Run("6. Verify update", func(t *testing.T) {
		statusCode, response, err := getProfile(env.Server.URL, client, token, "my-profile")
		if err != nil {
			t.Fatalf("Couldn't complete getProfile: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.UeAmbrUplink != "1 Gbps" {
			t.Fatalf("Expected uplink '1 Gbps', got %q", response.Result.UeAmbrUplink)
		}

		if response.Result.UeAmbrDownlink != "2 Gbps" {
			t.Fatalf("Expected downlink '2 Gbps', got %q", response.Result.UeAmbrDownlink)
		}
	})

	// Step 7: Delete the profile
	t.Run("7. Delete profile", func(t *testing.T) {
		statusCode, delResp, err := deleteProfile(env.Server.URL, client, token, "my-profile")
		if err != nil {
			t.Fatalf("Couldn't complete deleteProfile: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d (error: %s)", http.StatusOK, statusCode, delResp.Error)
		}
	})

	// Step 8: Verify deletion
	t.Run("8. Verify deletion", func(t *testing.T) {
		statusCode, _, err := getProfile(env.Server.URL, client, token, "my-profile")
		if err != nil {
			t.Fatalf("Couldn't complete getProfile: %s", err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("Expected status %d, got %d", http.StatusNotFound, statusCode)
		}
	})

	// Step 9: List after deletion
	t.Run("9. List profiles after deletion", func(t *testing.T) {
		statusCode, response, err := listProfiles(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("Couldn't complete listProfiles: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.TotalCount != 1 {
			t.Fatalf("Expected 1 profile after deletion, got %d", response.Result.TotalCount)
		}
	})
}

func TestProfileInvalidInput(t *testing.T) {
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

	testCases := []struct {
		name       string
		params     *CreateProfileParams
		expectCode int
	}{
		{
			name: "missing name",
			params: &CreateProfileParams{
				Name:           "",
				UeAmbrUplink:   "100 Mbps",
				UeAmbrDownlink: "200 Mbps",
			},
			expectCode: http.StatusBadRequest,
		},
		{
			name: "missing uplink",
			params: &CreateProfileParams{
				Name:           "bad-profile",
				UeAmbrUplink:   "",
				UeAmbrDownlink: "200 Mbps",
			},
			expectCode: http.StatusBadRequest,
		},
		{
			name: "missing downlink",
			params: &CreateProfileParams{
				Name:           "bad-profile",
				UeAmbrUplink:   "100 Mbps",
				UeAmbrDownlink: "",
			},
			expectCode: http.StatusBadRequest,
		},
		{
			name: "invalid uplink format",
			params: &CreateProfileParams{
				Name:           "bad-profile",
				UeAmbrUplink:   "not-a-bitrate",
				UeAmbrDownlink: "200 Mbps",
			},
			expectCode: http.StatusBadRequest,
		},
		{
			name: "invalid downlink format",
			params: &CreateProfileParams{
				Name:           "bad-profile",
				UeAmbrUplink:   "100 Mbps",
				UeAmbrDownlink: "not-a-bitrate",
			},
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			statusCode, _, err := createProfile(env.Server.URL, client, token, tc.params)
			if err != nil {
				t.Fatalf("Couldn't complete createProfile: %s", err)
			}

			if statusCode != tc.expectCode {
				t.Fatalf("Expected status %d, got %d", tc.expectCode, statusCode)
			}
		})
	}
}

func TestProfileDuplicate(t *testing.T) {
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

	profile := &CreateProfileParams{
		Name:           "dup-profile",
		UeAmbrUplink:   "100 Mbps",
		UeAmbrDownlink: "200 Mbps",
	}

	statusCode, _, err := createProfile(env.Server.URL, client, token, profile)
	if err != nil {
		t.Fatalf("Couldn't complete createProfile: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d", http.StatusCreated, statusCode)
	}

	// Try to create duplicate
	statusCode, _, err = createProfile(env.Server.URL, client, token, profile)
	if err != nil {
		t.Fatalf("Couldn't complete createProfile: %s", err)
	}

	if statusCode != http.StatusConflict {
		t.Fatalf("Expected status %d for duplicate, got %d", http.StatusConflict, statusCode)
	}
}

func TestProfileTooMany(t *testing.T) {
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

	// DB seeds 1 default profile. Create 11 more to reach the limit of 12.
	for i := 0; i < 11; i++ {
		profile := &CreateProfileParams{
			Name:           "profile-" + string(rune('a'+i)),
			UeAmbrUplink:   "100 Mbps",
			UeAmbrDownlink: "200 Mbps",
		}

		statusCode, _, err := createProfile(env.Server.URL, client, token, profile)
		if err != nil {
			t.Fatalf("Couldn't complete createProfile for profile-%c: %s", rune('a'+i), err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("Expected status %d for profile-%c, got %d", http.StatusCreated, rune('a'+i), statusCode)
		}
	}

	// The 13th profile should fail
	statusCode, _, err := createProfile(env.Server.URL, client, token, &CreateProfileParams{
		Name:           "one-too-many",
		UeAmbrUplink:   "100 Mbps",
		UeAmbrDownlink: "200 Mbps",
	})
	if err != nil {
		t.Fatalf("Couldn't complete createProfile: %s", err)
	}

	if statusCode != http.StatusConflict {
		t.Fatalf("Expected status %d for too many profiles, got %d", http.StatusConflict, statusCode)
	}
}

func TestProfileDeleteGuardSubscribers(t *testing.T) {
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

	// Create a profile
	profile := &CreateProfileParams{
		Name:           "guarded-profile",
		UeAmbrUplink:   "100 Mbps",
		UeAmbrDownlink: "200 Mbps",
	}

	statusCode, _, err := createProfile(env.Server.URL, client, token, profile)
	if err != nil {
		t.Fatalf("Couldn't complete createProfile: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d", http.StatusCreated, statusCode)
	}

	// Create a policy on this profile so subscribers can be assigned
	policy := &CreatePolicyParams{
		Name:                "guarded-policy",
		ProfileName:         "guarded-profile",
		SliceName:           "default",
		DataNetworkName:     "internet",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
	}

	statusCode, _, err = createPolicy(env.Server.URL, client, token, policy)
	if err != nil {
		t.Fatalf("Couldn't complete createPolicy: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d", http.StatusCreated, statusCode)
	}

	// Create a subscriber on this profile
	subscriber := &CreateSubscriberParams{
		Imsi:           "001010100007487",
		ProfileName:    "guarded-profile",
		SequenceNumber: "000000000001",
		Key:            "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
	}

	statusCode, _, err = createSubscriber(env.Server.URL, client, token, subscriber)
	if err != nil {
		t.Fatalf("Couldn't complete createSubscriber: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d", http.StatusCreated, statusCode)
	}

	// Try to delete the profile — should fail
	statusCode, _, err = deleteProfile(env.Server.URL, client, token, "guarded-profile")
	if err != nil {
		t.Fatalf("Couldn't complete deleteProfile: %s", err)
	}

	if statusCode != http.StatusConflict {
		t.Fatalf("Expected status %d for profile with subscribers, got %d", http.StatusConflict, statusCode)
	}

	// Delete the subscriber first, then the profile should succeed
	statusCode, _, err = deleteSubscriber(env.Server.URL, client, token, "001010100007487")
	if err != nil {
		t.Fatalf("Couldn't complete deleteSubscriber: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, statusCode)
	}

	// Delete the policy before the profile
	statusCode, _, err = deletePolicy(env.Server.URL, client, token, "guarded-policy")
	if err != nil {
		t.Fatalf("Couldn't complete deletePolicy: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, statusCode)
	}

	statusCode, _, err = deleteProfile(env.Server.URL, client, token, "guarded-profile")
	if err != nil {
		t.Fatalf("Couldn't complete deleteProfile: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, statusCode)
	}
}

func TestProfileUpdateNotFound(t *testing.T) {
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

	statusCode, _, err := editProfile(env.Server.URL, client, "nonexistent", token, &UpdateProfileParams{
		UeAmbrUplink:   "100 Mbps",
		UeAmbrDownlink: "200 Mbps",
	})
	if err != nil {
		t.Fatalf("Couldn't complete editProfile: %s", err)
	}

	if statusCode != http.StatusNotFound {
		t.Fatalf("Expected status %d, got %d", http.StatusNotFound, statusCode)
	}
}
