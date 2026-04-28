// Copyright 2026 Ella Networks

// Package-level registration of every typed replicated operation.
//
// Append-only: renaming, deleting, or relaxing RequireSchema breaks
// rolling upgrades. TestOperationsRegistry_AppendOnly enforces this
// against operations.lock.json.

package db

import (
	ellaraft "github.com/ellanetworks/core/internal/raft"
)

// Subscribers
var (
	opCreateSubscriber        = registerChangesetOp("CreateSubscriber", (*Database).applyCreateSubscriber)
	opUpdateSubscriberProfile = registerChangesetOp("UpdateSubscriberProfile", (*Database).applyUpdateSubscriberProfile)
	opEditSubscriberSeqNum    = registerChangesetOp("EditSubscriberSeqNum", (*Database).applyEditSubscriberSeqNum)
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
	// AllocateIPLease forwards intent only; leader resolves the IP
	// atomically under proposeMu (see applyAllocateIPLease).
	opAllocateIPLease = registerChangesetOp("AllocateIPLease", (*Database).applyAllocateIPLease, RequireSchema(9), AffectsTopic(TopicIPLeases))
)

// Audit logs
var (
	opInsertAuditLog = registerChangesetOp("InsertAuditLog", (*Database).applyInsertAuditLog)
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
	opUpdateProfile = registerChangesetOp("UpdateProfile", (*Database).applyUpdateProfile)
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
	opUpdateNetworkSlice = registerChangesetOp("UpdateNetworkSlice", (*Database).applyUpdateNetworkSlice)
	opDeleteNetworkSlice = registerChangesetOp("DeleteNetworkSlice", (*Database).applyDeleteNetworkSlice)
)

// Data networks
var (
	opCreateDataNetwork = registerChangesetOp("CreateDataNetwork", (*Database).applyCreateDataNetwork, AffectsTopic(TopicDataNetworks))
	opUpdateDataNetwork = registerChangesetOp("UpdateDataNetwork", (*Database).applyUpdateDataNetwork, AffectsTopic(TopicDataNetworks))
	opDeleteDataNetwork = registerChangesetOp("DeleteDataNetwork", (*Database).applyDeleteDataNetwork, AffectsTopic(TopicDataNetworks))
)

// Policies
var (
	opCreatePolicy = registerChangesetOp("CreatePolicy", (*Database).applyCreatePolicy, AffectsTopic(TopicPolicies))
	opUpdatePolicy = registerChangesetOp("UpdatePolicy", (*Database).applyUpdatePolicy, AffectsTopic(TopicPolicies))
	opDeletePolicy = registerChangesetOp("DeletePolicy", (*Database).applyDeletePolicy, AffectsTopic(TopicPolicies))
)

// Network rules
var (
	opCreateNetworkRule          = registerChangesetOp("CreateNetworkRule", (*Database).applyCreateNetworkRule, AffectsTopic(TopicNetworkRules))
	opUpdateNetworkRule          = registerChangesetOp("UpdateNetworkRule", (*Database).applyUpdateNetworkRule, AffectsTopic(TopicNetworkRules))
	opDeleteNetworkRule          = registerChangesetOp("DeleteNetworkRule", (*Database).applyDeleteNetworkRule, AffectsTopic(TopicNetworkRules))
	opDeleteNetworkRulesByPolicy = registerChangesetOp("DeleteNetworkRulesByPolicy", (*Database).applyDeleteNetworkRulesByPolicy, AffectsTopic(TopicNetworkRules))
)

// Home network key
var (
	opCreateHomeNetworkKey = registerChangesetOp("CreateHomeNetworkKey", (*Database).applyCreateHomeNetworkKey)
	opDeleteHomeNetworkKey = registerChangesetOp("DeleteHomeNetworkKey", (*Database).applyDeleteHomeNetworkKey)
)

// BGP. bgp_peers.nodeID added in v9.
var (
	opCreateBGPPeer            = registerChangesetOp("CreateBGPPeer", (*Database).applyCreateBGPPeer, RequireSchema(9), AffectsTopic(TopicBGPPeers))
	opUpdateBGPPeer            = registerChangesetOp("UpdateBGPPeer", (*Database).applyUpdateBGPPeer, RequireSchema(9), AffectsTopic(TopicBGPPeers))
	opDeleteBGPPeer            = registerChangesetOp("DeleteBGPPeer", (*Database).applyDeleteBGPPeer, RequireSchema(9), AffectsTopic(TopicBGPPeers))
	opUpdateBGPSettings        = registerChangesetOp("UpdateBGPSettings", (*Database).applyUpdateBGPSettings, AffectsTopic(TopicBGPSettings))
	opSetImportPrefixesForPeer = registerChangesetOp("SetImportPrefixesForPeer", (*Database).applySetImportPrefixesForPeer, AffectsTopic(TopicBGPPeers))
)

// NAT / N3 / Flow accounting
var (
	opUpdateNATSettings            = registerChangesetOp("UpdateNATSettings", (*Database).applyUpdateNATSettings, AffectsTopic(TopicNATSettings))
	opUpdateN3Settings             = registerChangesetOp("UpdateN3Settings", (*Database).applyUpdateN3Settings, AffectsTopic(TopicN3Settings))
	opUpdateFlowAccountingSettings = registerChangesetOp("UpdateFlowAccountingSettings", (*Database).applyUpdateFlowAccountingSettings, AffectsTopic(TopicFlowAccountingSettings))
)

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
var (
	opCreateRoute = registerChangesetOp("CreateRoute", (*Database).applyCreateRoute, AffectsTopic(TopicRoutes))
	opDeleteRoute = registerChangesetOp("DeleteRoute", (*Database).applyDeleteRoute, AffectsTopic(TopicRoutes))
)

// Cluster members. cluster_members table introduced in v9.
var (
	opUpsertClusterMember = registerChangesetOp("UpsertClusterMember", (*Database).applyUpsertClusterMember, RequireSchema(9))
	opDeleteClusterMember = registerChangesetOp("DeleteClusterMember", (*Database).applyDeleteClusterMember, RequireSchema(9))
	opSetDrainState       = registerChangesetOp("SetDrainState", (*Database).applySetDrainState, RequireSchema(9))
)

// Cluster PKI. All PKI tables introduced in v9.
var (
	opInsertPKIRoot            = registerChangesetOp("InsertPKIRoot", (*Database).applyInsertPKIRoot, RequireSchema(9))
	opSetPKIRootStatus         = registerChangesetOp("SetPKIRootStatus", (*Database).applySetPKIRootStatus, RequireSchema(9))
	opDeletePKIRoot            = registerChangesetOp("DeletePKIRoot", (*Database).applyDeletePKIRoot, RequireSchema(9))
	opInsertPKIIntermediate    = registerChangesetOp("InsertPKIIntermediate", (*Database).applyInsertPKIIntermediate, RequireSchema(9))
	opSetPKIIntermediateStatus = registerChangesetOp("SetPKIIntermediateStatus", (*Database).applySetPKIIntermediateStatus, RequireSchema(9))
	opDeletePKIIntermediate    = registerChangesetOp("DeletePKIIntermediate", (*Database).applyDeletePKIIntermediate, RequireSchema(9))
	opRecordIssuedCert         = registerChangesetOp("RecordIssuedCert", (*Database).applyInsertIssuedCert, RequireSchema(9))
	opDeleteExpiredIssuedCerts = registerChangesetOp("DeleteExpiredIssuedCerts", (*Database).applyDeleteIssuedCertsExpired, RequireSchema(9))
	opInsertRevokedCert        = registerChangesetOp("InsertRevokedCert", (*Database).applyInsertRevokedCert, RequireSchema(9))
	opDeletePurgedRevocations  = registerChangesetOp("DeletePurgedRevocations", (*Database).applyDeleteRevokedCertsPurged, RequireSchema(9))
	opMintJoinToken            = registerChangesetOp("MintJoinToken", (*Database).applyInsertJoinToken, RequireSchema(9))
	opConsumeJoinToken         = registerChangesetOp("ConsumeJoinToken", (*Database).applyConsumeJoinToken, RequireSchema(9))
	opDeleteStaleJoinTokens    = registerChangesetOp("DeleteStaleJoinTokens", (*Database).applyDeleteJoinTokensStale, RequireSchema(9))
	opInitializePKIState       = registerChangesetOp("InitializePKIState", (*Database).applyInitPKIState, RequireSchema(9))
	opBootstrapPKI             = registerChangesetOp("BootstrapPKI", (*Database).applyBootstrapPKIOp, RequireSchema(9))
	opAllocatePKISerial        = registerChangesetOp("AllocatePKISerial", (*Database).applyAllocatePKISerialOp, RequireSchema(9))
)

// Intent ops — bulk deletes and migrations dispatched explicitly by the
// FSM via CommandType. Call sites use intentOp.Invoke; the forwarded-op
// envelope carries the same name the leader's dispatcher looks up here.
var (
	opDeleteOldAuditLogs     = registerIntentOp("DeleteOldAuditLogs", ellaraft.CmdDeleteOldAuditLogs)
	opDeleteOldDailyUsage    = registerIntentOp("DeleteOldDailyUsage", ellaraft.CmdDeleteOldDailyUsage)
	opDeleteAllDynamicLeases = registerIntentOp("DeleteAllDynamicLeases", ellaraft.CmdDeleteAllDynamicLeases, AffectsTopic(TopicIPLeases))
	opDeleteExpiredSessions  = registerIntentOp("DeleteExpiredSessions", ellaraft.CmdDeleteExpiredSessions)
	opMigrateShared          = registerIntentOp("MigrateShared", ellaraft.CmdMigrateShared)
)
