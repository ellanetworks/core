// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func newDiscoveryTestHandlerWithUpgrade(t *testing.T, apiUpgraded <-chan struct{}) (http.Handler, *db.Database) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "db.sqlite3")

	dbInstance, err := db.NewDatabaseWithoutRaft(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	t.Cleanup(func() { _ = dbInstance.Close() })

	return server.NewDiscoveryHandler(server.DiscoveryHandlerConfig{
		DB:          dbInstance,
		Config:      config.Config{},
		APIUpgraded: apiUpgraded,
	}), dbInstance
}

func newInitRequest(ctx context.Context) *http.Request {
	body := strings.NewReader(`{"email":"admin@ellanetworks.com","password":"ValidPass123!"}`)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/init", body)
	req.Header.Set("Content-Type", "application/json")

	return req
}

func TestDiscoveryHandler_InitWaitsForAPIUpgrade(t *testing.T) {
	apiUpgraded := make(chan struct{})
	handler, _ := newDiscoveryTestHandlerWithUpgrade(t, apiUpgraded)

	done := make(chan *httptest.ResponseRecorder, 1)

	go func() {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, newInitRequest(context.Background()))

		done <- rec
	}()

	select {
	case rec := <-done:
		t.Fatalf("POST /api/v1/init returned before the API upgrade: %d (body=%s)", rec.Code, rec.Body.String())
	case <-time.After(200 * time.Millisecond):
	}

	close(apiUpgraded)

	select {
	case rec := <-done:
		if rec.Code != http.StatusCreated {
			t.Fatalf("POST /api/v1/init: want 201, got %d (body=%s)", rec.Code, rec.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("POST /api/v1/init did not return after the API upgrade")
	}
}

func TestDiscoveryHandler_InitReturns503WithoutCreatingUserWhenNotUpgraded(t *testing.T) {
	handler, dbInstance := newDiscoveryTestHandlerWithUpgrade(t, make(chan struct{}))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newInitRequest(ctx))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("POST /api/v1/init: want 503, got %d (body=%s)", rec.Code, rec.Body.String())
	}

	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Fatalf("Retry-After header missing")
	}

	numUsers, err := dbInstance.CountUsers(context.Background())
	if err != nil {
		t.Fatalf("CountUsers: %v", err)
	}

	if numUsers != 0 {
		t.Fatalf("want no users after a failed init, got %d", numUsers)
	}
}
