package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

// runAuditLogsHAMatrix triggers a known mutation on one node and asserts
// the resulting audit entry is visible — with the same ID — on all three
// nodes. The ID match proves the row replicated through Raft, rather
// than each node generating its own.
func runAuditLogsHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	const (
		canaryEmail = "apimat-ha-audit-canary@example.com"
		adminEmail  = "admin@ellanetworks.com"
	)

	nodes := h.Clients

	start := time.Now().UTC().Add(-30 * time.Second).Format(time.RFC3339)

	if err := nodes[0].CreateUser(ctx, &client.CreateUserOptions{
		Email:    canaryEmail,
		RoleID:   client.RoleReadOnly,
		Password: "ApiMatrixPassw0rd!",
	}); err != nil {
		t.Fatalf("create canary user on node 1: %v", err)
	}

	t.Cleanup(func() {
		if err := h.Leader.DeleteUser(ctx, &client.DeleteUserOptions{Email: canaryEmail}); err != nil {
			t.Logf("cleanup: delete canary user: %v", err)
		}
	})

	awaitConvergence(ctx, t, h)

	end := time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339)

	var canonicalID string

	for i, c := range nodes {
		logs, err := c.ListAuditLogs(ctx, &client.ListAuditLogsParams{
			Page:    1,
			PerPage: 100,
			Action:  "create_user",
			Start:   start,
			End:     end,
		})
		if err != nil {
			t.Fatalf("node %d list audit logs: %v", i+1, err)
		}

		found := findAuditLogByDetails(logs.Items, canaryEmail)
		if found == nil {
			t.Fatalf("node %d audit log for canary %q not found (page returned %d items, totalCount %d)",
				i+1, canaryEmail, len(logs.Items), logs.TotalCount)
		}

		if found.Action != "create_user" {
			t.Fatalf("node %d Action: got %q, want %q", i+1, found.Action, "create_user")
		}

		if found.User != adminEmail {
			t.Fatalf("node %d User (actor): got %q, want %q", i+1, found.User, adminEmail)
		}

		if found.ID == "" {
			t.Fatalf("node %d ID: got empty, want non-empty", i+1)
		}

		if _, err := time.Parse(time.RFC3339, found.Timestamp); err != nil {
			t.Fatalf("node %d Timestamp: not RFC 3339: %q (%v)", i+1, found.Timestamp, err)
		}

		if i == 0 {
			canonicalID = found.ID
		}

		if found.ID != canonicalID {
			t.Fatalf("node %d audit log ID: got %q, want %q (entry not replicated, each node generated its own)",
				i+1, found.ID, canonicalID)
		}
	}
}
