// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"strings"
	"testing"
)

func TestStaticLeaseLookupUsesTheIMSIIndex(t *testing.T) {
	database := newInitializedCommandTestDB(t)

	rows, err := database.reader().QueryContext(t.Context(), "EXPLAIN QUERY PLAN "+getStaticLeaseSQL, 1, "ipv4", "001010000000001")
	if err != nil {
		t.Fatalf("explain: %v", err)
	}

	defer func() { _ = rows.Close() }()

	var plan []string

	for rows.Next() {
		var id, parent, unused int

		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatalf("scan: %v", err)
		}

		plan = append(plan, detail)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	if got := strings.Join(plan, "; "); !strings.Contains(got, "idx_leases_imsi") {
		t.Fatalf("plan = %q, want a search on idx_leases_imsi", got)
	}
}
