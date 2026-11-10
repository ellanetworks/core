// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/db"
)

func healthDB(t *testing.T) *db.Database {
	t.Helper()

	dbInstance, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	t.Cleanup(func() { _ = dbInstance.Close() })

	return dbInstance
}

func getReadyz(t *testing.T, handler http.Handler, target string) (int, http.Header, server.Readiness) {
	t.Helper()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, nil))

	var body struct {
		Result server.Readiness `json:"result"`
	}

	if rec.Code != http.StatusBadRequest {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode readyz: %v (body=%s)", err, rec.Body.String())
		}
	}

	return rec.Code, rec.Header(), body.Result
}

func checkByName(t *testing.T, res server.Readiness, name string) server.Check {
	t.Helper()

	for _, c := range res.Checks {
		if c.Name == name {
			return c
		}
	}

	t.Fatalf("check %q missing from %+v", name, res.Checks)

	return server.Check{}
}

func TestLivez(t *testing.T) {
	rec := httptest.NewRecorder()
	server.Livez().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/livez", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rec.Code)
	}

	if rec.Body.Len() != 0 {
		t.Fatalf("want empty body, got %q", rec.Body.String())
	}
}

func TestReadyz_StandaloneReady(t *testing.T) {
	handler := server.Readyz(context.Background(), healthDB(t), false)

	code, header, res := getReadyz(t, handler, "/api/v1/readyz")

	if code != http.StatusOK {
		t.Fatalf("want 200, got %d (%+v)", code, res)
	}

	if !res.Ready {
		t.Fatalf("want ready, got %+v", res)
	}

	if got := header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("want Cache-Control no-store, got %q", got)
	}

	want := []string{
		server.CheckShutdown,
		server.CheckMember,
		server.CheckInitialized,
		server.CheckDrain,
		server.CheckDatabase,
		server.CheckLeader,
	}

	if len(res.Checks) != len(want) {
		t.Fatalf("want %d checks, got %+v", len(want), res.Checks)
	}

	for i, name := range want {
		if res.Checks[i].Name != name || !res.Checks[i].OK {
			t.Fatalf("check %d: want %q ok, got %+v", i, name, res.Checks[i])
		}
	}
}

func TestReadyz_Discovery(t *testing.T) {
	handler := server.Readyz(context.Background(), healthDB(t), true)

	code, _, res := getReadyz(t, handler, "/api/v1/readyz")

	if code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", code)
	}

	if c := checkByName(t, res, server.CheckMember); c.OK || c.Detail == "" {
		t.Fatalf("want member to fail with a detail, got %+v", c)
	}
}

func TestReadyz_ShuttingDown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := server.Readyz(ctx, healthDB(t), false)

	code, _, res := getReadyz(t, handler, "/api/v1/readyz")

	if code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", code)
	}

	if c := checkByName(t, res, server.CheckShutdown); c.OK {
		t.Fatalf("want shutdown to fail, got %+v", c)
	}
}

func TestReadyz_ClusterParam(t *testing.T) {
	handler := server.Readyz(context.Background(), healthDB(t), false)

	code, _, res := getReadyz(t, handler, "/api/v1/readyz?cluster=true")

	if code != http.StatusOK {
		t.Fatalf("want 200, got %d (%+v)", code, res)
	}

	if c := checkByName(t, res, server.CheckCluster); !c.OK {
		t.Fatalf("want cluster to pass on a standalone node, got %+v", c)
	}

	code, _, _ = getReadyz(t, handler, "/api/v1/readyz?cluster=maybe")

	if code != http.StatusBadRequest {
		t.Fatalf("want 400 for an invalid cluster value, got %d", code)
	}
}

func TestReadyz_DatabaseClosed(t *testing.T) {
	dbInstance := healthDB(t)

	if err := dbInstance.PlainDB().Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	code, _, res := getReadyz(t, server.Readyz(context.Background(), dbInstance, false), "/api/v1/readyz")

	if code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", code)
	}

	if c := checkByName(t, res, server.CheckDatabase); c.OK {
		t.Fatalf("want database to fail, got %+v", c)
	}
}
