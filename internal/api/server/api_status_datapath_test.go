// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
)

func statusHandler(t *testing.T, mode func() string) http.Handler {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "db.sqlite3")

	dbInstance, err := db.NewDatabaseWithoutRaft(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	t.Cleanup(func() { _ = dbInstance.Close() })

	ready := &atomic.Bool{}
	ready.Store(true)

	return server.GetStatus(dbInstance, ready, mode)
}

func statusBody(t *testing.T, handler http.Handler) map[string]any {
	t.Helper()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/status", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/status: want 200, got %d (body=%s)", rec.Code, rec.Body.String())
	}

	var body struct {
		Result map[string]any `json:"result"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode status: %v (body=%s)", err, rec.Body.String())
	}

	return body.Result
}

// The reported mode is the one attached with, not the one configured: the
// default chain falls back from driver-level XDP to TCX.
func TestStatusReportsEffectiveDatapathAttachMode(t *testing.T) {
	handler := statusHandler(t, func() string { return config.DatapathTCX })

	if got := statusBody(t, handler)["datapathAttachMode"]; got != config.DatapathTCX {
		t.Errorf("datapathAttachMode = %v, want %q", got, config.DatapathTCX)
	}
}

// Status answers before the datapath attaches, and omits the field then.
func TestStatusOmitsDatapathAttachModeBeforeAttach(t *testing.T) {
	for name, mode := range map[string]func() string{
		"no datapath":      nil,
		"not attached yet": func() string { return "" },
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := statusBody(t, statusHandler(t, mode))["datapathAttachMode"]; ok {
				t.Error("datapathAttachMode is present before the datapath attached")
			}
		})
	}
}
