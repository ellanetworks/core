// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

// runAuditLogsHAMatrix triggers a known mutation on one node and asserts
// the resulting audit entry is visible only on the node that served the
// request. audit_logs is a local-only table: each node records what it
// served, and the rows do not replicate through Raft.
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

		if i > 0 {
			if found != nil {
				t.Fatalf("node %d holds an audit entry for canary %q; audit_logs is local-only and must not replicate",
					i+1, canaryEmail)
			}

			continue
		}

		if found == nil {
			t.Fatalf("node 1 audit log for canary %q not found (page returned %d items, totalCount %d)",
				canaryEmail, len(logs.Items), logs.TotalCount)
		}

		if found.Action != "create_user" {
			t.Fatalf("node 1 Action: got %q, want %q", found.Action, "create_user")
		}

		if found.User != adminEmail {
			t.Fatalf("node 1 User (actor): got %q, want %q", found.User, adminEmail)
		}

		if found.ID == "" {
			t.Fatalf("node 1 ID: got empty, want non-empty")
		}

		if _, err := time.Parse(time.RFC3339, found.Timestamp); err != nil {
			t.Fatalf("node 1 Timestamp: not RFC 3339: %q (%v)", found.Timestamp, err)
		}
	}
}
