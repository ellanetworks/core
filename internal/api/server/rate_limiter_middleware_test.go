package server_test

import (
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/gin-gonic/gin"
)

func TestRateLimiterMiddleware(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath, gin.ReleaseMode)
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

	var wg sync.WaitGroup
	var successCount int32
	var rateLimitCount int32

	// Fire many concurrent requests.
	totalRequests := 200
	wg.Add(totalRequests)
	for i := 0; i < totalRequests; i++ {
		go func() {
			defer wg.Done()
			respCode, _, err := login(ts.URL, client, loginData)
			if err != nil {
				t.Errorf("login error: %s", err)
				return
			}
			if respCode == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			} else if respCode == http.StatusTooManyRequests {
				atomic.AddInt32(&rateLimitCount, 1)
			}
		}()
	}
	wg.Wait()

	if successCount < 100 {
		t.Fatalf("expected at least 100 successful logins, got %d", successCount)
	}
	if rateLimitCount == 0 {
		t.Fatalf("expected at least one rate limited response, got %d", rateLimitCount)
	}
}
