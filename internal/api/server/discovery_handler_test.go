package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
)

func newDiscoveryTestHandler(t *testing.T) http.Handler {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "db.sqlite3")

	dbInstance, err := db.NewDatabaseWithoutRaft(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	t.Cleanup(func() { _ = dbInstance.Close() })

	return server.NewDiscoveryHandler(server.DiscoveryHandlerConfig{
		DB:     dbInstance,
		Config: config.Config{},
	})
}

func TestDiscoveryHandler_KnownRouteServed(t *testing.T) {
	handler := newDiscoveryTestHandler(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/status", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/status: want 200, got %d (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestDiscoveryHandler_UnknownRouteIs503(t *testing.T) {
	handler := newDiscoveryTestHandler(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/subscribers/imsi-001019756150000", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("POST /api/v1/subscribers/...: want 503, got %d (body=%s)", rec.Code, rec.Body.String())
	}

	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Fatalf("Retry-After header missing")
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type: want application/json, got %q (body=%s)", got, rec.Body.String())
	}
}
