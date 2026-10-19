// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"testing"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

func seedMembers(t *testing.T, database *Database, ctx context.Context, nodeIDs ...string) {
	t.Helper()

	for _, id := range nodeIDs {
		m := &ClusterMember{
			NodeID:     id,
			APIAddress: "https://10.0.0.1:5000",
		}
		if err := database.UpsertClusterMember(ctx, m); err != nil {
			t.Fatalf("seed member %s: %v", id, err)
		}
	}
}

func memberIDs(t *testing.T, database *Database, ctx context.Context) []string {
	t.Helper()

	members, err := database.ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("ListClusterMembers: %v", err)
	}

	ids := make([]string, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.NodeID)
	}

	return ids
}

func equalIDs(a, b []string) bool {
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

	seedMembers(t, database, ctx, "1", "2", "3")

	database.clusterEnabled = true
	database.raftMemberIDs = func() []string { return []string{"1", "2"} }

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	if got := memberIDs(t, database, ctx); !equalIDs(got, []string{"1", "2"}) {
		t.Fatalf("members after reconcile: want [1 2], got %v", got)
	}
}

func TestReconcileClusterMembers_KeepsEveryConfiguredMember(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	seedMembers(t, database, ctx, "1", "2", "3")

	database.clusterEnabled = true
	database.raftMemberIDs = func() []string { return []string{"1", "2", "3"} }

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	if got := memberIDs(t, database, ctx); !equalIDs(got, []string{"1", "2", "3"}) {
		t.Fatalf("members after reconcile: want [1 2 3], got %v", got)
	}
}

func TestReconcileClusterMembers_KeepsNodeInConfigurationWithoutRow(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	seedMembers(t, database, ctx, "1")

	database.clusterEnabled = true
	database.raftMemberIDs = func() []string { return []string{"1", "2"} }

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	if got := memberIDs(t, database, ctx); !equalIDs(got, []string{"1"}) {
		t.Fatalf("members after reconcile: want [1], got %v", got)
	}
}

func TestReconcileClusterMembers_UnavailableConfigurationDeletesNothing(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	seedMembers(t, database, ctx, "1", "2", "3")

	database.clusterEnabled = true
	database.raftMemberIDs = func() []string { return nil }

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	if got := memberIDs(t, database, ctx); !equalIDs(got, []string{"1", "2", "3"}) {
		t.Fatalf("members after reconcile: want [1 2 3], got %v", got)
	}
}

func TestReconcileClusterMembers_StandaloneDeletesNothing(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	seedMembers(t, database, ctx, "1", "2", "3")

	if database.ClusterEnabled() {
		t.Fatal("newStandaloneDB reported clustering enabled; the standalone gate is not under test")
	}

	database.raftMemberIDs = func() []string { return []string{"1"} }

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	if got := memberIDs(t, database, ctx); !equalIDs(got, []string{"1", "2", "3"}) {
		t.Fatalf("standalone reconcile wiped the inventory a restored HA backup carries: want [1 2 3], got %v", got)
	}
}

func TestReconcileClusterMembers_PurgesLeasesAndCertPin(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	seedMembers(t, database, ctx, "1", "2")

	if err := database.UpsertClusterNodeCert(ctx, &ClusterNodeCert{
		NodeID:      "2",
		Fingerprint: "aa:bb:cc",
	}); err != nil {
		t.Fatalf("seed cert pin: %v", err)
	}

	database.clusterEnabled = true
	database.raftMemberIDs = func() []string { return []string{"1"} }

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	if got := memberIDs(t, database, ctx); !equalIDs(got, []string{"1"}) {
		t.Fatalf("members after reconcile: want [1], got %v", got)
	}

	certs, err := database.ListClusterNodeCerts(ctx)
	if err != nil {
		t.Fatalf("ListClusterNodeCerts: %v", err)
	}

	for _, c := range certs {
		if c.NodeID == "2" {
			t.Error("reconciled node kept its certificate pin; peers would still accept its handshakes")
		}
	}
}

func stubStatusProbe(statuses map[string]ellaraft.PeerStatus) func(context.Context, string, string) (ellaraft.PeerStatus, error) {
	return func(_ context.Context, nodeID string, _ string) (ellaraft.PeerStatus, error) {
		status, ok := statuses[nodeID]
		if !ok {
			return ellaraft.PeerStatus{}, errors.New("simulated peer unreachable")
		}

		return status, nil
	}
}

func seedVersionedMember(t *testing.T, database *Database, ctx context.Context, nodeID string, apiAddress string, binaryVersion string) {
	t.Helper()

	if err := database.UpsertClusterMember(ctx, &ClusterMember{
		NodeID:        nodeID,
		APIAddress:    apiAddress,
		BinaryVersion: binaryVersion,
	}); err != nil {
		t.Fatalf("seed member %s: %v", nodeID, err)
	}
}

func getMember(t *testing.T, database *Database, ctx context.Context, nodeID string) *ClusterMember {
	t.Helper()

	m, err := database.GetClusterMember(ctx, nodeID)
	if err != nil {
		t.Fatalf("GetClusterMember %s: %v", nodeID, err)
	}

	return m
}

func TestReconcileClusterMembers_RefreshesStaleVersionAndAddress(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	seedVersionedMember(t, database, ctx, "2", "https://10.0.0.2:5000", "v1.19.0")

	before := getMember(t, database, ctx, "2")

	database.clusterEnabled = true
	database.raftMemberIDs = func() []string { return []string{"2"} }
	database.raftServers = stubServers([]string{"2"})
	database.probeMemberStatus = stubStatusProbe(map[string]ellaraft.PeerStatus{
		"2": {SchemaVersion: 1, BinaryVersion: "v1.20.0", APIAddress: "https://10.0.0.20:5000"},
	})

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	got := getMember(t, database, ctx, "2")
	if got.BinaryVersion != "v1.20.0" {
		t.Fatalf("binaryVersion: want v1.20.0, got %q", got.BinaryVersion)
	}

	if got.APIAddress != "https://10.0.0.20:5000" {
		t.Fatalf("apiAddress: want https://10.0.0.20:5000, got %q", got.APIAddress)
	}

	if got.AMFPointer != before.AMFPointer {
		t.Fatalf("amfPointer changed on refresh: was %d, now %d", before.AMFPointer, got.AMFPointer)
	}
}

func TestReconcileClusterMembers_KeepsFieldsAPeerDoesNotReport(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	seedVersionedMember(t, database, ctx, "2", "https://10.0.0.2:5000", "v1.19.0")

	database.clusterEnabled = true
	database.raftMemberIDs = func() []string { return []string{"2"} }
	database.raftServers = stubServers([]string{"2"})
	database.probeMemberStatus = stubStatusProbe(map[string]ellaraft.PeerStatus{
		"2": {SchemaVersion: 1},
	})

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	got := getMember(t, database, ctx, "2")
	if got.BinaryVersion != "v1.19.0" || got.APIAddress != "https://10.0.0.2:5000" {
		t.Fatalf("row changed for a peer that reports neither field: %+v", got)
	}
}

func TestReconcileClusterMembers_UnreachablePeerKeepsRow(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	seedVersionedMember(t, database, ctx, "2", "https://10.0.0.2:5000", "v1.19.0")

	database.clusterEnabled = true
	database.raftMemberIDs = func() []string { return []string{"2"} }
	database.raftServers = stubServers([]string{"2"})
	database.probeMemberStatus = stubStatusProbe(nil)

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	got := getMember(t, database, ctx, "2")
	if got.BinaryVersion != "v1.19.0" || got.APIAddress != "https://10.0.0.2:5000" {
		t.Fatalf("row changed for an unreachable peer: %+v", got)
	}
}

func TestReconcileClusterMembers_DoesNotProbeSelf(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	selfID := database.RaftID()
	seedVersionedMember(t, database, ctx, selfID, "https://10.0.0.1:5000", "v1.20.0")

	database.clusterEnabled = true
	database.raftMemberIDs = func() []string { return []string{selfID} }
	database.raftServers = stubServers([]string{selfID})
	database.probeMemberStatus = func(context.Context, string, string) (ellaraft.PeerStatus, error) {
		t.Fatal("leader probed its own status")
		return ellaraft.PeerStatus{}, nil
	}

	if err := database.reconcileClusterMembers(ctx); err != nil {
		t.Fatalf("reconcileClusterMembers: %v", err)
	}

	if got := getMember(t, database, ctx, selfID); got.BinaryVersion != "v1.20.0" {
		t.Fatalf("self row changed: %+v", got)
	}
}
