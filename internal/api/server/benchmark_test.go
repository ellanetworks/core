package server_test

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/api/server"
)

func BenchmarkLoginHandler(b *testing.B) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath, server.TestMode)
	if err != nil {
		b.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	user := &CreateUserParams{
		Email:    "my.user123@ellanetworks.com",
		Password: "password123",
		Role:     "admin",
	}
	statusCode, _, err := createUser(ts.URL, client, "", user)
	if err != nil {
		b.Fatalf("couldn't create user: %s", err)
	}
	if statusCode != http.StatusCreated {
		b.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
	}

	loginData := &LoginParams{
		Email:    "my.user123@ellanetworks.com",
		Password: "password123",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		code, _, err := login(ts.URL, client, loginData)
		if err != nil {
			b.Fatalf("login failed: %s", err)
		}
		if code != http.StatusOK {
			b.Fatalf("unexpected status code: got %d, want %d", code, http.StatusOK)
		}
	}
}
