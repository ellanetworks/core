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

	sessions, err := clientObj.ListPositioningSessions(context.Background())
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

	if _, err := clientObj.ListPositioningSessions(context.Background()); err == nil {
		t.Fatalf("expected error, got none")
	}
}
