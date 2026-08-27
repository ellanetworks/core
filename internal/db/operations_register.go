// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package-level registration of every typed replicated operation.
// Renaming, deleting, or relaxing RequireSchema breaks rolling upgrades.

package db

import (
	ellaraft "github.com/ellanetworks/core/internal/raft"
)

// Subscribers. subscribers.description added in v18: an INSERT changeset
// carries every column of the new row, and the profile update writes the
// column outright.
var (
	opCreateSubscriber        = registerChangesetOp("CreateSubscriber", (*Database).applyCreateSubscriber, RequireSchema(18))
	opUpdateSubscriberProfile = registerChangesetOp("UpdateSubscriberProfile", (*Database).applyUpdateSubscriberProfile, RequireSchema(18), AffectsTopic(TopicSessionReconcile))
	opEditSubscriberSeqNum    = registerChangesetOp("EditSubscriberSeqNum", (*Database).applyEditSubscriberSeqNum)
	opAdvanceSubscriberSQN    = registerChangesetOpReturning[AdvanceSQNPayload, *AdvancedCredentials]("AdvanceSubscriberSQN", (*Database).applyAdvanceSubscriberSQN)
	opDeleteSubscriber        = registerChangesetOp("DeleteSubscriber", (*Database).applyDeleteSubscriber)
)

// Daily usage
var (
	opIncrementDailyUsage = registerChangesetOp("IncrementDailyUsage", (*Database).applyIncrementDailyUsage)
	opClearDailyUsage     = registerChangesetOp("ClearDailyUsage", (*Database).applyClearDailyUsageOp)
)

// IP leases. ip_leases.nodeID added in v9.
var (
	opCreateLease               = registerChangesetOp("CreateLease", (*Database).applyCreateLease, RequireSchema(9), AffectsTopic(TopicIPLeases))
	opUpdateLeaseSession        = registerChangesetOp("UpdateLeaseSession", (*Database).applyUpdateLeaseSession, RequireSchema(9), AffectsTopic(TopicIPLeases))
	opDeleteDynamicLease        = registerChangesetOp("DeleteDynamicLease", (*Database).applyDeleteDynamicLease, RequireSchema(9), AffectsTopic(TopicIPLeases))
	opDeleteDynamicLeasesByNode = registerChangesetOp("DeleteDynamicLeasesByNode", (*Database).applyDeleteDynamicLeasesByNode, RequireSchema(9), AffectsTopic(TopicIPLeases))
	opUpdateLeaseNode           = registerChangesetOp("UpdateLeaseNode", (*Database).applyUpdateLeaseNode, RequireSchema(9), AffectsTopic(TopicIPLeases))
	opAllocateIPLease           = registerChangesetOpReturning[allocateIPLeasePayload, string]("AllocateIPLease", (*Database).applyAllocateIPLease, RequireSchema(12), AffectsTopic(TopicIPLeases))
	opAllocateIPv6Lease         = registerChangesetOpReturning[allocateIPLeasePayload, string]("AllocateIPv6Lease", (*Database).applyAllocateIPLease, RequireSchema(12), AffectsTopic(TopicIPLeases))
	opReleaseIPLease            = registerChangesetOpReturning[releaseIPLeasePayload, string]("ReleaseIPLease", (*Database).applyReleaseIPLease, RequireSchema(13), AffectsTopic(TopicIPLeases))
	opCreateStaticLease         = registerChangesetOp("CreateStaticLease", (*Database).applyCreateStaticLease, RequireSchema(13), AffectsTopic(TopicIPLeases), AffectsTopic(TopicSessionReconcile))
	opUpdateStaticLeaseAddress  = registerChangesetOp("UpdateStaticLeaseAddress", (*Database).applyUpdateStaticLeaseAddress, RequireSchema(13), AffectsTopic(TopicIPLeases), AffectsTopic(TopicSessionReconcile))
	opDeleteStaticLease         = registerChangesetOp("DeleteStaticLease", (*Database).applyDeleteStaticLease, RequireSchema(13), AffectsTopic(TopicIPLeases), AffectsTopic(TopicSessionReconcile))
)

// Audit logs
var (
	_ = registerChangesetOp("InsertAuditLog", (*Database).applyInsertAuditLog)
)

// retiredChangesetOps and retiredIntentOps name operations that stay
// registered — a follower on an older binary still forwards them by name — but
// that a leader cannot honor: audit_logs is local-only, and a leader has no way
// to write another node's rows.
//
// ApplyForwardedOperation rejects them. Capturing an InsertAuditLog on the
// leader yields an empty changeset, since audit_logs is unattached, and the
// propose path reads an empty changeset as a successful no-op — leaving the
// forwarding node told that a discarded audit record was written.
var (
	retiredChangesetOps = map[string]struct{}{
		"InsertAuditLog": {},
	}

	retiredIntentOps = map[string]struct{}{
		"DeleteOldAuditLogs": {},
	}
)

// Users
var (
	opCreateUser         = registerChangesetOp("CreateUser", (*Database).applyCreateUser)
	opUpdateUser         = registerChangesetOp("UpdateUser", (*Database).applyUpdateUser)
	opUpdateUserPassword = registerChangesetOp("UpdateUserPassword", (*Database).applyUpdateUserPassword)
	opDeleteUser         = registerChangesetOp("DeleteUser", (*Database).applyDeleteUser)
)

// Profiles
var (
	opCreateProfile = registerChangesetOp("CreateProfile", (*Database).applyCreateProfile)
	opUpdateProfile = registerChangesetOp("UpdateProfile", (*Database).applyUpdateProfile, AffectsTopic(TopicSessionReconcile))
	opDeleteProfile = registerChangesetOp("DeleteProfile", (*Database).applyDeleteProfile)
)

// API tokens
var (
	opCreateAPIToken = registerChangesetOp("CreateAPIToken", (*Database).applyCreateAPIToken)
	opDeleteAPIToken = registerChangesetOp("DeleteAPIToken", (*Database).applyDeleteAPIToken)
)

// Sessions
var (
	opCreateSession            = registerChangesetOp("CreateSession", (*Database).applyCreateSession)
	opDeleteSessionByTokenHash = registerChangesetOp("DeleteSessionByTokenHash", (*Database).applyDeleteSessionByTokenHash)
	opDeleteOldestSessions     = registerChangesetOp("DeleteOldestSessions", (*Database).applyDeleteOldestSessions)
	opDeleteAllSessionsForUser = registerChangesetOp("DeleteAllSessionsForUser", (*Database).applyDeleteAllSessionsForUser)
	opDeleteAllSessions        = registerChangesetOp("DeleteAllSessions", (*Database).applyDeleteAllSessionsOp)
)

// Network slices
var (
	opCreateNetworkSlice = registerChangesetOp("CreateNetworkSlice", (*Database).applyCreateNetworkSlice)
	opUpdateNetworkSlice = registerChangesetOp("UpdateNetworkSlice", (*Database).applyUpdateNetworkSlice, AffectsTopic(TopicSessionReconcile))
	opDeleteNetworkSlice = registerChangesetOp("DeleteNetworkSlice", (*Database).applyDeleteNetworkSlice, AffectsTopic(TopicSessionReconcile))
)

// Data networks
var (
	opCreateDataNetwork = registerChangesetOp("CreateDataNetwork", (*Database).applyCreateDataNetwork, AffectsTopic(TopicDataNetworks), AffectsTopic(TopicSessionReconcile))
	opUpdateDataNetwork = registerChangesetOp("UpdateDataNetwork", (*Database).applyUpdateDataNetwork, AffectsTopic(TopicDataNetworks), AffectsTopic(TopicSessionReconcile))
	opDeleteDataNetwork = registerChangesetOp("DeleteDataNetwork", (*Database).applyDeleteDataNetwork, AffectsTopic(TopicDataNetworks), AffectsTopic(TopicSessionReconcile))
)

// Policies
var (
	opCreatePolicy          = registerChangesetOp("CreatePolicy", (*Database).applyCreatePolicy, AffectsTopic(TopicPolicies), AffectsTopic(TopicSessionReconcile))
	opUpdatePolicy          = registerChangesetOp("UpdatePolicy", (*Database).applyUpdatePolicy, AffectsTopic(TopicPolicies), AffectsTopic(TopicSessionReconcile))
	opDeletePolicy          = registerChangesetOp("DeletePolicy", (*Database).applyDeletePolicy, AffectsTopic(TopicPolicies), AffectsTopic(TopicSessionReconcile))
	opSetDefaultPolicy      = registerChangesetOp("SetDefaultPolicy", (*Database).applySetDefaultPolicy, RequireSchema(14), AffectsTopic(TopicPolicies), AffectsTopic(TopicSessionReconcile))
	opCreatePolicyWithRules = registerChangesetOp("CreatePolicyWithRules", (*Database).applyCreatePolicyWithRules, AffectsTopic(TopicPolicies), AffectsTopic(TopicNetworkRules), AffectsTopic(TopicSessionReconcile))
	opUpdatePolicyWithRules = registerChangesetOp("UpdatePolicyWithRules", (*Database).applyUpdatePolicyWithRules, AffectsTopic(TopicPolicies), AffectsTopic(TopicNetworkRules), AffectsTopic(TopicSessionReconcile))
)

// Network rules
var (
	opCreateNetworkRule          = registerChangesetOp("CreateNetworkRule", (*Database).applyCreateNetworkRule, AffectsTopic(TopicNetworkRules))
	opUpdateNetworkRule          = registerChangesetOp("UpdateNetworkRule", (*Database).applyUpdateNetworkRule, AffectsTopic(TopicNetworkRules))
	opDeleteNetworkRule          = registerChangesetOp("DeleteNetworkRule", (*Database).applyDeleteNetworkRule, AffectsTopic(TopicNetworkRules))
	opDeleteNetworkRulesByPolicy = registerChangesetOp("DeleteNetworkRulesByPolicy", (*Database).applyDeleteNetworkRulesByPolicy, AffectsTopic(TopicNetworkRules))
)

// Framed routes
var (
	opReplaceFramedRoutes = registerChangesetOp("ReplaceFramedRoutes", (*Database).applyReplaceFramedRoutes, RequireSchema(16), AffectsTopic(TopicSessionReconcile), AffectsTopic(TopicFramedRoutes))
)

// Home network key
var (
	opCreateHomeNetworkKey = registerChangesetOp("CreateHomeNetworkKey", (*Database).applyCreateHomeNetworkKey)
	opDeleteHomeNetworkKey = registerChangesetOp("DeleteHomeNetworkKey", (*Database).applyDeleteHomeNetworkKey)
)

// BGP. bgp_peers.nodeID added in v9.

// Retention
var (
	opSetRetentionPolicy = registerChangesetOp("SetRetentionPolicy", (*Database).applySetRetentionPolicy)
)

// Operator
var (
	opInitializeOperator               = registerChangesetOp("InitializeOperator", (*Database).applyInitializeOperator)
	opUpdateOperatorTracking           = registerChangesetOp("UpdateOperatorTracking", (*Database).applyUpdateOperatorTracking)
	opUpdateOperatorID                 = registerChangesetOp("UpdateOperatorID", (*Database).applyUpdateOperatorID)
	opUpdateOperatorCode               = registerChangesetOp("UpdateOperatorCode", (*Database).applyUpdateOperatorCode)
	opUpdateOperatorSecurityAlgorithms = registerChangesetOp("UpdateOperatorSecurityAlgorithms", (*Database).applyUpdateOperatorSecurityAlgorithms)
	opUpdateOperatorSPN                = registerChangesetOp("UpdateOperatorSPN", (*Database).applyUpdateOperatorSPN)
	opUpdateOperatorAMFIdentity        = registerChangesetOp("UpdateOperatorAMFIdentity", (*Database).applyUpdateOperatorAMFIdentity, RequireSchema(9))
	opUpdateOperatorClusterID          = registerChangesetOp("UpdateOperatorClusterID", (*Database).applyUpdateOperatorClusterID)
)

// JWT secret
var (
	opSetJWTSecret = registerChangesetOp("SetJWTSecret", (*Database).applySetJWTSecret)
)

// Routes

// Cluster members. cluster_members table introduced in v9.
var (
	opUpsertClusterMember = registerChangesetOp("UpsertClusterMember", (*Database).applyUpsertClusterMember, RequireSchema(9))
	opDeleteClusterMember = registerChangesetOp("DeleteClusterMember", (*Database).applyDeleteClusterMember, RequireSchema(9))
	opSetDrainState       = registerChangesetOp("SetDrainState", (*Database).applySetDrainState, RequireSchema(9))
)

// Cluster PKI. cluster_join_tokens dates from v9;
// cluster_node_certs and cluster_join_hmac are added in v12.
var (
	opUpsertNodeCert        = registerChangesetOp("UpsertClusterNodeCert", (*Database).applyUpsertNodeCert, RequireSchema(12), AffectsTopic(TopicClusterNodeCerts))
	opDeleteNodeCert        = registerChangesetOp("DeleteClusterNodeCert", (*Database).applyDeleteNodeCert, RequireSchema(12), AffectsTopic(TopicClusterNodeCerts))
	opMintJoinToken         = registerChangesetOp("MintJoinToken", (*Database).applyInsertJoinToken, RequireSchema(9))
	opConsumeJoinToken      = registerChangesetOp("ConsumeJoinToken", (*Database).applyConsumeJoinToken, RequireSchema(9))
	opRedeemJoinToken       = registerChangesetOpReturning[redeemJoinTokenPayload, *RedeemJoinTokenResult]("RedeemJoinToken", (*Database).applyRedeemJoinToken, RequireSchema(12), AffectsTopic(TopicClusterNodeCerts))
	opDeleteStaleJoinTokens = registerChangesetOp("DeleteStaleJoinTokens", (*Database).applyDeleteJoinTokensStale, RequireSchema(9))
	opInitJoinHMAC          = registerChangesetOp("InitClusterJoinHMACKey", (*Database).applyInitJoinHMAC, RequireSchema(12))
)

// Intent ops — bulk deletes and migrations dispatched explicitly by the
// FSM via CommandType. Call sites use intentOp.Invoke; the forwarded-op
// envelope carries the same name the leader's dispatcher looks up here.
var (
	_                        = registerIntentOp("DeleteOldAuditLogs", ellaraft.CmdDeleteOldAuditLogs)
	opDeleteOldDailyUsage    = registerIntentOp("DeleteOldDailyUsage", ellaraft.CmdDeleteOldDailyUsage)
	opDeleteAllDynamicLeases = registerIntentOp("DeleteAllDynamicLeases", ellaraft.CmdDeleteAllDynamicLeases, AffectsTopic(TopicIPLeases))
	opDeleteExpiredSessions  = registerIntentOpReturning[int]("DeleteExpiredSessions", ellaraft.CmdDeleteExpiredSessions)
	opMigrateShared          = registerIntentOp("MigrateShared", ellaraft.CmdMigrateShared)
)
