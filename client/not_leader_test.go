// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ellanetworks/core/client"
)

func clientAgainst(t *testing.T, status int, body string) *client.Client {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))

	t.Cleanup(srv.Close)

	c, err := client.New(&client.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	return c
}

func TestNotLeaderErrorCarriesTheLeaderHint(t *testing.T) {
	c := clientAgainst(t, http.StatusMisdirectedRequest, `{
		"error": "This node is not the cluster leader; retry against node node-a (1111) at 10.0.0.1:5002",
		"leaderNodeId": "1111",
		"leaderAPIAddress": "10.0.0.1:5002"
	}`)

	err := c.PromoteClusterMember(context.Background(), "2222")

	var notLeader *client.NotLeaderError
	if !errors.As(err, &notLeader) {
		t.Fatalf("err = %v, want a *client.NotLeaderError", err)
	}

	if notLeader.LeaderNodeID != "1111" {
		t.Errorf("LeaderNodeID = %q, want 1111", notLeader.LeaderNodeID)
	}

	if notLeader.LeaderAPIAddress != "10.0.0.1:5002" {
		t.Errorf("LeaderAPIAddress = %q, want 10.0.0.1:5002", notLeader.LeaderAPIAddress)
	}

	if notLeader.StatusCode != http.StatusMisdirectedRequest {
		t.Errorf("StatusCode = %d, want %d", notLeader.StatusCode, http.StatusMisdirectedRequest)
	}
}

func TestNotLeaderErrorWithoutAnElectedLeader(t *testing.T) {
	c := clientAgainst(t, http.StatusMisdirectedRequest,
		`{"error": "This node is not the cluster leader; no leader is currently elected, retry shortly"}`)

	err := c.PromoteClusterMember(context.Background(), "2222")

	var notLeader *client.NotLeaderError
	if !errors.As(err, &notLeader) {
		t.Fatalf("err = %v, want a *client.NotLeaderError", err)
	}

	if notLeader.LeaderNodeID != "" || notLeader.LeaderAPIAddress != "" {
		t.Errorf("got hint %q at %q, want none", notLeader.LeaderNodeID, notLeader.LeaderAPIAddress)
	}
}

func TestOtherErrorsAreNotNotLeaderErrors(t *testing.T) {
	c := clientAgainst(t, http.StatusForbidden, `{"error": "forbidden"}`)

	err := c.PromoteClusterMember(context.Background(), "2222")

	var notLeader *client.NotLeaderError
	if errors.As(err, &notLeader) {
		t.Fatalf("err = %v, want a plain error", err)
	}

	if err == nil {
		t.Fatal("expected an error")
	}
}
