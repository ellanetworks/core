package server_test

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
)

type GetStatusResponseResult struct {
	Version string `json:"version"`
}

type GetStatusResponse struct {
	Error  string                  `json:"error,omitempty"`
	Result GetStatusResponseResult `json:"result"`
}

func getStatus(url string, client *http.Client) (int, *GetStatusResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/status", nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var radioResponse GetStatusResponse
	if err := json.NewDecoder(res.Body).Decode(&radioResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &radioResponse, nil
}

// This is an end-to-end test for the status handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestStatusEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, err := setupServer(db_path)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("1. Get status", func(t *testing.T) {
		statusCode, response, err := getStatus(ts.URL, client)
		if err != nil {
			t.Fatalf("couldn't get status: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Version == "" {
			t.Fatalf("expected version to be non-empty")
		}
	})
}
