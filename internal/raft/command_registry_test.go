// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import "testing"

// pinnedCommandTypes is the replicated command vocabulary as of the
// current release. Editing it is the deliberate act that adding or
// retiring a CommandType requires; see the CommandType doc comment for
// what else that change has to carry.
var pinnedCommandTypes = map[CommandType]string{
	0:   "Changeset",
	12:  "DeleteOldDailyUsage",
	23:  "DeleteAllDynamicLeases",
	31:  "DeleteOldAuditLogs",
	72:  "DeleteExpiredSessions",
	220: "MigrateShared",
}

const commandTypeRule = `A CommandType was added, retired or renumbered.

Every node applies every committed entry, so a node running an older
binary halts on an entry whose type it does not know, and halts again on
every restart because the entry is durable.

Ship the change with a schema migration and declare db.RequireSchema(N)
on the operation that proposes the command, so the leader withholds it
until every member reports a binary that supports N. Then update
pinnedCommandTypes.`

func TestCommandTypeRegistryIsPinned(t *testing.T) {
	for cmd, name := range pinnedCommandTypes {
		got, ok := commandNames[cmd]
		if !ok {
			t.Errorf("CommandType %d (%s) is pinned but absent from commandNames\n\n%s", cmd, name, commandTypeRule)
			continue
		}

		if got != name {
			t.Errorf("CommandType %d: registered as %q, pinned as %q\n\n%s", cmd, got, name, commandTypeRule)
		}
	}

	for cmd, name := range commandNames {
		if _, ok := pinnedCommandTypes[cmd]; !ok {
			t.Errorf("CommandType %d (%s) is registered but not pinned\n\n%s", cmd, name, commandTypeRule)
		}
	}
}
