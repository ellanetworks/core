package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const (
	Mcc = "123"
	Mnc = "456"
)

type GetNetworkResponseResult struct {
	Mcc string `json:"mcc,omitempty"`
	Mnc string `json:"mnc,omitempty"`
}

type GetNetworkResponse struct {
	Result GetNetworkResponseResult `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

type UpdateNetworkParams struct {
	Mcc string `json:"mcc,omitempty"`
	Mnc string `json:"mnc,omitempty"`
}

type UpdateNetworkResponseResult struct {
	Message string `json:"message"`
}

type UpdateNetworkResponse struct {
	Result UpdateNetworkResponseResult `json:"result"`
	Error  string                      `json:"error,omitempty"`
}

func getNetwork(url string, client *http.Client, token string) (int, *GetNetworkResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/network", nil)
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
	var networkSliceResponse GetNetworkResponse
	if err := json.NewDecoder(res.Body).Decode(&networkSliceResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &networkSliceResponse, nil
}

func updateNetwork(url string, client *http.Client, token string, data *UpdateNetworkParams) (int, *UpdateNetworkResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/network", strings.NewReader(string(body)))
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
	var createResponse UpdateNetworkResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

// This is an end-to-end test for the networks handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestApiNetworksEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(db_path)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Update network", func(t *testing.T) {
		updateNetworkParams := &UpdateNetworkParams{
			Mcc: "123",
			Mnc: "456",
		}
		statusCode, response, err := updateNetwork(ts.URL, client, token, updateNetworkParams)
		if err != nil {
			t.Fatalf("couldn't create network: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "Network updated successfully" {
			t.Fatalf("expected message %q, got %q", "Network updated successfully", response.Result.Message)
		}
	})

	t.Run("2. Get network", func(t *testing.T) {
		statusCode, response, err := getNetwork(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get network: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Mcc != "123" {
			t.Fatalf("expected mcc %s, got %s", "123", response.Result.Mcc)
		}
		if response.Result.Mnc != "456" {
			t.Fatalf("expected mnc %s, got %s", "456", response.Result.Mnc)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("4. Update network - no mnc", func(t *testing.T) {
		updateNetworkParams := &UpdateNetworkParams{
			Mcc: "123",
		}
		statusCode, response, err := updateNetwork(ts.URL, client, token, updateNetworkParams)
		if err != nil {
			t.Fatalf("couldn't create network: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
		if response.Error != "mnc is missing" {
			t.Fatalf("expected error %q, got %q", "mnc is missing", response.Error)
		}
	})
}

func TestUpdateNetworkInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(db_path)
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
		testName string
		mcc      string
		mnc      string
		error    string
	}{
		{
			testName: "Invalid mcc - strings instead of numbers",
			mcc:      "abc",
			mnc:      Mnc,
			error:    "Invalid mcc format. Must be a 3-decimal digit.",
		},
		{
			testName: "Invalid mcc - too long",
			mcc:      "1234",
			mnc:      Mnc,
			error:    "Invalid mcc format. Must be a 3-decimal digit.",
		},
		{
			testName: "Invalid mcc - too short",
			mcc:      "12",
			mnc:      Mnc,
			error:    "Invalid mcc format. Must be a 3-decimal digit.",
		},
		{
			testName: "Invalid mnc - strings instead of numbers",
			mcc:      Mcc,
			mnc:      "abc",
			error:    "Invalid mnc format. Must be a 2 or 3-decimal digit.",
		},
		{
			testName: "Invalid mnc - too long",
			mcc:      Mcc,
			mnc:      "1234",
			error:    "Invalid mnc format. Must be a 2 or 3-decimal digit.",
		},
		{
			testName: "Invalid mnc - too short",
			mcc:      Mcc,
			mnc:      "1",
			error:    "Invalid mnc format. Must be a 2 or 3-decimal digit.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			updateNetworkParams := &UpdateNetworkParams{
				Mcc: tt.mcc,
				Mnc: tt.mnc,
			}
			statusCode, response, err := updateNetwork(ts.URL, client, token, updateNetworkParams)
			if err != nil {
				t.Fatalf("couldn't update network: %s", err)
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
