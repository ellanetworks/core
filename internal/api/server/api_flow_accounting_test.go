// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
)

type GetFlowAccountingInfoResponseResult struct {
	Enabled bool `json:"enabled"`
}

type GetFlowAccountingInfoResponse struct {
	Result GetFlowAccountingInfoResponseResult `json:"result"`
	Error  string                              `json:"error,omitempty"`
}

type UpdateFlowAccountingInfoParams struct {
	Enabled bool `json:"enabled"`
}

type UpdateFlowAccountingInfoResponseResult struct {
	Message string `json:"message"`
}

type UpdateFlowAccountingInfoResponse struct {
	Result UpdateFlowAccountingInfoResponseResult `json:"result"`
	Error  string                                 `json:"error,omitempty"`
}

func getFlowAccountingInfo(url string, client *http.Client, token string) (int, *GetFlowAccountingInfoResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/networking/flow-accounting", nil)
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

	var response GetFlowAccountingInfoResponse

	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func updateFlowAccountingInfo(url string, client *http.Client, token string, enabled bool) (int, *UpdateFlowAccountingInfoResponse, error) {
	params := UpdateFlowAccountingInfoParams{
		Enabled: enabled,
	}

	payloadBytes, err := json.Marshal(params)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/networking/flow-accounting", bytes.NewReader(payloadBytes))
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

	var response UpdateFlowAccountingInfoResponse

	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func TestApiFlowAccountingInfoEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}

	defer ts.Close()

	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Get flow accounting info (default)", func(t *testing.T) {
		statusCode, response, err := getFlowAccountingInfo(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get flow accounting info: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected no error, got %s", response.Error)
		}

		if !response.Result.Enabled {
			t.Fatalf("expected flow accounting to be enabled by default")
		}
	})

	t.Run("2. Update flow accounting info to disable", func(t *testing.T) {
		statusCode, response, err := updateFlowAccountingInfo(ts.URL, client, token, false)
		if err != nil {
			t.Fatalf("couldn't update flow accounting info: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected no error, got %s", response.Error)
		}

		if response.Result.Message != "Flow accounting settings updated successfully" {
			t.Fatalf("unexpected message: %s", response.Result.Message)
		}
	})

	t.Run("3. Get flow accounting info (disabled)", func(t *testing.T) {
		statusCode, response, err := getFlowAccountingInfo(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get flow accounting info: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected no error, got %s", response.Error)
		}

		if response.Result.Enabled {
			t.Fatalf("expected flow accounting to be disabled")
		}
	})
}
