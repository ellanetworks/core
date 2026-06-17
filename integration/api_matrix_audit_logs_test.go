// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

// runAuditLogsMatrix triggers a known mutation (create user) and verifies
// the corresponding audit entry surfaces through the ListAuditLogs API
// with the documented shape. Filters by action + time window so the
// matrix's other audit emissions don't interfere.
func runAuditLogsMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	const (
		canaryEmail = "apimat-audit-canary@example.com"
		adminEmail  = "admin@ellanetworks.com"
	)

	start := time.Now().UTC().Add(-30 * time.Second).Format(time.RFC3339)

	if err := c.CreateUser(ctx, &client.CreateUserOptions{
		Email:    canaryEmail,
		RoleID:   client.RoleReadOnly,
		Password: "ApiMatrixPassw0rd!",
	}); err != nil {
		t.Fatalf("create canary user: %v", err)
	}

	t.Cleanup(func() {
		if err := c.DeleteUser(ctx, &client.DeleteUserOptions{Email: canaryEmail}); err != nil {
			t.Logf("cleanup: delete canary user: %v", err)
		}
	})

	end := time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339)

	logs, err := c.ListAuditLogs(ctx, &client.ListAuditLogsParams{
		Page:    1,
		PerPage: 100,
		Action:  "create_user",
		Start:   start,
		End:     end,
	})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}

	found := findAuditLogByDetails(logs.Items, canaryEmail)
	if found == nil {
		t.Fatalf("audit log entry for canary %q not found (page returned %d items, totalCount %d)",
			canaryEmail, len(logs.Items), logs.TotalCount)
	}

	if found.Action != "create_user" {
		t.Fatalf("Action: got %q, want %q", found.Action, "create_user")
	}

	if found.User != adminEmail {
		t.Fatalf("User (actor): got %q, want %q", found.User, adminEmail)
	}

	if found.ID == "" {
		t.Fatalf("ID: got empty, want non-empty")
	}

	if found.Timestamp == "" {
		t.Fatalf("Timestamp: got empty, want non-empty")
	}

	if _, err := time.Parse(time.RFC3339, found.Timestamp); err != nil {
		t.Fatalf("Timestamp: not RFC 3339: %q (%v)", found.Timestamp, err)
	}
}

func findAuditLogByDetails(items []client.AuditLog, marker string) *client.AuditLog {
	for i := range items {
		if strings.Contains(items[i].Details, marker) {
			return &items[i]
		}
	}

	return nil
}
