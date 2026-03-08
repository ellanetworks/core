package server_test

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAPISpecEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := newTestClient(ts)

	t.Run("Returns valid YAML with correct content type", func(t *testing.T) {
		req, err := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/v1/openapi.yaml", nil)
		if err != nil {
			t.Fatalf("couldn't create request: %s", err)
		}

		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("couldn't send request: %s", err)
		}

		defer func() {
			if err := res.Body.Close(); err != nil {
				t.Fatal(err)
			}
		}()

		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, res.StatusCode)
		}

		contentType := res.Header.Get("Content-Type")
		if contentType != "application/openapi+yaml" {
			t.Fatalf("expected content type application/openapi+yaml, got %s", contentType)
		}

		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("couldn't read body: %s", err)
		}

		bodyStr := string(body)
		if !strings.Contains(bodyStr, "openapi:") {
			t.Fatal("response body does not contain 'openapi:' key")
		}

		if !strings.Contains(bodyStr, "Ella Core API") {
			t.Fatal("response body does not contain expected title 'Ella Core API'")
		}

		if !strings.Contains(bodyStr, "/api/v1/subscribers") {
			t.Fatal("response body does not contain expected path '/api/v1/subscribers'")
		}
	})

	t.Run("Does not require authentication", func(t *testing.T) {
		req, err := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/v1/openapi.yaml", nil)
		if err != nil {
			t.Fatalf("couldn't create request: %s", err)
		}

		// Explicitly no Authorization header
		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("couldn't send request: %s", err)
		}

		defer func() {
			if err := res.Body.Close(); err != nil {
				t.Fatal(err)
			}
		}()

		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected unauthenticated access to return %d, got %d", http.StatusOK, res.StatusCode)
		}
	})
}
