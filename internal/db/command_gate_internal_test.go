// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"testing"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

var baselineIntentCmds = map[ellaraft.CommandType]bool{
	ellaraft.CmdDeleteOldDailyUsage:    true,
	ellaraft.CmdDeleteAllDynamicLeases: true,
	ellaraft.CmdDeleteOldAuditLogs:     true,
	ellaraft.CmdDeleteExpiredSessions:  true,
	ellaraft.CmdMigrateShared:          true,
}

func TestIntentOpsOutsideBaselineDeclareSchema(t *testing.T) {
	for name, h := range intentOps {
		if baselineIntentCmds[h.cmdType] {
			continue
		}

		if h.minSchema <= 1 {
			t.Errorf("intent op %q proposes %s without RequireSchema; "+
				"declare the version of the migration that ships the command "+
				"so the leader withholds it until every member supports it",
				name, h.cmdType)
		}
	}
}

func TestBaselineIntentCmdsAreRegistered(t *testing.T) {
	registered := map[ellaraft.CommandType]bool{}
	for _, h := range intentOps {
		registered[h.cmdType] = true
	}

	for cmd := range baselineIntentCmds {
		if !registered[cmd] {
			t.Errorf("baseline command %s is absent from the intent op registry; "+
				"retiring a command type strands members that still propose it", cmd)
		}
	}
}
