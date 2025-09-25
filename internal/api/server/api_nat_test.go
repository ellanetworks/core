package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
)

type GetNATInfoResponseResult struct {
	Enabled bool `json:"enabled"`
}

type GetNATInfoResponse struct {
	Result GetNATInfoResponseResult `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

type UpdateNATInfoParams struct {
	Enabled bool `json:"enabled"`
}

type UpdateNATInfoResponseResult struct {
	Message string `json:"message"`
}

type UpdateNATInfoResponse struct {
	Result UpdateNATInfoResponseResult `json:"result"`
	Error  string                      `json:"error,omitempty"`
}

func getNATInfo(url string, client *http.Client, token string) (int, *GetNATInfoResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/networking/nat", nil)
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

	var natResponse GetNATInfoResponse

	if err := json.NewDecoder(res.Body).Decode(&natResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &natResponse, nil
}

func updateNATInfo(url string, client *http.Client, token string, enabled bool) (int, *UpdateOperatorSliceResponse, error) {
	params := UpdateNATInfoParams{
		Enabled: enabled,
	}

	payloadBytes, err := json.Marshal(params)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/networking/nat", bytes.NewReader(payloadBytes))
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

	var updateResponse UpdateOperatorSliceResponse

	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &updateResponse, nil
}

func TestApiNATInfoEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}

	defer ts.Close()

	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Get NAT info (default)", func(t *testing.T) {
		statusCode, natResponse, err := getNATInfo(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get NAT info: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if natResponse.Error != "" {
			t.Fatalf("expected no error, got %s", natResponse.Error)
		}

		if !natResponse.Result.Enabled {
			t.Fatalf("expected NAT to be enabled by default")
		}
	})

	t.Run("2. Update NAT info to disable", func(t *testing.T) {
		statusCode, updateResponse, err := updateNATInfo(ts.URL, client, token, false)
		if err != nil {
			t.Fatalf("couldn't update NAT info: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if updateResponse.Error != "" {
			t.Fatalf("expected no error, got %s", updateResponse.Error)
		}

		if updateResponse.Result.Message != "NAT settings updated successfully" {
			t.Fatalf("unexpected message: %s", updateResponse.Result.Message)
		}
	})

	t.Run("3. Get NAT info (disabled)", func(t *testing.T) {
		statusCode, natResponse, err := getNATInfo(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get NAT info: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if natResponse.Error != "" {
			t.Fatalf("expected no error, got %s", natResponse.Error)
		}

		if natResponse.Result.Enabled {
			t.Fatalf("expected NAT to be disabled")
		}
	})
}
