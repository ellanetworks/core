package server_test

import (
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
)

func TestSlicesEndToEnd(t *testing.T) {
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

	// Step 1: List slices (default slice should exist)
	t.Run("1. List default slices", func(t *testing.T) {
		statusCode, response, err := listSlices(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("Couldn't complete listSlices: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.TotalCount != 1 {
			t.Fatalf("Expected 1 default slice, got %d", response.Result.TotalCount)
		}

		if response.Result.Items[0].Name != "default" {
			t.Fatalf("Expected default slice name, got %q", response.Result.Items[0].Name)
		}
	})

	// Step 2: Get the default slice
	t.Run("2. Get default slice", func(t *testing.T) {
		statusCode, response, err := getSlice(env.Server.URL, client, token, "default")
		if err != nil {
			t.Fatalf("Couldn't complete getSlice: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.Name != "default" {
			t.Fatalf("Expected name 'default', got %q", response.Result.Name)
		}

		if response.Result.Sst != 1 {
			t.Fatalf("Expected SST 1, got %d", response.Result.Sst)
		}

		if response.Result.Sd != "" {
			t.Fatalf("Expected SD '', got %q", response.Result.Sd)
		}
	})

	// Step 3: Update the default slice
	t.Run("3. Update default slice", func(t *testing.T) {
		updateParams := &UpdateSliceParams{
			Sst: 2,
			Sd:  "aabbcc",
		}

		statusCode, updateResp, err := editSlice(env.Server.URL, client, "default", token, updateParams)
		if err != nil {
			t.Fatalf("Couldn't complete editSlice: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d (error: %s)", http.StatusOK, statusCode, updateResp.Error)
		}
	})

	// Step 4: Verify update
	t.Run("4. Verify slice update", func(t *testing.T) {
		statusCode, response, err := getSlice(env.Server.URL, client, token, "default")
		if err != nil {
			t.Fatalf("Couldn't complete getSlice: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Result.Sst != 2 {
			t.Fatalf("Expected updated SST 2, got %d", response.Result.Sst)
		}

		if response.Result.Sd != "aabbcc" {
			t.Fatalf("Expected updated SD 'aabbcc', got %q", response.Result.Sd)
		}
	})

	// Step 5: Get not found
	t.Run("5. Get non-existent slice", func(t *testing.T) {
		statusCode, _, err := getSlice(env.Server.URL, client, token, "nonexistent")
		if err != nil {
			t.Fatalf("Couldn't complete getSlice: %s", err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("Expected status %d, got %d", http.StatusNotFound, statusCode)
		}
	})
}

func TestSliceMultipleSlicesAllowed(t *testing.T) {
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

	// Default slice already exists; creating a second one should succeed
	slice := &CreateSliceParams{
		Name: "second-slice",
		Sst:  1,
		Sd:   "abcdef",
	}

	statusCode, _, err := createSlice(env.Server.URL, client, token, slice)
	if err != nil {
		t.Fatalf("Couldn't complete createSlice: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status %d for multi-slice creation, got %d", http.StatusCreated, statusCode)
	}
}

func TestSliceInvalidInput(t *testing.T) {
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

	// Test invalid update on the default slice
	testCases := []struct {
		name       string
		params     *UpdateSliceParams
		expectCode int
	}{
		{
			name: "missing SST",
			params: &UpdateSliceParams{
				Sst: 0,
				Sd:  "102030",
			},
			expectCode: http.StatusBadRequest,
		},
		{
			name: "SST too large",
			params: &UpdateSliceParams{
				Sst: 256,
				Sd:  "102030",
			},
			expectCode: http.StatusBadRequest,
		},
		{
			name: "invalid SD format",
			params: &UpdateSliceParams{
				Sst: 1,
				Sd:  "not-hex",
			},
			expectCode: http.StatusBadRequest,
		},
		{
			name: "SD too short",
			params: &UpdateSliceParams{
				Sst: 1,
				Sd:  "abc",
			},
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			statusCode, _, err := editSlice(env.Server.URL, client, "default", token, tc.params)
			if err != nil {
				t.Fatalf("Couldn't complete editSlice: %s", err)
			}

			if statusCode != tc.expectCode {
				t.Fatalf("Expected status %d, got %d", tc.expectCode, statusCode)
			}
		})
	}
}

func TestSliceDeleteGuardPolicies(t *testing.T) {
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

	// Default slice has the default policy, so deleting it should fail
	statusCode, delResp, err := deleteSlice(env.Server.URL, client, token, "default")
	if err != nil {
		t.Fatalf("Couldn't complete deleteSlice: %s", err)
	}

	if statusCode != http.StatusConflict {
		t.Fatalf("Expected status %d for slice with policies, got %d (error: %s)", http.StatusConflict, statusCode, delResp.Error)
	}
}

func TestSliceUpdateNotFound(t *testing.T) {
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

	statusCode, _, err := editSlice(env.Server.URL, client, "nonexistent", token, &UpdateSliceParams{
		Sst: 1,
		Sd:  "102030",
	})
	if err != nil {
		t.Fatalf("Couldn't complete editSlice: %s", err)
	}

	if statusCode != http.StatusNotFound {
		t.Fatalf("Expected status %d, got %d", http.StatusNotFound, statusCode)
	}
}

func TestSliceDeleteNotFound(t *testing.T) {
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

	statusCode, _, err := deleteSlice(env.Server.URL, client, token, "nonexistent")
	if err != nil {
		t.Fatalf("Couldn't complete deleteSlice: %s", err)
	}

	if statusCode != http.StatusNotFound {
		t.Fatalf("Expected status %d, got %d", http.StatusNotFound, statusCode)
	}
}

func TestSliceMaxLimitEnforced(t *testing.T) {
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

	// DB initialization creates 1 default slice, so create 7 more to hit the limit of 8
	for i := 1; i <= 7; i++ {
		slice := &CreateSliceParams{
			Name: fmt.Sprintf("slice-%d", i),
			Sst:  i,
			Sd:   fmt.Sprintf("%06x", i),
		}

		statusCode, _, err := createSlice(env.Server.URL, client, token, slice)
		if err != nil {
			t.Fatalf("Couldn't create slice %d: %s", i, err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("Expected status %d for slice %d, got %d", http.StatusCreated, i, statusCode)
		}
	}

	// The 9th slice (8 + 1 default) should be rejected
	excessSlice := &CreateSliceParams{
		Name: "excess-slice",
		Sst:  9,
		Sd:   "aaaaaa",
	}

	statusCode, response, err := createSlice(env.Server.URL, client, token, excessSlice)
	if err != nil {
		t.Fatalf("Couldn't attempt excess slice creation: %s", err)
	}

	if statusCode != http.StatusBadRequest {
		t.Fatalf("Expected status %d, got %d", http.StatusBadRequest, statusCode)
	}

	if response.Error != "Maximum number of slices reached (8)" {
		t.Fatalf("expected error %q, got %q", "Maximum number of slices reached (8)", response.Error)
	}
}
