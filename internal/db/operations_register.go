// Copyright 2026 Ella Networks

// Package-level registration of every typed replicated operation.
// Exposed as vars so call sites can write `opFoo.Invoke(db, payload)`
// without running a lookup on the hot path. registerChangesetOp panics
// on duplicate names, so a collision surfaces at import time.
//
// Do not reuse names across schema versions: forwarded operations arriving
// from a rolling upgrade reach this table keyed by name, and a silent
// re-mapping would replicate the wrong statement.

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

// IP leases
var (
	opCreateLease               = registerChangesetOp("CreateLease", (*Database).applyCreateLease)
	opUpdateLeaseSession        = registerChangesetOp("UpdateLeaseSession", (*Database).applyUpdateLeaseSession)
	opDeleteDynamicLease        = registerChangesetOp("DeleteDynamicLease", (*Database).applyDeleteDynamicLease)
	opDeleteDynamicLeasesByNode = registerChangesetOp("DeleteDynamicLeasesByNode", (*Database).applyDeleteDynamicLeasesByNode)
	opUpdateLeaseNode           = registerChangesetOp("UpdateLeaseNode", (*Database).applyUpdateLeaseNode)
	// AllocateIPLease replaces the follower-side pre-pick-then-forward
	// path. The wire payload is just the intent (poolID, IMSI, sessionID,
	// nodeID); the leader's apply function does the SELECT-then-INSERT
	// atomically inside leaderCaptureAndPropose, so concurrent allocations
	// from any node are serialised by proposeMu.
	opAllocateIPLease = registerChangesetOp("AllocateIPLease", (*Database).applyAllocateIPLease)
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
	opCreateDataNetwork = registerChangesetOp("CreateDataNetwork", (*Database).applyCreateDataNetwork)
	opUpdateDataNetwork = registerChangesetOp("UpdateDataNetwork", (*Database).applyUpdateDataNetwork)
	opDeleteDataNetwork = registerChangesetOp("DeleteDataNetwork", (*Database).applyDeleteDataNetwork)
)

// Policies
var (
	opCreatePolicy = registerChangesetOp("CreatePolicy", (*Database).applyCreatePolicy)
	opUpdatePolicy = registerChangesetOp("UpdatePolicy", (*Database).applyUpdatePolicy)
	opDeletePolicy = registerChangesetOp("DeletePolicy", (*Database).applyDeletePolicy)
)

// Network rules
var (
	opCreateNetworkRule          = registerChangesetOp("CreateNetworkRule", (*Database).applyCreateNetworkRule)
	opUpdateNetworkRule          = registerChangesetOp("UpdateNetworkRule", (*Database).applyUpdateNetworkRule)
	opDeleteNetworkRule          = registerChangesetOp("DeleteNetworkRule", (*Database).applyDeleteNetworkRule)
	opDeleteNetworkRulesByPolicy = registerChangesetOp("DeleteNetworkRulesByPolicy", (*Database).applyDeleteNetworkRulesByPolicy)
)

// Home network key
var (
	opCreateHomeNetworkKey = registerChangesetOp("CreateHomeNetworkKey", (*Database).applyCreateHomeNetworkKey)
	opDeleteHomeNetworkKey = registerChangesetOp("DeleteHomeNetworkKey", (*Database).applyDeleteHomeNetworkKey)
)

// BGP
var (
	opCreateBGPPeer            = registerChangesetOp("CreateBGPPeer", (*Database).applyCreateBGPPeer)
	opUpdateBGPPeer            = registerChangesetOp("UpdateBGPPeer", (*Database).applyUpdateBGPPeer)
	opDeleteBGPPeer            = registerChangesetOp("DeleteBGPPeer", (*Database).applyDeleteBGPPeer)
	opUpdateBGPSettings        = registerChangesetOp("UpdateBGPSettings", (*Database).applyUpdateBGPSettings)
	opSetImportPrefixesForPeer = registerChangesetOp("SetImportPrefixesForPeer", (*Database).applySetImportPrefixesForPeer)
)

// NAT / N3 / Flow accounting
var (
	opUpdateNATSettings            = registerChangesetOp("UpdateNATSettings", (*Database).applyUpdateNATSettings)
	opUpdateN3Settings             = registerChangesetOp("UpdateN3Settings", (*Database).applyUpdateN3Settings)
	opUpdateFlowAccountingSettings = registerChangesetOp("UpdateFlowAccountingSettings", (*Database).applyUpdateFlowAccountingSettings)
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
	opUpdateOperatorAMFIdentity        = registerChangesetOp("UpdateOperatorAMFIdentity", (*Database).applyUpdateOperatorAMFIdentity)
	opUpdateOperatorClusterID          = registerChangesetOp("UpdateOperatorClusterID", (*Database).applyUpdateOperatorClusterID)
)

// JWT secret
var (
	opSetJWTSecret = registerChangesetOp("SetJWTSecret", (*Database).applySetJWTSecret)
)

// Routes
var (
	opCreateRoute = registerChangesetOp("CreateRoute", (*Database).applyCreateRoute)
	opDeleteRoute = registerChangesetOp("DeleteRoute", (*Database).applyDeleteRoute)
)

// Cluster members
var (
	opUpsertClusterMember = registerChangesetOp("UpsertClusterMember", (*Database).applyUpsertClusterMember)
	opDeleteClusterMember = registerChangesetOp("DeleteClusterMember", (*Database).applyDeleteClusterMember)
	opSetDrainState       = registerChangesetOp("SetDrainState", (*Database).applySetDrainState)
)

// Cluster PKI
var (
	opInsertPKIRoot            = registerChangesetOp("InsertPKIRoot", (*Database).applyInsertPKIRoot)
	opSetPKIRootStatus         = registerChangesetOp("SetPKIRootStatus", (*Database).applySetPKIRootStatus)
	opDeletePKIRoot            = registerChangesetOp("DeletePKIRoot", (*Database).applyDeletePKIRoot)
	opInsertPKIIntermediate    = registerChangesetOp("InsertPKIIntermediate", (*Database).applyInsertPKIIntermediate)
	opSetPKIIntermediateStatus = registerChangesetOp("SetPKIIntermediateStatus", (*Database).applySetPKIIntermediateStatus)
	opDeletePKIIntermediate    = registerChangesetOp("DeletePKIIntermediate", (*Database).applyDeletePKIIntermediate)
	opRecordIssuedCert         = registerChangesetOp("RecordIssuedCert", (*Database).applyInsertIssuedCert)
	opDeleteExpiredIssuedCerts = registerChangesetOp("DeleteExpiredIssuedCerts", (*Database).applyDeleteIssuedCertsExpired)
	opInsertRevokedCert        = registerChangesetOp("InsertRevokedCert", (*Database).applyInsertRevokedCert)
	opDeletePurgedRevocations  = registerChangesetOp("DeletePurgedRevocations", (*Database).applyDeleteRevokedCertsPurged)
	opMintJoinToken            = registerChangesetOp("MintJoinToken", (*Database).applyInsertJoinToken)
	opConsumeJoinToken         = registerChangesetOp("ConsumeJoinToken", (*Database).applyConsumeJoinToken)
	opDeleteStaleJoinTokens    = registerChangesetOp("DeleteStaleJoinTokens", (*Database).applyDeleteJoinTokensStale)
	opInitializePKIState       = registerChangesetOp("InitializePKIState", (*Database).applyInitPKIState)
	opBootstrapPKI             = registerChangesetOp("BootstrapPKI", (*Database).applyBootstrapPKIOp)
	opAllocatePKISerial        = registerChangesetOp("AllocatePKISerial", (*Database).applyAllocatePKISerialOp)
)

// Intent ops — bulk deletes and migrations dispatched explicitly by the
// FSM via CommandType. Call sites use intentOp.Invoke; the forwarded-op
// envelope carries the same name the leader's dispatcher looks up here.
var (
	opDeleteOldAuditLogs     = registerIntentOp("DeleteOldAuditLogs", ellaraft.CmdDeleteOldAuditLogs)
	opDeleteOldDailyUsage    = registerIntentOp("DeleteOldDailyUsage", ellaraft.CmdDeleteOldDailyUsage)
	opDeleteAllDynamicLeases = registerIntentOp("DeleteAllDynamicLeases", ellaraft.CmdDeleteAllDynamicLeases)
	opDeleteExpiredSessions  = registerIntentOp("DeleteExpiredSessions", ellaraft.CmdDeleteExpiredSessions)
	opMigrateShared          = registerIntentOp("MigrateShared", ellaraft.CmdMigrateShared)
)
