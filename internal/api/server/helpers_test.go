// Copyright 2026 Ella Networks

package server

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestGetActorFromContext_ClusterPortUsesNodeID(t *testing.T) {
	ctx := context.WithValue(context.Background(), peerNodeIDCtxKey{}, 5)
	ctx = context.WithValue(ctx, contextKeyEmail, "alice@example.com")

	req := httptest.NewRequestWithContext(ctx, "POST", "/cluster/members", nil)

	got := getActorFromContext(req)
	if want := "ella-node-5"; got != want {
		t.Fatalf("cluster-port actor: got %q, want %q (node-id must take precedence over email)", got, want)
	}
}

func TestGetActorFromContext_APIPortFallsBackToEmail(t *testing.T) {
	ctx := context.WithValue(context.Background(), contextKeyEmail, "alice@example.com")

	req := httptest.NewRequestWithContext(ctx, "POST", "/api/v1/cluster/members", nil)

	got := getActorFromContext(req)
	if want := "alice@example.com"; got != want {
		t.Fatalf("API-port actor: got %q, want %q", got, want)
	}
}

func TestGetActorFromContext_NoIdentityReturnsEmpty(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)

	if got := getActorFromContext(req); got != "" {
		t.Fatalf("no-identity actor: got %q, want empty", got)
	}
}
