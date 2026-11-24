// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"slices"
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

var operationsAtCommandSchema = []string{
	"AdvanceSubscriberSQN",
	"AllocateIPLease",
	"AllocateIPv6Lease",
	"ClearDailyUsage",
	"ClearSMSMemoryFull",
	"CompareAndSwapIMSRegistration",
	"CompareAndSwapIMSRegistrationSets",
	"ConsumeJoinToken",
	"CreateAPIToken",
	"CreateCellPosition",
	"CreateDataNetwork",
	"CreateHomeNetworkKey",
	"CreateIMSApplicationServer",
	"CreateIMSPeer",
	"CreateIMSServiceProfile",
	"CreateLease",
	"CreateNetworkRule",
	"CreateNetworkSlice",
	"CreatePolicy",
	"CreatePolicyWithRules",
	"CreateProfile",
	"CreateRoamingDiameterPeer",
	"CreateRoamingPartner",
	"CreateSMSC",
	"CreateSMSCPeer",
	"CreateSession",
	"CreateSliceDataNetwork",
	"CreateStaticLease",
	"CreateSubscriber",
	"CreateUser",
	"DeleteAPIToken",
	"DeleteAllDynamicLeases",
	"DeleteAllSessions",
	"DeleteAllSessionsForUser",
	"DeleteCellPosition",
	"DeleteClusterMember",
	"DeleteClusterNodeCert",
	"DeleteDataNetwork",
	"DeleteDynamicLease",
	"DeleteDynamicLeasesByNode",
	"DeleteExpiredSessions",
	"DeleteHomeNetworkKey",
	"DeleteIMSApplicationServer",
	"DeleteIMSPeer",
	"DeleteIMSServiceProfile",
	"DeleteIPSMGWRegistration",
	"DeleteNetworkRule",
	"DeleteNetworkRulesByPolicy",
	"DeleteNetworkSlice",
	"DeleteOldAuditLogs",
	"DeleteOldDailyUsage",
	"DeleteOldestSessions",
	"DeletePolicy",
	"DeleteProfile",
	"DeleteRoamingDiameterPeer",
	"DeleteRoamingPartner",
	"DeleteSEPPIdentity",
	"DeleteSMSC",
	"DeleteSMSCPeer",
	"DeleteSMSWaiting",
	"DeleteSessionByTokenHash",
	"DeleteSliceDataNetwork",
	"DeleteStaleJoinTokens",
	"DeleteStaleSMSWaiting",
	"DeleteStaticLease",
	"DeleteSubscriber",
	"DeleteUser",
	"EditSubscriberSeqNum",
	"IncrementDailyUsage",
	"IncrementDailyUsageBatch",
	"InitClusterJoinHMACKey",
	"InitSEPPCA",
	"InitializeOperator",
	"InsertAuditLog",
	"LinkOIDCIdentity",
	"LogInWithOIDC",
	"MigrateShared",
	"MintJoinToken",
	"PurgeUERegistration",
	"PutGeneratedSEPPIdentity",
	"PutIPSMGWRegistration",
	"PutSEPPIdentity",
	"RecordSMSWaiting",
	"RedeemJoinToken",
	"RegisterUE",
	"RegisterVisitorUE",
	"ReleaseIPLease",
	"RemoveSMSWaitingCentre",
	"ReplaceFramedRoutes",
	"ReplacePCSCFAddresses",
	"ReplaceSEPPCA",
	"RotateJWTSecret",
	"SetDefaultPolicy",
	"SetDisplayName",
	"SetDrainState",
	"SetJWTSecret",
	"SetProfileIMSServiceProfiles",
	"SetRetentionPolicy",
	"SetRoamingPartnerWithdrawing",
	"SetVisitorDeregistering",
	"SetVisitorMSISDN",
	"UnlinkUser",
	"UpdateCellPosition",
	"UpdateDataNetwork",
	"UpdateIMSApplicationServer",
	"UpdateIMSPeer",
	"UpdateIMSServiceProfile",
	"UpdateIMSSettings",
	"UpdateLeaseNode",
	"UpdateLeaseSession",
	"UpdateNetworkRule",
	"UpdateNetworkSlice",
	"UpdateOIDC",
	"UpdateOperatorAMFIdentity",
	"UpdateOperatorClusterID",
	"UpdateOperatorCode",
	"UpdateOperatorID",
	"UpdateOperatorSPN",
	"UpdateOperatorSecurityAlgorithms",
	"UpdateOperatorTracking",
	"UpdatePolicy",
	"UpdatePolicyWithRules",
	"UpdateProfile",
	"UpdateRadioEventSettings",
	"UpdateRoamingDiameterPeer",
	"UpdateRoamingPartner",
	"UpdateSMSC",
	"UpdateSMSCPeer",
	"UpdateSMSSettings",
	"UpdateSliceDataNetwork",
	"UpdateStaticLeaseAddress",
	"UpdateSubscriberProfile",
	"UpdateUser",
	"UpdateUserPassword",
	"UpsertClusterMember",
	"UpsertClusterNodeCert",
	"UseOIDCState",
	"VerifyState",
}

func TestOperationsAddedAfterTheCommandSchemaDeclareTheirSchema(t *testing.T) {
	registered := map[string]int{}

	for name, h := range changesetOps {
		registered[name] = h.minSchema
	}

	for name, h := range intentOps {
		registered[name] = h.minSchema
	}

	for name, schema := range registered {
		if slices.Contains(operationsAtCommandSchema, name) {
			continue
		}

		if schema <= commandSchema || schema > SchemaVersion() {
			t.Errorf("operation %q was added after schema %d but requires schema %d; "+
				"declare RequireSchema(N) with N the migration that introduces it (%d < N <= %d), "+
				"so no node applies it before every member runs code that knows it",
				name, commandSchema, schema, commandSchema, SchemaVersion())
		}
	}
}

func TestEveryCommandRequiresTheCommandSchema(t *testing.T) {
	database := newDatabaseAtVersion(t, commandSchema-1)
	ctx := context.Background()

	var names []string

	for name := range changesetOps {
		names = append(names, name)
	}

	for name := range intentOps {
		names = append(names, name)
	}

	slices.Sort(names)

	commands := 0

	for _, name := range names {
		h, ok := commandHandlerFor(name)
		if !ok {
			continue
		}

		commands++

		if h.minSchema < commandSchema {
			t.Errorf("operation %q has a command form that requires schema %d, below the command schema %d", name, h.minSchema, commandSchema)
		}

		body := []byte(`{"l":1,"c":[{"o":"` + name + `","p":{}}]}`)

		out, err := database.ApplyEntries(ctx, []ellaraft.Entry{{Index: uint64(commands), Term: 1, Data: body}})
		if err != nil {
			t.Errorf("operation %q on a schema %d database: %v, want a migration-pending result", name, commandSchema-1, err)
			continue
		}

		results, _ := out[0].([]commandResult)
		if len(results) != 1 || !errors.Is(results[0].err, ErrMigrationPending) {
			t.Errorf("operation %q applied as a command on a schema %d database: got %+v, want a migration-pending result", name, commandSchema-1, results)
		}
	}

	if commands < 100 {
		t.Fatalf("only %d operations have a command form; the registry scan is broken", commands)
	}
}
