package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
)

type RegisterFleetParams struct {
	ActivationToken string `json:"activationToken"`
}

type RegisterFleetResponseResult struct {
	Message string `json:"message"`
}

type RegisterFleetResponse struct {
	Result RegisterFleetResponseResult `json:"result"`
	Error  string                      `json:"error,omitempty"`
}

func registerFleet(url string, client *http.Client, token string, params *RegisterFleetParams) (int, *RegisterFleetResponse, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/fleet/register", bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var response RegisterFleetResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func registerFleetRaw(url string, client *http.Client, token string, body string) (int, *RegisterFleetResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/fleet/register", bytes.NewReader([]byte(body)))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var response RegisterFleetResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func TestApiFleetEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}

	defer ts.Close()

	client := ts.Client()

	adminToken, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Register fleet without auth token returns 401", func(t *testing.T) {
		params := &RegisterFleetParams{
			ActivationToken: "test-token",
		}

		statusCode, _, err := registerFleet(ts.URL, client, "", params)
		if err != nil {
			t.Fatalf("couldn't call register fleet: %s", err)
		}

		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
		}
	})

	t.Run("2. Register fleet with invalid JSON returns 400", func(t *testing.T) {
		statusCode, response, err := registerFleetRaw(ts.URL, client, adminToken, "not-json")
		if err != nil {
			t.Fatalf("couldn't call register fleet: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if response.Error == "" {
			t.Fatalf("expected error message, got empty string")
		}
	})

	t.Run("3. Register fleet with empty activation token returns 400", func(t *testing.T) {
		params := &RegisterFleetParams{
			ActivationToken: "",
		}

		statusCode, response, err := registerFleet(ts.URL, client, adminToken, params)
		if err != nil {
			t.Fatalf("couldn't call register fleet: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if response.Error != "activationToken is missing" {
			t.Fatalf("expected error 'activationToken is missing', got %q", response.Error)
		}
	})

	t.Run("4. Register fleet with empty body returns 400", func(t *testing.T) {
		statusCode, response, err := registerFleetRaw(ts.URL, client, adminToken, "{}")
		if err != nil {
			t.Fatalf("couldn't call register fleet: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if response.Error != "activationToken is missing" {
			t.Fatalf("expected error 'activationToken is missing', got %q", response.Error)
		}
	})

	t.Run("5. Register fleet with valid token fails because fleet server is unreachable", func(t *testing.T) {
		params := &RegisterFleetParams{
			ActivationToken: "valid-activation-token",
		}

		statusCode, response, err := registerFleet(ts.URL, client, adminToken, params)
		if err != nil {
			t.Fatalf("couldn't call register fleet: %s", err)
		}

		// The handler should return 500 because the fleet server at 127.0.0.1:5003 is not running
		if statusCode != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, statusCode)
		}

		if response.Error != "Failed to register to fleet" {
			t.Fatalf("expected error 'Failed to register to fleet', got %q", response.Error)
		}
	})
}

func TestApiFleetAuthorization(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}

	defer ts.Close()

	client := ts.Client()

	adminToken, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	readOnlyToken, err := createUserAndLogin(ts.URL, adminToken, "readonly@ellanetworks.com", RoleReadOnly, client)
	if err != nil {
		t.Fatalf("couldn't create read-only user and login: %s", err)
	}

	networkManagerToken, err := createUserAndLogin(ts.URL, adminToken, "netmanager@ellanetworks.com", RoleNetworkManager, client)
	if err != nil {
		t.Fatalf("couldn't create network manager user and login: %s", err)
	}

	params := &RegisterFleetParams{
		ActivationToken: "test-token",
	}

	t.Run("1. ReadOnly user cannot register fleet", func(t *testing.T) {
		statusCode, response, err := registerFleet(ts.URL, client, readOnlyToken, params)
		if err != nil {
			t.Fatalf("couldn't call register fleet: %s", err)
		}

		if statusCode != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, statusCode)
		}

		if response.Error != "Forbidden" {
			t.Fatalf("expected error 'Forbidden', got %q", response.Error)
		}
	})

	t.Run("2. NetworkManager user cannot register fleet", func(t *testing.T) {
		statusCode, response, err := registerFleet(ts.URL, client, networkManagerToken, params)
		if err != nil {
			t.Fatalf("couldn't call register fleet: %s", err)
		}

		if statusCode != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, statusCode)
		}

		if response.Error != "Forbidden" {
			t.Fatalf("expected error 'Forbidden', got %q", response.Error)
		}
	})
}
