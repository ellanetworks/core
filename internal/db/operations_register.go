// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package-level registration of every typed replicated operation.
// Renaming, deleting, or relaxing RequireSchema breaks rolling upgrades.

package db

import (
	ellaraft "github.com/ellanetworks/core/internal/raft"
)

// Subscribers. subscribers.description added in v18: the insert and the
// profile update write the column.
var (
	opCreateSubscriber        = registerChangesetOp("CreateSubscriber", (*Database).applyCreateSubscriber)
	opUpdateSubscriberProfile = registerChangesetOp("UpdateSubscriberProfile", (*Database).applyUpdateSubscriberProfile, AffectsTopic(TopicSessionReconcile))
	opEditSubscriberSeqNum    = registerChangesetOp("EditSubscriberSeqNum", (*Database).applyEditSubscriberSeqNum)
	opAdvanceSubscriberSQN    = registerChangesetOpReturning[AdvanceSQNPayload, *AdvancedCredentials]("AdvanceSubscriberSQN", (*Database).applyAdvanceSubscriberSQN)
	opDeleteSubscriber        = registerChangesetOp("DeleteSubscriber", (*Database).applyDeleteSubscriber, AffectsTopic(TopicUERegistrations))
)

// Daily usage
var (
	opIncrementDailyUsage      = registerChangesetOp("IncrementDailyUsage", (*Database).applyIncrementDailyUsage)
	opIncrementDailyUsageBatch = registerChangesetOpReturning[DailyUsageBatch, droppedDailyUsage]("IncrementDailyUsageBatch", (*Database).applyIncrementDailyUsageBatch)
	opClearDailyUsage          = registerChangesetOp("ClearDailyUsage", (*Database).applyClearDailyUsageOp)
)

// IP leases. ip_leases.nodeID added in v9.
var (
	opCreateLease               = registerChangesetOp("CreateLease", (*Database).applyCreateLease, AffectsTopic(TopicIPLeases))
	opUpdateLeaseSession        = registerChangesetOp("UpdateLeaseSession", (*Database).applyUpdateLeaseSession, AffectsTopic(TopicIPLeases))
	opDeleteDynamicLease        = registerChangesetOp("DeleteDynamicLease", (*Database).applyDeleteDynamicLease, AffectsTopic(TopicIPLeases))
	opDeleteDynamicLeasesByNode = registerChangesetOp("DeleteDynamicLeasesByNode", (*Database).applyDeleteDynamicLeasesByNode, AffectsTopic(TopicIPLeases))
	opUpdateLeaseNode           = registerChangesetOp("UpdateLeaseNode", (*Database).applyUpdateLeaseNode, AffectsTopic(TopicIPLeases))
	opAllocateIPLease           = registerChangesetOpReturning[allocateIPLeasePayload, string]("AllocateIPLease", (*Database).applyAllocateIPLease, AffectsTopic(TopicIPLeases))
	opAllocateIPv6Lease         = registerChangesetOpReturning[allocateIPLeasePayload, string]("AllocateIPv6Lease", (*Database).applyAllocateIPLease, AffectsTopic(TopicIPLeases))
	opReleaseIPLease            = registerChangesetOpReturning[releaseIPLeasePayload, string]("ReleaseIPLease", (*Database).applyReleaseIPLease, AffectsTopic(TopicIPLeases))
	opCreateStaticLease         = registerChangesetOp("CreateStaticLease", (*Database).applyCreateStaticLease, AffectsTopic(TopicIPLeases), AffectsTopic(TopicSessionReconcile))
	opUpdateStaticLeaseAddress  = registerChangesetOp("UpdateStaticLeaseAddress", (*Database).applyUpdateStaticLeaseAddress, AffectsTopic(TopicIPLeases), AffectsTopic(TopicSessionReconcile))
	opDeleteStaticLease         = registerChangesetOp("DeleteStaticLease", (*Database).applyDeleteStaticLease, AffectsTopic(TopicIPLeases), AffectsTopic(TopicSessionReconcile))
)

// Audit logs
var (
	_ = registerChangesetOp("InsertAuditLog", (*Database).applyInsertAuditLog)
)

var (
	retiredChangesetOps = map[string]struct{}{
		"InsertAuditLog":                {},
		"CompareAndSwapIMSRegistration": {},
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

// Single sign-on
var (
	opUpdateOIDC       = registerChangesetOp("UpdateOIDC", (*Database).applyUpdateOIDC, AffectsTopic(TopicOIDC))
	opUnlinkUser       = registerChangesetOp("UnlinkUser", (*Database).applyUnlinkUser)
	opUseOIDCState     = registerChangesetOp("UseOIDCState", (*Database).applyUseOIDCState)
	opLinkOIDCIdentity = registerChangesetOp("LinkOIDCIdentity", (*Database).applyLinkOIDCIdentity)
	opLogInWithOIDC    = registerChangesetOpReturning[OIDCLogin, OIDCLoginResult]("LogInWithOIDC", (*Database).applyLogInWithOIDC)
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
	opDeleteNetworkSlice = registerChangesetOp("DeleteNetworkSlice", (*Database).applyDeleteNetworkSlice, AffectsTopic(TopicDataNetworks), AffectsTopic(TopicSessionReconcile))
)

var (
	opCreateSliceDataNetwork = registerChangesetOp("CreateSliceDataNetwork", (*Database).applyCreateSliceDataNetwork, AffectsTopic(TopicDataNetworks), AffectsTopic(TopicSessionReconcile))
	opUpdateSliceDataNetwork = registerChangesetOp("UpdateSliceDataNetwork", (*Database).applyUpdateSliceDataNetwork, AffectsTopic(TopicDataNetworks), AffectsTopic(TopicSessionReconcile))
	opDeleteSliceDataNetwork = registerChangesetOp("DeleteSliceDataNetwork", (*Database).applyDeleteSliceDataNetwork, AffectsTopic(TopicDataNetworks), AffectsTopic(TopicSessionReconcile))
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
	opSetDefaultPolicy      = registerChangesetOp("SetDefaultPolicy", (*Database).applySetDefaultPolicy, AffectsTopic(TopicPolicies), AffectsTopic(TopicSessionReconcile))
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
	opReplaceFramedRoutes = registerChangesetOp("ReplaceFramedRoutes", (*Database).applyReplaceFramedRoutes, AffectsTopic(TopicSessionReconcile), AffectsTopic(TopicFramedRoutes))
)

// Home network key
var (
	opCreateHomeNetworkKey = registerChangesetOp("CreateHomeNetworkKey", (*Database).applyCreateHomeNetworkKey)
	opDeleteHomeNetworkKey = registerChangesetOp("DeleteHomeNetworkKey", (*Database).applyDeleteHomeNetworkKey)
)

// BGP. bgp_peers.nodeID added in v9.

// Retention
var (
	opSetRetentionPolicy       = registerChangesetOp("SetRetentionPolicy", (*Database).applySetRetentionPolicy)
	opUpdateRadioEventSettings = registerChangesetOp("UpdateRadioEventSettings", (*Database).applyUpdateRadioEventSettings, AffectsTopic(TopicRadioEventSettings))
)

// Operator
var (
	opInitializeOperator               = registerChangesetOp("InitializeOperator", (*Database).applyInitializeOperator)
	opUpdateOperatorTracking           = registerChangesetOp("UpdateOperatorTracking", (*Database).applyUpdateOperatorTracking)
	opUpdateOperatorID                 = registerChangesetOpReturning[Operator, bool]("UpdateOperatorID", (*Database).applyUpdateOperatorID, AffectsTopic(TopicOperatorIdentity), AffectsTopic(TopicSEPPIdentity))
	opUpdateOperatorCode               = registerChangesetOp("UpdateOperatorCode", (*Database).applyUpdateOperatorCode)
	opUpdateOperatorSecurityAlgorithms = registerChangesetOp("UpdateOperatorSecurityAlgorithms", (*Database).applyUpdateOperatorSecurityAlgorithms)
	opUpdateOperatorSPN                = registerChangesetOp("UpdateOperatorSPN", (*Database).applyUpdateOperatorSPN)
	opUpdateOperatorAMFIdentity        = registerChangesetOp("UpdateOperatorAMFIdentity", (*Database).applyUpdateOperatorAMFIdentity, AffectsTopic(TopicOperatorIdentity))
	opUpdateOperatorClusterID          = registerChangesetOp("UpdateOperatorClusterID", (*Database).applyUpdateOperatorClusterID)
)

// JWT secret
var (
	opSetJWTSecret    = registerChangesetOp("SetJWTSecret", (*Database).applySetJWTSecret, AffectsTopic(TopicJWTSecret))
	opRotateJWTSecret = registerIntentOp("RotateJWTSecret", ellaraft.CmdRotateJWTSecret, AffectsTopic(TopicJWTSecret), asCommand(withoutResult((*Database).applyRotateJWTSecret)))
)

var (
	opCreateCellPosition = registerIntentOp("CreateCellPosition", ellaraft.CmdPutCellPosition, asCommand((*Database).applyCreateCellPositionCommand))
	opUpdateCellPosition = registerIntentOp("UpdateCellPosition", ellaraft.CmdPutCellPosition, asCommand((*Database).applyUpdateCellPositionCommand))
	opDeleteCellPosition = registerIntentOp("DeleteCellPosition", ellaraft.CmdDeleteCellPosition, asCommand((*Database).applyDeleteCellPositionCommand))
)

// Routes

// Cluster members. cluster_members table introduced in v9.
var (
	opUpsertClusterMember = registerChangesetOp("UpsertClusterMember", (*Database).applyUpsertClusterMember, AffectsTopic(TopicClusterMembers))
	opDeleteClusterMember = registerChangesetOp("DeleteClusterMember", (*Database).applyDeleteClusterMember, AffectsTopic(TopicUERegistrations), AffectsTopic(TopicSEPPIdentity))
	opSetDrainState       = registerChangesetOpReturning[ClusterMember, string]("SetDrainState", (*Database).applySetDrainState, AffectsTopic(TopicClusterMembers))
	opSetDisplayName      = registerChangesetOp("SetDisplayName", (*Database).applySetDisplayName, AffectsTopic(TopicClusterMembers))
)

var (
	opRegisterUE          = registerChangesetOpReturning[registerUEPayload, int64]("RegisterUE", (*Database).applyRegisterUE, AffectsTopic(TopicUERegistrations), skipWhenUnchanged((*Database).registerUEUnchanged))
	opPurgeUERegistration = registerChangesetOp("PurgeUERegistration", (*Database).applyPurgeUERegistration, AffectsTopic(TopicUERegistrations))
)

var (
	opUpdateSMSSettings      = registerChangesetOp("UpdateSMSSettings", (*Database).applyUpdateSMSSettings, AffectsTopic(TopicSMSSettings))
	_                        = registerChangesetOp("CreateSMSCPeer", (*Database).applyCreateSMSCPeer, AffectsTopic(TopicSMSSettings))
	_                        = registerChangesetOp("UpdateSMSCPeer", (*Database).applyUpdateSMSCPeer, AffectsTopic(TopicSMSSettings))
	_                        = registerChangesetOp("DeleteSMSCPeer", (*Database).applyDeleteSMSCPeer, AffectsTopic(TopicSMSSettings))
	opCreateSMSC             = registerChangesetOp("CreateSMSC", (*Database).applyCreateSMSC, AffectsTopic(TopicSMSSettings))
	opUpdateSMSC             = registerChangesetOp("UpdateSMSC", (*Database).applyUpdateSMSC, AffectsTopic(TopicSMSSettings))
	opDeleteSMSC             = registerChangesetOp("DeleteSMSC", (*Database).applyDeleteSMSC, AffectsTopic(TopicSMSSettings))
	opRecordSMSWaiting       = registerChangesetOp("RecordSMSWaiting", (*Database).applyRecordSMSWaiting)
	opClearSMSMemoryFull     = registerChangesetOp("ClearSMSMemoryFull", (*Database).applyClearSMSMemoryFull)
	opRemoveSMSWaitingCentre = registerChangesetOp("RemoveSMSWaitingCentre", (*Database).applyRemoveSMSWaitingCentre)
	opDeleteSMSWaiting       = registerChangesetOp("DeleteSMSWaiting", (*Database).applyDeleteSMSWaiting)
	opDeleteStaleSMSWaiting  = registerChangesetOp("DeleteStaleSMSWaiting", (*Database).applyDeleteStaleSMSWaiting)
)

var _ = registerChangesetOp("ReplacePCSCFAddresses", (*Database).applyReplacePCSCFAddresses)

var (
	opUpdateIMSSettings = registerChangesetOp("UpdateIMSSettings", (*Database).applyUpdateIMSSettings, AffectsTopic(TopicIMSSettings))
	opCreateIMSPeer     = registerChangesetOp("CreateIMSPeer", (*Database).applyCreateIMSPeer, AffectsTopic(TopicIMSSettings))
	opUpdateIMSPeer     = registerChangesetOp("UpdateIMSPeer", (*Database).applyUpdateIMSPeer, AffectsTopic(TopicIMSSettings))
	opDeleteIMSPeer     = registerChangesetOp("DeleteIMSPeer", (*Database).applyDeleteIMSPeer, AffectsTopic(TopicIMSSettings))
)

var (
	_                               = registerChangesetOp("CompareAndSwapIMSRegistration", (*Database).applyRetiredCompareAndSwapIMSRegistration)
	opCompareAndSwapIMSRegistration = registerChangesetOpReturning[imsRegistrationCASPayload, *imsRegistrationCASResult]("CompareAndSwapIMSRegistrationSets", (*Database).applyCompareAndSwapIMSRegistration)
)

var (
	opCreateIMSApplicationServer   = registerChangesetOp("CreateIMSApplicationServer", (*Database).applyCreateIMSApplicationServer)
	opUpdateIMSApplicationServer   = registerChangesetOp("UpdateIMSApplicationServer", (*Database).applyUpdateIMSApplicationServer)
	opDeleteIMSApplicationServer   = registerChangesetOp("DeleteIMSApplicationServer", (*Database).applyDeleteIMSApplicationServer)
	opCreateIMSServiceProfile      = registerChangesetOp("CreateIMSServiceProfile", (*Database).applyCreateIMSServiceProfile)
	opUpdateIMSServiceProfile      = registerChangesetOp("UpdateIMSServiceProfile", (*Database).applyUpdateIMSServiceProfile)
	opDeleteIMSServiceProfile      = registerChangesetOp("DeleteIMSServiceProfile", (*Database).applyDeleteIMSServiceProfile)
	opSetProfileIMSServiceProfiles = registerChangesetOp("SetProfileIMSServiceProfiles", (*Database).applySetProfileIMSServiceProfiles)
	opPutIPSMGWRegistration        = registerChangesetOp("PutIPSMGWRegistration", (*Database).applyPutIPSMGWRegistration)
	opDeleteIPSMGWRegistration     = registerChangesetOp("DeleteIPSMGWRegistration", (*Database).applyDeleteIPSMGWRegistration)
)

var (
	opCreateRoamingPartner      = registerChangesetOp("CreateRoamingPartner", (*Database).applyCreateRoamingPartner, AffectsTopic(TopicRoaming))
	opUpdateRoamingPartner      = registerChangesetOp("UpdateRoamingPartner", (*Database).applyUpdateRoamingPartner, AffectsTopic(TopicRoaming), AffectsTopic(TopicSessionReconcile))
	opDeleteRoamingPartner      = registerChangesetOp("DeleteRoamingPartner", (*Database).applyDeleteRoamingPartner, AffectsTopic(TopicRoaming))
	opCreateRoamingDiameterPeer = registerChangesetOp("CreateRoamingDiameterPeer", (*Database).applyCreateRoamingDiameterPeer, AffectsTopic(TopicRoaming))
	opUpdateRoamingDiameterPeer = registerChangesetOp("UpdateRoamingDiameterPeer", (*Database).applyUpdateRoamingDiameterPeer, AffectsTopic(TopicRoaming))
	opDeleteRoamingDiameterPeer = registerChangesetOp("DeleteRoamingDiameterPeer", (*Database).applyDeleteRoamingDiameterPeer, AffectsTopic(TopicRoaming))
	opPutSEPPIdentity           = registerChangesetOp("PutSEPPIdentity", (*Database).applyPutSEPPIdentity, AffectsTopic(TopicSEPPIdentity))
	opInitSEPPCA                = registerChangesetOpReturning[SEPPCA, bool]("InitSEPPCA", (*Database).applyInitSEPPCA, AffectsTopic(TopicSEPPIdentity))
	opReplaceSEPPCA             = registerChangesetOp("ReplaceSEPPCA", (*Database).applyReplaceSEPPCA, AffectsTopic(TopicSEPPIdentity))
	opRegisterVisitorUE         = registerChangesetOpReturning[registerVisitorUEPayload, int64]("RegisterVisitorUE", (*Database).applyRegisterVisitorUE, AffectsTopic(TopicUERegistrations))
)

// Cluster PKI. cluster_join_tokens dates from v9;
// cluster_node_certs and cluster_join_hmac are added in v12.
var (
	opUpsertNodeCert        = registerChangesetOp("UpsertClusterNodeCert", (*Database).applyUpsertNodeCert, AffectsTopic(TopicClusterNodeCerts))
	opDeleteNodeCert        = registerChangesetOp("DeleteClusterNodeCert", (*Database).applyDeleteNodeCert, AffectsTopic(TopicClusterNodeCerts))
	opMintJoinToken         = registerChangesetOp("MintJoinToken", (*Database).applyInsertJoinToken)
	opConsumeJoinToken      = registerChangesetOp("ConsumeJoinToken", (*Database).applyConsumeJoinToken)
	opRedeemJoinToken       = registerChangesetOpReturning[redeemJoinTokenPayload, *RedeemJoinTokenResult]("RedeemJoinToken", (*Database).applyRedeemJoinToken, AffectsTopic(TopicClusterNodeCerts))
	opDeleteStaleJoinTokens = registerChangesetOp("DeleteStaleJoinTokens", (*Database).applyDeleteJoinTokensStale)
	opInitJoinHMAC          = registerChangesetOp("InitClusterJoinHMACKey", (*Database).applyInitJoinHMAC)
)

var opVerifyState = registerChangesetOpReturning[struct{}, uint64](verifyStateOp, (*Database).applyVerifyState)

// Intent ops — bulk deletes and migrations dispatched explicitly by the
// FSM via CommandType below commandSchema; from commandSchema on, every one
// except MigrateShared is proposed as a command. Call sites use
// intentOp.Invoke; the forwarded-op envelope carries the same name the
// leader's dispatcher looks up here.
var (
	_                        = registerIntentOp("DeleteOldAuditLogs", ellaraft.CmdDeleteOldAuditLogs)
	opDeleteOldDailyUsage    = registerIntentOp("DeleteOldDailyUsage", ellaraft.CmdDeleteOldDailyUsage, asCommand(withoutResult((*Database).applyDeleteOldDailyUsage)))
	opDeleteAllDynamicLeases = registerIntentOp("DeleteAllDynamicLeases", ellaraft.CmdDeleteAllDynamicLeases, AffectsTopic(TopicIPLeases), asCommand((*Database).applyDeleteAllDynamicLeasesCommand))
	opDeleteExpiredSessions  = registerIntentOpReturning[int]("DeleteExpiredSessions", ellaraft.CmdDeleteExpiredSessions, asCommand((*Database).applyDeleteExpiredSessions))
	opMigrateShared          = registerIntentOp("MigrateShared", ellaraft.CmdMigrateShared)
)
