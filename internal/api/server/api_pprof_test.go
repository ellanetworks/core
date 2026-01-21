package server_test

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"testing"
)

func getPprof(url string, client *http.Client, token string, endpoint string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/pprof"+endpoint, nil)
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

	tests := []struct {
		testName string
		endpoint string
	}{
		{
			testName: "Root pprof endpoint",
			endpoint: "",
		},
		{
			testName: "Cmdline endpoint",
			endpoint: "/cmdline",
		},
		{
			testName: "Profile endpoint",
			endpoint: "/profile?seconds=1",
		},
		{
			testName: "Symbol endpoint",
			endpoint: "/symbol",
		},
		{
			testName: "Trace endpoint",
			endpoint: "/trace?seconds=1",
		},
		{
			testName: "allocs endpoint",
			endpoint: "/allocs",
		},
		{
			testName: "block endpoint",
			endpoint: "/block",
		},
		{
			testName: "goroutine endpoint",
			endpoint: "/goroutine",
		},
		{
			testName: "heap endpoint",
			endpoint: "/heap",
		},
		{
			testName: "mutex endpoint",
			endpoint: "/mutex",
		},
		{
			testName: "threadcreate endpoint",
			endpoint: "/threadcreate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			statusCode, bodyBytes, err := getPprof(ts.URL, client, token, tt.endpoint)
			if err != nil {
				t.Fatalf("couldn't do request: %s", err)
			}

			if statusCode != http.StatusOK {
				t.Fatalf("expected status code %d, got %d", http.StatusOK, statusCode)
			}

			if len(bodyBytes) == 0 {
				t.Fatalf("expected non-empty response body")
			}
		})
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

	tests := []struct {
		testName string
		endpoint string
	}{
		{
			testName: "Root pprof endpoint",
			endpoint: "",
		},
		{
			testName: "Cmdline endpoint",
			endpoint: "/cmdline",
		},
		{
			testName: "Profile endpoint",
			endpoint: "/profile?seconds=1",
		},
		{
			testName: "Symbol endpoint",
			endpoint: "/symbol",
		},
		{
			testName: "Trace endpoint",
			endpoint: "/trace?seconds=1",
		},
		{
			testName: "allocs endpoint",
			endpoint: "/allocs",
		},
		{
			testName: "block endpoint",
			endpoint: "/block",
		},
		{
			testName: "goroutine endpoint",
			endpoint: "/goroutine",
		},
		{
			testName: "heap endpoint",
			endpoint: "/heap",
		},
		{
			testName: "mutex endpoint",
			endpoint: "/mutex",
		},
		{
			testName: "threadcreate endpoint",
			endpoint: "/threadcreate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			statusCode, _, err := getPprof(ts.URL, client, "invalid-token", tt.endpoint)
			if err != nil {
				t.Fatalf("couldn't do request: %s", err)
			}

			if statusCode != http.StatusUnauthorized {
				t.Fatalf("expected status code %d, got %d", http.StatusUnauthorized, statusCode)
			}
		})
	}
}
