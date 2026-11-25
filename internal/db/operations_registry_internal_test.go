// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import "testing"

var pinnedChangesetOps = map[string]int{
	"AdvanceSubscriberSQN":              1,
	"AllocateIPLease":                   1,
	"AllocateIPv6Lease":                 1,
	"ClearDailyUsage":                   1,
	"ConsumeJoinToken":                  1,
	"CreateAPIToken":                    1,
	"CreateDataNetwork":                 1,
	"CreateHomeNetworkKey":              1,
	"CreateLease":                       1,
	"CreateNetworkRule":                 1,
	"CreateNetworkSlice":                1,
	"CreatePolicy":                      1,
	"CreatePolicyWithRules":             1,
	"CreateProfile":                     1,
	"CreateSession":                     1,
	"CreateStaticLease":                 1,
	"CreateSubscriber":                  1,
	"CreateUser":                        1,
	"DeleteAPIToken":                    1,
	"DeleteAllSessions":                 1,
	"DeleteAllSessionsForUser":          1,
	"DeleteClusterMember":               1,
	"DeleteClusterNodeCert":             1,
	"DeleteDataNetwork":                 1,
	"DeleteDynamicLease":                1,
	"DeleteDynamicLeasesByNode":         1,
	"DeleteHomeNetworkKey":              1,
	"DeleteNetworkRule":                 1,
	"DeleteNetworkRulesByPolicy":        1,
	"DeleteNetworkSlice":                1,
	"DeleteOldestSessions":              1,
	"DeletePolicy":                      1,
	"DeleteProfile":                     1,
	"DeleteSessionByTokenHash":          1,
	"DeleteStaleJoinTokens":             1,
	"DeleteStaticLease":                 1,
	"DeleteSubscriber":                  1,
	"DeleteUser":                        1,
	"EditSubscriberSeqNum":              1,
	"IncrementDailyUsage":               1,
	"IncrementDailyUsageBatch":          1,
	"InitClusterJoinHMACKey":            1,
	"InitializeOperator":                1,
	"InsertAuditLog":                    1,
	"MintJoinToken":                     1,
	"RedeemJoinToken":                   1,
	"PurgeUERegistration":               1,
	"RegisterUE":                        1,
	"ReleaseIPLease":                    1,
	"ReplaceFramedRoutes":               1,
	"SetDefaultPolicy":                  1,
	"SetDisplayName":                    1,
	"SetDrainState":                     1,
	"SetJWTSecret":                      1,
	"SetRetentionPolicy":                1,
	"UpdateDataNetwork":                 1,
	"UpdateLeaseNode":                   1,
	"UpdateLeaseSession":                1,
	"UpdateNetworkRule":                 1,
	"UpdateNetworkSlice":                1,
	"UpdateOperatorAMFIdentity":         1,
	"UpdateOperatorClusterID":           1,
	"UpdateOperatorCode":                1,
	"UpdateOperatorID":                  1,
	"UpdateOperatorSPN":                 1,
	"UpdateOperatorSecurityAlgorithms":  1,
	"UpdateOperatorTracking":            1,
	"UpdatePolicy":                      1,
	"UpdatePolicyWithRules":             1,
	"UpdateProfile":                     1,
	"UpdateSMSSettings":                 1,
	"CreateSMSCPeer":                    1,
	"UpdateSMSCPeer":                    1,
	"DeleteSMSCPeer":                    1,
	"CreateSMSC":                        1,
	"CreateSliceDataNetwork":            1,
	"UpdateSliceDataNetwork":            1,
	"DeleteSliceDataNetwork":            1,
	"UpdateSMSC":                        1,
	"DeleteSMSC":                        1,
	"RecordSMSWaiting":                  1,
	"ClearSMSMemoryFull":                1,
	"RemoveSMSWaitingCentre":            1,
	"DeleteSMSWaiting":                  1,
	"DeleteStaleSMSWaiting":             1,
	"ReplacePCSCFAddresses":             1,
	"CompareAndSwapIMSRegistration":     1,
	"CompareAndSwapIMSRegistrationSets": 1,
	"CreateIMSApplicationServer":        1,
	"UpdateIMSApplicationServer":        1,
	"DeleteIMSApplicationServer":        1,
	"CreateIMSServiceProfile":           1,
	"UpdateIMSServiceProfile":           1,
	"DeleteIMSServiceProfile":           1,
	"SetProfileIMSServiceProfiles":      1,
	"PutIPSMGWRegistration":             1,
	"DeleteIPSMGWRegistration":          1,
	"UpdateStaticLeaseAddress":          1,
	"UpdateSubscriberProfile":           1,
	"UpdateUser":                        1,
	"UpdateOIDC":                        1,
	"UnlinkUser":                        1,
	"UseOIDCState":                      1,
	"LogInWithOIDC":                     1,
	"LinkOIDCIdentity":                  1,
	"UpdateUserPassword":                1,
	"UpsertClusterMember":               1,
	"UpsertClusterNodeCert":             1,
	"CreateRoamingPartner":              1,
	"UpdateRoamingPartner":              1,
	"DeleteRoamingPartner":              1,
	"CreateRoamingDiameterPeer":         1,
	"UpdateRoamingDiameterPeer":         1,
	"DeleteRoamingDiameterPeer":         1,
	"UpdateIMSSettings":                 1,
	"CreateIMSPeer":                     1,
	"UpdateIMSPeer":                     1,
	"DeleteIMSPeer":                     1,
	"PutSEPPIdentity":                   1,
	"InitSEPPCA":                        1,
	"ReplaceSEPPCA":                     1,
	"RegisterVisitorUE":                 1,
	"SetVisitorMSISDN":                  1,
	"SetRoamingPartnerWithdrawing":      1,
	"SetVisitorDeregistering":           1,
	"PutGeneratedSEPPIdentity":          1,
	"DeleteSEPPIdentity":                1,
	"UpdateRadioEventSettings":          1,
	"VerifyState":                       1,
}

var pinnedIntentOps = map[string]int{
	"CreateCellPosition":     1,
	"DeleteAllDynamicLeases": 1,
	"DeleteCellPosition":     1,
	"DeleteExpiredSessions":  1,
	"DeleteOldAuditLogs":     1,
	"DeleteOldDailyUsage":    1,
	"MigrateShared":          1,
	"RotateJWTSecret":        1,
	"UpdateCellPosition":     1,
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

Adding one is safe but ordered: a node running the new binary cannot
forward a new operation to a leader that predates it, so the leader must
be upgraded first.

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
