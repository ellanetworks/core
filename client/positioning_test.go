// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestListPositioningSessions_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"id": "sess-1", "supi": "imsi-001010000000001", "session_type": 1, "method": "dl-tdoa", "status": 2, "created_at": 100, "updated_at": 200}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{Requester: fake}

	sessions, err := clientObj.ListPositioningSessions(context.Background(), "imsi-001010000000001")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	if sessions[0].ID != "sess-1" || sessions[0].SUPI != "imsi-001010000000001" {
		t.Fatalf("unexpected session: %+v", sessions[0])
	}

	if fake.lastOpts.Path != "api/beta/positioning/sessions" {
		t.Fatalf("unexpected path: %s", fake.lastOpts.Path)
	}

	if got := fake.lastOpts.Query.Get("supi"); got != "imsi-001010000000001" {
		t.Fatalf("unexpected supi query: %q", got)
	}
}

func TestListPositioningSessions_EmptySUPI(t *testing.T) {
	clientObj := &client.Client{Requester: &fakeRequester{}}

	if _, err := clientObj.ListPositioningSessions(context.Background(), ""); err == nil {
		t.Fatalf("expected error for empty supi, got none")
	}
}

func TestListPositioningSessions_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 500,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Failed to list sessions"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{Requester: fake}

	if _, err := clientObj.ListPositioningSessions(context.Background(), "imsi-001010000000001"); err == nil {
		t.Fatalf("expected error, got none")
	}
}
