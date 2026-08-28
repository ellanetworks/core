// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"testing"
)

func seedMembers(t *testing.T, database *Database, ctx context.Context, nodeIDs ...int) {
	t.Helper()

	for _, id := range nodeIDs {
		m := &ClusterMember{
			NodeID:      id,
			RaftAddress: "10.0.0.1:7000",
			APIAddress:  "https://10.0.0.1:5000",
			Suffrage:    "voter",
		}
		if err := database.UpsertClusterMember(ctx, m); err != nil {
			t.Fatalf("seed member %d: %v", id, err)
		}
	}
}

func memberIDs(t *testing.T, database *Database, ctx context.Context) []int {
	t.Helper()

	members, err := database.ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("ListClusterMembers: %v", err)
	}

	ids := make([]int, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.NodeID)
	}

	return ids
}

func equalIDs(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func TestReconcileClusterMembers_DeletesRowAbsentFromConfiguration(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	seedMembers(t, database, ctx, 1, 2, 3)

	database.clusterEnabled = true
	database.raftMemberIDs = func() []int { return []int{1, 2} }

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	if got := memberIDs(t, database, ctx); !equalIDs(got, []int{1, 2}) {
		t.Fatalf("members after reconcile: want [1 2], got %v", got)
	}
}

func TestReconcileClusterMembers_KeepsEveryConfiguredMember(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	seedMembers(t, database, ctx, 1, 2, 3)

	database.clusterEnabled = true
	database.raftMemberIDs = func() []int { return []int{1, 2, 3} }

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	if got := memberIDs(t, database, ctx); !equalIDs(got, []int{1, 2, 3}) {
		t.Fatalf("members after reconcile: want [1 2 3], got %v", got)
	}
}

func TestReconcileClusterMembers_KeepsNodeInConfigurationWithoutRow(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	seedMembers(t, database, ctx, 1)

	database.clusterEnabled = true
	database.raftMemberIDs = func() []int { return []int{1, 2} }

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	if got := memberIDs(t, database, ctx); !equalIDs(got, []int{1}) {
		t.Fatalf("members after reconcile: want [1], got %v", got)
	}
}

func TestReconcileClusterMembers_UnavailableConfigurationDeletesNothing(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	seedMembers(t, database, ctx, 1, 2, 3)

	database.clusterEnabled = true
	database.raftMemberIDs = func() []int { return nil }

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	if got := memberIDs(t, database, ctx); !equalIDs(got, []int{1, 2, 3}) {
		t.Fatalf("members after reconcile: want [1 2 3], got %v", got)
	}
}

func TestReconcileClusterMembers_NoRaftAccessorDeletesNothing(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	seedMembers(t, database, ctx, 1, 2)

	database.clusterEnabled = true
	database.raftMemberIDs = nil

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	if got := memberIDs(t, database, ctx); !equalIDs(got, []int{1, 2}) {
		t.Fatalf("members after reconcile: want [1 2], got %v", got)
	}
}

func TestReconcileClusterMembers_StandaloneDeletesNothing(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	seedMembers(t, database, ctx, 1, 2, 3)

	if database.ClusterEnabled() {
		t.Fatal("newStandaloneDB reported clustering enabled; the standalone gate is not under test")
	}

	database.raftMemberIDs = func() []int { return []int{1} }

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	if got := memberIDs(t, database, ctx); !equalIDs(got, []int{1, 2, 3}) {
		t.Fatalf("standalone reconcile wiped the inventory a restored HA backup carries: want [1 2 3], got %v", got)
	}
}

func TestReconcileClusterMembers_PurgesLeasesAndCertPin(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	seedMembers(t, database, ctx, 1, 2)

	if err := database.UpsertClusterNodeCert(ctx, &ClusterNodeCert{
		NodeID:      2,
		Fingerprint: "aa:bb:cc",
	}); err != nil {
		t.Fatalf("seed cert pin: %v", err)
	}

	database.clusterEnabled = true
	database.raftMemberIDs = func() []int { return []int{1} }

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	if got := memberIDs(t, database, ctx); !equalIDs(got, []int{1}) {
		t.Fatalf("members after reconcile: want [1], got %v", got)
	}

	certs, err := database.ListClusterNodeCerts(ctx)
	if err != nil {
		t.Fatalf("ListClusterNodeCerts: %v", err)
	}

	for _, c := range certs {
		if c.NodeID == 2 {
			t.Error("reconciled node kept its certificate pin; peers would still accept its handshakes")
		}
	}
}
