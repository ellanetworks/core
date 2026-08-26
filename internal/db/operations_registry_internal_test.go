// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import "testing"

// Editing these maps is the deliberate act that adding, retiring or
// renaming a replicated operation requires.
var pinnedChangesetOps = map[string]int{
	"AdvanceSubscriberSQN":             1,
	"AllocateIPLease":                  12,
	"AllocateIPv6Lease":                12,
	"ClearDailyUsage":                  1,
	"ConsumeJoinToken":                 9,
	"CreateAPIToken":                   1,
	"CreateDataNetwork":                1,
	"CreateHomeNetworkKey":             1,
	"CreateLease":                      9,
	"CreateNetworkRule":                1,
	"CreateNetworkSlice":               1,
	"CreatePolicy":                     1,
	"CreatePolicyWithRules":            1,
	"CreateProfile":                    1,
	"CreateSession":                    1,
	"CreateStaticLease":                13,
	"CreateSubscriber":                 1,
	"CreateUser":                       1,
	"DeleteAPIToken":                   1,
	"DeleteAllSessions":                1,
	"DeleteAllSessionsForUser":         1,
	"DeleteClusterMember":              9,
	"DeleteClusterNodeCert":            12,
	"DeleteDataNetwork":                1,
	"DeleteDynamicLease":               9,
	"DeleteDynamicLeasesByNode":        9,
	"DeleteHomeNetworkKey":             1,
	"DeleteNetworkRule":                1,
	"DeleteNetworkRulesByPolicy":       1,
	"DeleteNetworkSlice":               1,
	"DeleteOldestSessions":             1,
	"DeletePolicy":                     1,
	"DeleteProfile":                    1,
	"DeleteSessionByTokenHash":         1,
	"DeleteStaleJoinTokens":            9,
	"DeleteStaticLease":                13,
	"DeleteSubscriber":                 1,
	"DeleteUser":                       1,
	"EditSubscriberSeqNum":             1,
	"IncrementDailyUsage":              1,
	"InitClusterJoinHMACKey":           12,
	"InitializeOperator":               1,
	"InsertAuditLog":                   1,
	"MintJoinToken":                    9,
	"RedeemJoinToken":                  12,
	"ReleaseIPLease":                   13,
	"ReplaceFramedRoutes":              16,
	"SetDefaultPolicy":                 14,
	"SetDrainState":                    9,
	"SetJWTSecret":                     1,
	"SetRetentionPolicy":               1,
	"UpdateDataNetwork":                1,
	"UpdateLeaseNode":                  9,
	"UpdateLeaseSession":               9,
	"UpdateNetworkRule":                1,
	"UpdateNetworkSlice":               1,
	"UpdateOperatorAMFIdentity":        9,
	"UpdateOperatorClusterID":          1,
	"UpdateOperatorCode":               1,
	"UpdateOperatorID":                 1,
	"UpdateOperatorSPN":                1,
	"UpdateOperatorSecurityAlgorithms": 1,
	"UpdateOperatorTracking":           1,
	"UpdatePolicy":                     1,
	"UpdatePolicyWithRules":            1,
	"UpdateProfile":                    1,
	"UpdateStaticLeaseAddress":         13,
	"UpdateSubscriberProfile":          1,
	"UpdateUser":                       1,
	"UpdateUserPassword":               1,
	"UpsertClusterMember":              9,
	"UpsertClusterNodeCert":            12,
}

var pinnedIntentOps = map[string]int{
	"DeleteAllDynamicLeases": 1,
	"DeleteExpiredSessions":  1,
	"DeleteOldAuditLogs":     1,
	"DeleteOldDailyUsage":    1,
	"MigrateShared":          1,
}

const operationRule = `A replicated operation was added, retired, renamed, or had its
RequireSchema changed.

The operation name is the forwarded wire contract: a follower running an
older binary sends its own name to whichever node is leader, and the
leader dispatches by that name. Retiring a name breaks every write
forwarded by a not-yet-upgraded node during a rolling upgrade, which is
why an operation with no remaining Go caller must still stay registered.
Lowering RequireSchema lets an operation apply against a schema that
predates the columns it writes.

Add or amend the pinned entry deliberately, in the same change.`

func TestChangesetOpRegistryIsPinned(t *testing.T) {
	assertRegistryPinned(t, pinnedChangesetOps, changesetOpSchemas())
}

func TestIntentOpRegistryIsPinned(t *testing.T) {
	assertRegistryPinned(t, pinnedIntentOps, intentOpSchemas())
}

func changesetOpSchemas() map[string]int {
	out := make(map[string]int, len(changesetOps))
	for name, h := range changesetOps {
		out[name] = h.minSchema
	}

	return out
}

func intentOpSchemas() map[string]int {
	out := make(map[string]int, len(intentOps))
	for name, h := range intentOps {
		out[name] = h.minSchema
	}

	return out
}

func assertRegistryPinned(t *testing.T, pinned, registered map[string]int) {
	t.Helper()

	for name, want := range pinned {
		got, ok := registered[name]
		if !ok {
			t.Errorf("operation %q is pinned but no longer registered\n\n%s", name, operationRule)
			continue
		}

		if got != want {
			t.Errorf("operation %q: RequireSchema is %d, pinned as %d\n\n%s", name, got, want, operationRule)
		}
	}

	for name := range registered {
		if _, ok := pinned[name]; !ok {
			t.Errorf("operation %q is registered but not pinned\n\n%s", name, operationRule)
		}
	}
}
