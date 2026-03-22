package server_test

import (
	"net/http"
	"path/filepath"
	"testing"
)

func BenchmarkLoginHandler(b *testing.B) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		b.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	user := &CreateUserParams{
		Email:    FirstUserEmail,
		Password: "password123",
		RoleID:   RoleAdmin,
	}

	statusCode, _, err := createUser(env.Server.URL, client, "", user)
	if err != nil {
		b.Fatalf("couldn't create user: %s", err)
	}

	if statusCode != http.StatusCreated {
		b.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
	}

	loginData := &LoginParams{
		Email:    FirstUserEmail,
		Password: "password123",
	}

	for b.Loop() {
		code, _, err := login(env.Server.URL, client, loginData)
		if err != nil {
			b.Fatalf("login failed: %s", err)
		}

		if code != http.StatusOK {
			b.Fatalf("unexpected status code: got %d, want %d", code, http.StatusOK)
		}
	}
}
