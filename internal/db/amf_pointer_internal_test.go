// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"fmt"
	"testing"
)

func upsertMember(t *testing.T, d *Database, nodeID string) int {
	t.Helper()

	member := &ClusterMember{
		NodeID:        nodeID,
		APIAddress:    nodeID + ":5002",
		BinaryVersion: "test",
	}

	if _, err := d.applyUpsertClusterMember(context.Background(), member); err != nil {
		t.Fatalf("applyUpsertClusterMember(%s): %v", nodeID, err)
	}

	return member.AMFPointer
}

func TestAllocateAMFPointerGivesEachMemberADistinctValue(t *testing.T) {
	d := newStandaloneDB(t)

	seen := make(map[int]string)

	for _, nodeID := range []string{
		"01890000-0000-7000-8000-00000000000a",
		"01890000-0000-7000-8000-00000000000b",
		"01890000-0000-7000-8000-00000000000c",
	} {
		pointer := upsertMember(t, d, nodeID)

		if pointer < DefaultAMFPointer || pointer > MaxAMFPointer {
			t.Fatalf("node %s got pointer %d, want a value in [%d, %d]",
				nodeID, pointer, DefaultAMFPointer, MaxAMFPointer)
		}

		if prev, dup := seen[pointer]; dup {
			t.Fatalf("nodes %s and %s share AMF Pointer %d; every node stamps it into its GUTIs",
				prev, nodeID, pointer)
		}

		seen[pointer] = nodeID
	}
}

func TestAllocateAMFPointerKeepsAMemberPointerAcrossUpserts(t *testing.T) {
	d := newStandaloneDB(t)

	const nodeID = "01890000-0000-7000-8000-00000000000a"

	first := upsertMember(t, d, nodeID)

	if first != DefaultAMFPointer {
		t.Fatalf("first member got pointer %d, want %d", first, DefaultAMFPointer)
	}

	if again := upsertMember(t, d, nodeID); again != first {
		t.Fatalf("re-upsert moved the pointer from %d to %d; GUTIs already issued under it must keep routing", first, again)
	}

	member, err := d.GetClusterMember(context.Background(), nodeID)
	if err != nil {
		t.Fatalf("GetClusterMember: %v", err)
	}

	if member.AMFPointer != first {
		t.Fatalf("persisted pointer = %d, want %d: an in-memory allocation is lost on restart",
			member.AMFPointer, first)
	}
}

func TestAllocateAMFPointerReusesAGapLeftByARemovedMember(t *testing.T) {
	d := newStandaloneDB(t)
	ctx := context.Background()

	a := "01890000-0000-7000-8000-00000000000a"
	b := "01890000-0000-7000-8000-00000000000b"

	upsertMember(t, d, a)

	second := upsertMember(t, d, b)

	if _, err := d.applyDeleteClusterMember(ctx, &nodeIDPayload{Value: "01890000-0000-7000-8000-00000000000a"}); err != nil {
		t.Fatalf("delete member a: %v", err)
	}

	c := upsertMember(t, d, "01890000-0000-7000-8000-00000000000c")

	if c == second {
		t.Fatalf("new member took %d, the value member b still holds", c)
	}
}

func TestAllocateAMFPointerExhaustionIsAnError(t *testing.T) {
	d := newStandaloneDB(t)

	for i := DefaultAMFPointer; i <= MaxAMFPointer; i++ {
		upsertMember(t, d, fmt.Sprintf("01890000-0000-7000-8000-%012d", i))
	}

	member := &ClusterMember{
		NodeID:     "01890000-0000-7000-8000-ffffffffffff",
		APIAddress: "x:5002",
	}

	if _, err := d.applyUpsertClusterMember(context.Background(), member); err == nil {
		t.Fatalf("a %dth member was admitted; the AMF Pointer field is 6 bits", MaxAMFPointer+1)
	}
}

func TestAllocateAMFPointerKeepsALegacyIntegerIdentity(t *testing.T) {
	d := newStandaloneDB(t)

	if got := upsertMember(t, d, "7"); got != 7 {
		t.Fatalf("legacy node 7 got pointer %d, want 7: an upgraded cluster keeps the pointer it already issued GUTIs under", got)
	}
}
