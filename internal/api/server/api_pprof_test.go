package server_test

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"testing"
)

func getPprof(url string, client *http.Client, token string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/pprof", nil)
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

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, nil, err
	}

	return res.StatusCode, bodyBytes, nil
}

func TestGetPprof_Authorized(t *testing.T) {
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

	statusCode, resp, err := getPprof(ts.URL, client, token)
	if err != nil {
		t.Fatalf("couldn't list radio events: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status code %d, got %d", http.StatusOK, statusCode)
	}

	if len(resp) == 0 {
		t.Fatalf("expected non-empty response body")
	}
}

func TestGetPprof_Unauthorized(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}

	defer ts.Close()

	client := ts.Client()

	statusCode, _, err := getPprof(ts.URL, client, "invalid-token")
	if err != nil {
		t.Fatalf("couldn't list radio events: %s", err)
	}

	if statusCode != http.StatusUnauthorized {
		t.Fatalf("expected status code %d, got %d", http.StatusUnauthorized, statusCode)
	}
}
