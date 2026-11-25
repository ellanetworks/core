// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"testing"
	"time"
)

func TestNewLeaderCompactsEntriesFromBeforeCommands(t *testing.T) {
	tc := SetupTestCluster(t, 3, newTestApplier(t))

	leader := tc.WaitForLeader(10 * time.Second)
	if leader == nil {
		t.Fatal("no leader")
	}

	var lastIndex uint64

	for i := range 3 {
		cmd, err := NewCommand(CmdChangeset, map[string]int{"i": i})
		if err != nil {
			t.Fatalf("new command: %v", err)
		}

		result, err := leader.Propose(cmd, 5*time.Second)
		if err != nil {
			t.Fatalf("propose: %v", err)
		}

		lastIndex = result.Index
	}

	if err := tc.WaitForConvergence(lastIndex, 10*time.Second); err != nil {
		t.Fatalf("convergence: %v", err)
	}

	tc.StopNode(tc.LeaderIndex())

	deadline := time.Now().Add(15 * time.Second)

	for {
		next := tc.Leader()
		if next != nil && next != leader {
			index, _, err := lastPreCommandEntry(next.logStore)
			if err != nil {
				t.Fatalf("scan log: %v", err)
			}

			if index == 0 {
				first, err := next.logStore.FirstIndex()
				if err != nil {
					t.Fatalf("first index: %v", err)
				}

				if first != lastIndex+1 {
					t.Fatalf("first log index = %d, want %d: only the entries from before commands may be compacted", first, lastIndex+1)
				}

				return
			}
		}

		if time.Now().After(deadline) {
			t.Fatal("the new leader kept the entries from before commands in its log")
		}

		time.Sleep(50 * time.Millisecond)
	}
}
