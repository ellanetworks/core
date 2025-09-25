package server_test

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitializeInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
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
			statusCode, response, err := initializeAPI(ts.URL, client, initializeParams)
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
