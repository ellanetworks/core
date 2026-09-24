// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	hraft "github.com/hashicorp/raft"
)

// TestClusterPropose_HappyPath runs a real single-node Raft cluster
// through the handler and asserts the command committed and returned
// the right envelope.
func TestClusterPropose_HappyPath(t *testing.T) {
	testDB := newTestDB(t)

	payload, err := json.Marshal(map[string]int64{"value": 30})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	envelope, err := json.Marshal(ellaraft.ProposeForwardRequest{
		Operation: "DeleteOldDailyUsage",
		Payload:   payload,
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, ellaraft.ProposeForwardPath, bytes.NewReader(envelope))
	req.Header.Set("Content-Type", ellaraft.ProposeForwardContentType)

	w := httptest.NewRecorder()
	ClusterPropose(testDB).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var env ellaraft.ProposeForwardResponse
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if env.Index == 0 {
		t.Fatalf("index must be non-zero: %+v", env)
	}
}

// TestClusterPropose_NotLeader covers the common-case misroute: a
// follower (or a standalone DB that never elected) receives the forward
// and must return 421 so the caller retries elsewhere.
func TestClusterPropose_NotLeader(t *testing.T) {
	cfg := ellaraft.FastTestConfig()
	cfg.Enabled = true
	cfg.Bootstrap = false
	cfg.RaftID = "11111111-1111-1111-1111-111111111111"
	cfg.BindAddress = "127.0.0.1:0"
	cfg.AdvertiseAddress = "127.0.0.1:0"

	testDB, err := db.NewDatabase(t.Context(), filepath.Join(t.TempDir(), "test.db"), cfg)
	if err != nil {
		t.Fatalf("create follower db: %v", err)
	}

	t.Cleanup(func() { _ = testDB.Close() })

	if testDB.IsLeader() {
		t.Fatal("unbootstrapped node must not be leader")
	}

	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, ellaraft.ProposeForwardPath, bytes.NewReader([]byte{0, 0}))

	w := httptest.NewRecorder()
	ClusterPropose(testDB).ServeHTTP(w, req)

	if w.Code != http.StatusMisdirectedRequest {
		t.Fatalf("expected 421, got %d", w.Code)
	}

	var env ellaraft.ProposeForwardErrorBody
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !strings.Contains(env.Message, "not the leader") {
		t.Fatalf("expected not-leader message, got %q", env.Message)
	}
}

func TestClusterPropose_BadRequest(t *testing.T) {
	testDB := newTestDB(t)

	for _, tc := range []struct {
		name        string
		body        []byte
		wantMessage string
	}{
		{"truncated envelope", []byte(`{"operation":"DeleteOldDailyUsage","payload":`), "invalid envelope"},
		{"malformed payload", []byte(`{"operation":"DeleteOldDailyUsage","payload":{"value":}}`), "invalid envelope"},
		{"empty operation", []byte(`{"operation":""}`), "empty operation"},
		{"unknown operation", []byte(`{"operation":"DefinitelyNotARegisteredOp","payload":{}}`), "unknown operation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(),
				http.MethodPost, ellaraft.ProposeForwardPath, bytes.NewReader(tc.body))

			w := httptest.NewRecorder()
			ClusterPropose(testDB).ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
			}

			var env ellaraft.ProposeForwardErrorBody
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("decode: %v", err)
			}

			if !strings.HasPrefix(env.Message, tc.wantMessage) {
				t.Fatalf("expected message starting with %q, got %q", tc.wantMessage, env.Message)
			}
		})
	}
}

func TestClusterPropose_BodyTooLarge(t *testing.T) {
	testDB := newTestDB(t)

	// One byte over the cap is enough to reject; we don't need a full
	// MaxProposeForwardBodyBytes buffer for correctness.
	oversize := make([]byte, ellaraft.MaxProposeForwardBodyBytes+1)

	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, ellaraft.ProposeForwardPath, bytes.NewReader(oversize))

	w := httptest.NewRecorder()
	ClusterPropose(testDB).ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", w.Code)
	}
}

func TestMapApplyErrorToHTTP(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      error
		want     int
		wantCode string
	}{
		{"unknown operation", db.ErrUnknownOperation, http.StatusBadRequest, ""},
		{"not leader", hraft.ErrNotLeader, http.StatusMisdirectedRequest, ""},
		{"leadership lost", hraft.ErrLeadershipLost, http.StatusConflict, ellaraft.ForwardCodeOutcomeUnknown},
		{"enqueue timeout", hraft.ErrEnqueueTimeout, http.StatusServiceUnavailable, ""},
		{"raft shutdown", hraft.ErrRaftShutdown, http.StatusServiceUnavailable, ""},
		{"propose timeout", fmt.Errorf("%w: barrier", db.ErrProposeTimeout), http.StatusServiceUnavailable, ""},
		{"migration pending", db.ErrMigrationPending, http.StatusServiceUnavailable, ellaraft.ForwardCodeMigrationPend},
		{"already exists", fmt.Errorf("insert: %w", db.ErrAlreadyExists), http.StatusConflict, ellaraft.ForwardCodeAlreadyExists},
		{"not found", fmt.Errorf("update: %w", db.ErrNotFound), http.StatusConflict, ellaraft.ForwardCodeNotFound},
		{"unclassified", errors.New("boom"), http.StatusInternalServerError, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			mapApplyErrorToHTTP(context.Background(), w, tc.err)

			if w.Code != tc.want {
				t.Fatalf("status for %v: want %d, got %d", tc.err, tc.want, w.Code)
			}

			var body ellaraft.ProposeForwardErrorBody
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error body: %v", err)
			}

			if body.Code != tc.wantCode {
				t.Fatalf("code for %v: want %q, got %q", tc.err, tc.wantCode, body.Code)
			}
		})
	}
}
