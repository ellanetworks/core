// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"testing"
)

func TestUpsertClusterMemberUUIDSucceedsAtLatestSchema(t *testing.T) {
	ctx := context.Background()
	d := newStandaloneDB(t)

	const uuidNodeID = "0199c4f1-2ab3-7c1d-9f2a-6fe8422da1ec"

	if _, err := d.applyUpsertClusterMember(ctx, &ClusterMember{
		NodeID:     uuidNodeID,
		APIAddress: "10.0.0.4:5002",
	}); err != nil {
		t.Fatalf("upsert of a UUID identity at the latest schema: %v", err)
	}

	member, err := d.GetClusterMember(ctx, uuidNodeID)
	if err != nil {
		t.Fatalf("GetClusterMember: %v", err)
	}

	if member.NodeID != uuidNodeID {
		t.Fatalf("nodeID = %q, want %q", member.NodeID, uuidNodeID)
	}
}
