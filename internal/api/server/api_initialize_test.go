package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type InitializeResponseResult struct {
	Message string `json:"message"`
	Token   string `json:"token"`
}

type InitializeResponse struct {
	Result InitializeResponseResult `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

type InitializeParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func initialize(url string, client *http.Client, data *InitializeParams) (int, *InitializeResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/init", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return res.StatusCode, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var initResponse InitializeResponse

	if err := json.NewDecoder(res.Body).Decode(&initResponse); err != nil {
		return res.StatusCode, nil, err
	}

	return res.StatusCode, &initResponse, nil
}

func TestInitializeInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	tests := []struct {
		email    string
		password string
		error    string
	}{
		{
			email:    strings.Repeat("a", 257),
			password: Password,
			error:    "Invalid email format",
		},
		{
			email:    "abcdef",
			password: Password,
			error:    "Invalid email format",
		},
		{
			email:    "abcdef@",
			password: Password,
			error:    "Invalid email format",
		},
		{
			email:    "abcdef@gmail.",
			password: Password,
			error:    "Invalid email format",
		},
		{
			email:    "abcd@ef@ellanetworks.com",
			password: Password,
			error:    "Invalid email format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			initializeParams := &InitializeParams{
				Email:    tt.email,
				Password: tt.password,
			}

			statusCode, response, err := initialize(ts.URL, client, initializeParams)
			if err != nil {
				t.Fatalf("couldn't create user: %s", err)
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
