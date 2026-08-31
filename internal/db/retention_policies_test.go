// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestRetentionPolicyEndToEnd(t *testing.T) {
	database := setupTestDB(t)

	res, err := database.GetRetentionPolicy(context.Background(), db.CategoryAuditLogs)
	if err != nil {
		t.Fatalf("couldn't get audit log retention policy: %s", err)
	}

	if res != 7 {
		t.Fatalf("Expected default audit log retention policy to be 7 days, but got %d", res)
	}

	policy := &db.RetentionPolicy{
		Category: db.CategoryAuditLogs,
		Days:     60,
	}

	err = database.SetRetentionPolicy(context.Background(), policy)
	if err != nil {
		t.Fatalf("couldn't set audit log retention policy: %s", err)
	}

	res, err = database.GetRetentionPolicy(context.Background(), db.CategoryAuditLogs)
	if err != nil {
		t.Fatalf("couldn't get audit log retention policy: %s", err)
	}

	if res != 60 {
		t.Fatalf("Expected audit log retention policy to be 60 days, but got %d", res)
	}
}
