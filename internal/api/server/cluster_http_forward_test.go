// Copyright 2026 Ella Networks

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

// TestClusterPropose_HappyPath runs a real single-node Raft cluster
// through the handler and asserts the command committed and returned
// the right envelope.
func TestClusterPropose_HappyPath(t *testing.T) {
	testDB := newLeaderTestDB(t)

	cmd, err := ellaraft.NewCommand(ellaraft.CmdDeleteOldAuditLogs, map[string]string{"value": "1970-01-01"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, ellaraft.ProposeForwardPath, bytes.NewReader(data))
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

	if got := w.Header().Get(ellaraft.HeaderAppliedIndex); got == "" {
		t.Fatalf("missing X-Ella-Applied-Index header")
	}
}

// TestClusterPropose_NotLeader covers the common-case misroute: a
// follower (or a standalone DB that never elected) receives the forward
// and must return 421 so the caller retries elsewhere.
func TestClusterPropose_NotLeader(t *testing.T) {
	// Use newTestDB without waiting for leadership; racy but fine —
	// the test re-checks IsLeader below and skips if the race lost.
	testDB := newTestDB(t)

	if testDB.IsLeader() {
		t.Skip("single-node DB already elected; cannot test follower path here")
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

func TestClusterPropose_EmptyBody(t *testing.T) {
	testDB := newLeaderTestDB(t)

	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, ellaraft.ProposeForwardPath, nil)

	w := httptest.NewRecorder()
	ClusterPropose(testDB).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d", w.Code)
	}
}

func TestClusterPropose_ShortBody(t *testing.T) {
	testDB := newLeaderTestDB(t)

	// One byte is below the minimum Command header (2-byte CommandType).
	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, ellaraft.ProposeForwardPath, bytes.NewReader([]byte{0}))

	w := httptest.NewRecorder()
	ClusterPropose(testDB).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for short body, got %d", w.Code)
	}
}

func TestClusterPropose_BodyTooLarge(t *testing.T) {
	testDB := newLeaderTestDB(t)

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

func TestClusterPropose_MalformedPayloadRejected(t *testing.T) {
	testDB := newLeaderTestDB(t)

	// The FSM is fail-stop on apply errors — any command whose payload
	// fails to unmarshal in applyCommand panics the node. Validation
	// in the handler must reject the garbage before raft.Apply so a
	// buggy (or malicious) follower can't crash the leader.
	badCmd := []byte{0x00, 0x00, 0x7b, 0x2f, 0x2f} // type=Changeset, payload="{//"

	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, ellaraft.ProposeForwardPath, bytes.NewReader(badCmd))

	w := httptest.NewRecorder()
	ClusterPropose(testDB).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed payload, got %d body=%s", w.Code, w.Body.String())
	}

	var env ellaraft.ProposeForwardErrorBody
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !strings.Contains(env.Message, "JSON") {
		t.Fatalf("error message should mention JSON validation failure, got %q", env.Message)
	}
}

func TestClusterPropose_UnknownCommandTypeRejected(t *testing.T) {
	testDB := newLeaderTestDB(t)

	// Type 0xFFFF is not in the registered command set; reject at
	// validation rather than letting the FSM return "unknown command
	// type" which fail-stops the node.
	unknown := []byte{0xFF, 0xFF, 0x7b, 0x7d} // type=65535, payload="{}"

	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, ellaraft.ProposeForwardPath, bytes.NewReader(unknown))

	w := httptest.NewRecorder()
	ClusterPropose(testDB).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown command type, got %d", w.Code)
	}
}
