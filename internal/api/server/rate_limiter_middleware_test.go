package server_test

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/gin-gonic/gin"
)

func TestRateLimiterMiddleware(t *testing.T) {
	tempDir := t.TempDir()
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(db_path, gin.ReleaseMode)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	_, err = createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	loginData := &LoginParams{
		Email:    "my.user123@ellanetworks.com",
		Password: "password123",
	}
	server.ResetVisitors()

	for i := 0; i < 100; i++ {
		respCode, _, err := login(ts.URL, client, loginData)
		if err != nil {
			t.Fatalf("couldn't login: %s", err)
		}
		if respCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, respCode)
		}
	}

	respCode, _, err := login(ts.URL, client, loginData)
	if err != nil {
		t.Fatalf("couldn't login: %s", err)
	}
	if respCode != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, respCode)
	}
}
