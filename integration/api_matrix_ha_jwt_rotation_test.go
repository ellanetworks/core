// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runJWTRotationHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	login := func(node int) *client.Client {
		clients, err := newHANodeClients()
		if err != nil {
			t.Fatalf("new clients: %v", err)
		}

		c := clients[node]

		if err := c.Login(ctx, &client.LoginOptions{Email: "admin@ellanetworks.com", Password: "admin"}); err != nil {
			t.Fatalf("login on node %d: %v", node+1, err)
		}

		return c
	}

	validOn := func(node int, token string) bool {
		clients, err := newHANodeClients()
		if err != nil {
			t.Fatalf("new clients: %v", err)
		}

		probe := clients[node]
		probe.SetToken(token)

		res, err := probe.LookupToken(ctx)

		return err == nil && res.Valid
	}

	before := login(0)
	oldToken := before.GetToken()

	for i := range h.Clients {
		if !validOn(i, oldToken) {
			t.Fatalf("a JWT issued by node 1 is rejected by node %d before any rotation", i+1)
		}
	}

	if err := h.Clients[1].RotateSecret(ctx); err != nil {
		t.Fatalf("rotate secret on node 2: %v", err)
	}

	awaitConvergence(ctx, t, h)

	for i := range h.Clients {
		if validOn(i, oldToken) {
			t.Errorf("node %d still accepts a JWT signed with the rotated-out secret", i+1)
		}
	}

	if err := before.Refresh(ctx); err == nil {
		t.Errorf("node 1 refreshed a session that the rotation on node 2 must have invalidated")
	}

	after := login(2)
	newToken := after.GetToken()

	for i := range h.Clients {
		if !validOn(i, newToken) {
			t.Errorf("node %d rejects a JWT issued by node 3 after the rotation", i+1)
		}
	}
}
