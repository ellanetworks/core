// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import "github.com/ellanetworks/core/diameter/tgpp"

type ULRFlags uint32

const (
	ULRFlagSingleRegistrationIndication     ULRFlags = 1 << 0
	ULRFlagS6aS6dIndicator                  ULRFlags = 1 << 1
	ULRFlagSkipSubscriberData               ULRFlags = 1 << 2
	ULRFlagGPRSSubscriptionDataIndicator    ULRFlags = 1 << 3
	ULRFlagNodeTypeIndicator                ULRFlags = 1 << 4
	ULRFlagInitialAttachIndicator           ULRFlags = 1 << 5
	ULRFlagPSLCSNotSupportedByUE            ULRFlags = 1 << 6
	ULRFlagSMSOnlyIndication                ULRFlags = 1 << 7
	ULRFlagDualRegistration5GIndicator      ULRFlags = 1 << 8
	ULRFlagInterPLMNInterMMEHandover        ULRFlags = 1 << 9
	ULRFlagIntraPLMNInterMMEHandover        ULRFlags = 1 << 10
	ULRFlagPDNContextSyncUpNotRequested     ULRFlags = 1 << 11
	ULRFlagDisasterRoamingServiceIndication ULRFlags = 1 << 12
	ulrFlagsDefined                         ULRFlags = 1<<13 - 1
)

var ulrFlagNames = []string{
	"SINGLE_REGISTRATION_INDICATION", "S6A_S6D_INDICATOR", "SKIP_SUBSCRIBER_DATA", "GPRS_SUBSCRIPTION_DATA_INDICATOR",
	"NODE_TYPE_INDICATOR", "INITIAL_ATTACH_INDICATOR", "PS_LCS_NOT_SUPPORTED_BY_UE", "SMS_ONLY_INDICATION",
	"DUAL_REGISTRATION_5G_INDICATOR", "INTER_PLMN_INTER_MME_HANDOVER", "INTRA_PLMN_INTER_MME_HANDOVER",
	"PDN_CONTEXT_SYNC_UP_WITH_UDM_NOT_REQUESTED", "DISASTER_ROAMING_SERVICE_IN_EPS_INDICATION",
}

func (f ULRFlags) String() string { return tgpp.BitNames(uint32(f), ulrFlagNames...) }

func (f ULRFlags) Known() bool { return f&^ulrFlagsDefined == 0 }

type ULAFlags uint32

const (
	ULAFlagSeparationIndication ULAFlags = 1 << 0
	ULAFlagMMERegisteredForSMS  ULAFlags = 1 << 1
	ulaFlagsDefined             ULAFlags = 1<<2 - 1
)

var ulaFlagNames = []string{"SEPARATION_INDICATION", "MME_REGISTERED_FOR_SMS"}

func (f ULAFlags) String() string { return tgpp.BitNames(uint32(f), ulaFlagNames...) }

func (f ULAFlags) Known() bool { return f&^ulaFlagsDefined == 0 }

type AIRFlags uint32

const (
	AIRFlagSendUEUsageType                  AIRFlags = 1 << 0
	AIRFlagDisasterRoamingServiceIndication AIRFlags = 1 << 1
	airFlagsDefined                         AIRFlags = 1<<2 - 1
)

var airFlagNames = []string{"SEND_UE_USAGE_TYPE", "DISASTER_ROAMING_SERVICE_IN_EPS_INDICATION"}

func (f AIRFlags) String() string { return tgpp.BitNames(uint32(f), airFlagNames...) }

func (f AIRFlags) Known() bool { return f&^airFlagsDefined == 0 }

type CLRFlags uint32

const (
	CLRFlagS6aS6dIndicator  CLRFlags = 1 << 0
	CLRFlagReattachRequired CLRFlags = 1 << 1
	clrFlagsDefined         CLRFlags = 1<<2 - 1
)

var clrFlagNames = []string{"S6A_S6D_INDICATOR", "REATTACH_REQUIRED"}

func (f CLRFlags) String() string { return tgpp.BitNames(uint32(f), clrFlagNames...) }

func (f CLRFlags) Known() bool { return f&^clrFlagsDefined == 0 }

type IDRFlags uint32

const (
	IDRFlagUEReachabilityRequest         IDRFlags = 1 << 0
	IDRFlagTADSDataRequest               IDRFlags = 1 << 1
	IDRFlagEPSUserStateRequest           IDRFlags = 1 << 2
	IDRFlagEPSLocationInformationRequest IDRFlags = 1 << 3
	IDRFlagCurrentLocationRequest        IDRFlags = 1 << 4
	IDRFlagLocalTimeZoneRequest          IDRFlags = 1 << 5
	IDRFlagRemoveSMSRegistration         IDRFlags = 1 << 6
	IDRFlagRATTypeRequested              IDRFlags = 1 << 7
	IDRFlagPCSCFRestorationRequest       IDRFlags = 1 << 8
	idrFlagsDefined                      IDRFlags = 1<<9 - 1
)

var idrFlagNames = []string{
	"UE_REACHABILITY_REQUEST", "T_ADS_DATA_REQUEST", "EPS_USER_STATE_REQUEST", "EPS_LOCATION_INFORMATION_REQUEST",
	"CURRENT_LOCATION_REQUEST", "LOCAL_TIME_ZONE_REQUEST", "REMOVE_SMS_REGISTRATION", "RAT_TYPE_REQUESTED",
	"P_CSCF_RESTORATION_REQUEST",
}

func (f IDRFlags) String() string { return tgpp.BitNames(uint32(f), idrFlagNames...) }

func (f IDRFlags) Known() bool { return f&^idrFlagsDefined == 0 }

type IDAFlags uint32

const (
	IDAFlagNetworkNodeAreaRestricted IDAFlags = 1 << 0
	idaFlagsDefined                  IDAFlags = 1<<1 - 1
)

func (f IDAFlags) String() string { return tgpp.BitNames(uint32(f), "NETWORK_NODE_AREA_RESTRICTED") }

func (f IDAFlags) Known() bool { return f&^idaFlagsDefined == 0 }

type DSRFlags uint32

const (
	DSRFlagRegionalSubscriptionWithdrawal            DSRFlags = 1 << 0
	DSRFlagCompleteAPNConfigurationProfileWithdrawal DSRFlags = 1 << 1
	DSRFlagSubscribedChargingCharacteristics         DSRFlags = 1 << 2
	DSRFlagPDNSubscriptionContextsWithdrawal         DSRFlags = 1 << 3
	DSRFlagSTNSR                                     DSRFlags = 1 << 4
	DSRFlagCompletePDPContextListWithdrawal          DSRFlags = 1 << 5
	DSRFlagPDPContextsWithdrawal                     DSRFlags = 1 << 6
	DSRFlagRoamingRestrictedDueToUnsupportedFeature  DSRFlags = 1 << 7
	DSRFlagTraceDataWithdrawal                       DSRFlags = 1 << 8
	DSRFlagCSGDeleted                                DSRFlags = 1 << 9
	DSRFlagAPNOIReplacement                          DSRFlags = 1 << 10
	DSRFlagGMLCListWithdrawal                        DSRFlags = 1 << 11
	DSRFlagLCSWithdrawal                             DSRFlags = 1 << 12
	DSRFlagSMSWithdrawal                             DSRFlags = 1 << 13
	DSRFlagSubscribedPeriodicRAUTAUTimerWithdrawal   DSRFlags = 1 << 14
	DSRFlagSubscribedVSRVCCWithdrawal                DSRFlags = 1 << 15
	DSRFlagAMSISDNWithdrawal                         DSRFlags = 1 << 16
	DSRFlagProSeWithdrawal                           DSRFlags = 1 << 17
	DSRFlagResetIDs                                  DSRFlags = 1 << 18
	DSRFlagDLBufferingSuggestedPacketCount           DSRFlags = 1 << 19
	DSRFlagSubscribedIMSIGroupID                     DSRFlags = 1 << 20
	DSRFlagDeleteMonitoringEvents                    DSRFlags = 1 << 21
	DSRFlagUserPlaneIntegrityProtection              DSRFlags = 1 << 22
	DSRFlagMSISDNWithdrawal                          DSRFlags = 1 << 23
	DSRFlagUEUsageTypeWithdrawal                     DSRFlags = 1 << 24
	DSRFlagV2XWithdrawal                             DSRFlags = 1 << 25
	DSRFlagExternalIdentifierWithdrawal              DSRFlags = 1 << 26
	DSRFlagAerialUESubscriptionWithdrawal            DSRFlags = 1 << 27
	DSRFlagPagingTimeWindowWithdrawal                DSRFlags = 1 << 28
	DSRFlagActiveTimeWithdrawal                      DSRFlags = 1 << 29
	DSRFlagEDRXCycleLengthWithdrawal                 DSRFlags = 1 << 30
	DSRFlagServiceGapTimeWithdrawal                  DSRFlags = 1 << 31
	dsrPDNAndPDPContextsWithdrawal                   DSRFlags = DSRFlagPDNSubscriptionContextsWithdrawal | DSRFlagPDPContextsWithdrawal
)

var dsrFlagNames = []string{
	"REGIONAL_SUBSCRIPTION_WITHDRAWAL", "COMPLETE_APN_CONFIGURATION_PROFILE_WITHDRAWAL",
	"SUBSCRIBED_CHARGING_CHARACTERISTICS_WITHDRAWAL", "PDN_SUBSCRIPTION_CONTEXTS_WITHDRAWAL", "STN_SR",
	"COMPLETE_PDP_CONTEXT_LIST_WITHDRAWAL", "PDP_CONTEXTS_WITHDRAWAL", "ROAMING_RESTRICTED_DUE_TO_UNSUPPORTED_FEATURE",
	"TRACE_DATA_WITHDRAWAL", "CSG_DELETED", "APN_OI_REPLACEMENT", "GMLC_LIST_WITHDRAWAL", "LCS_WITHDRAWAL",
	"SMS_WITHDRAWAL", "SUBSCRIBED_PERIODIC_RAU_TAU_TIMER_WITHDRAWAL", "SUBSCRIBED_VSRVCC_WITHDRAWAL", "A_MSISDN_WITHDRAWAL",
	"PROSE_WITHDRAWAL", "RESET_IDS", "DL_BUFFERING_SUGGESTED_PACKET_COUNT_WITHDRAWAL", "SUBSCRIBED_IMSI_GROUP_ID_WITHDRAWAL",
	"DELETE_MONITORING_EVENTS", "USER_PLANE_INTEGRITY_PROTECTION_WITHDRAWAL", "MSISDN_WITHDRAWAL",
	"UE_USAGE_TYPE_WITHDRAWAL", "V2X_WITHDRAWAL", "EXTERNAL_IDENTIFIER_WITHDRAWAL", "AERIAL_UE_SUBSCRIPTION_WITHDRAWAL",
	"PAGING_TIME_WINDOW_SUBSCRIPTION_WITHDRAWAL", "ACTIVE_TIME_WITHDRAWAL", "EDRX_CYCLE_LENGTH_WITHDRAWAL",
	"SERVICE_GAP_TIME_WITHDRAWAL",
}

func (f DSRFlags) String() string { return tgpp.BitNames(uint32(f), dsrFlagNames...) }

func (f DSRFlags) Known() bool {
	return f&dsrPDNAndPDPContextsWithdrawal != dsrPDNAndPDPContextsWithdrawal
}

type DSAFlags uint32

const (
	DSAFlagNetworkNodeAreaRestricted DSAFlags = 1 << 0
	dsaFlagsDefined                  DSAFlags = 1<<1 - 1
)

func (f DSAFlags) String() string { return tgpp.BitNames(uint32(f), "NETWORK_NODE_AREA_RESTRICTED") }

func (f DSAFlags) Known() bool { return f&^dsaFlagsDefined == 0 }

type PURFlags uint32

const (
	PURFlagUEPurgedInMME  PURFlags = 1 << 0
	PURFlagUEPurgedInSGSN PURFlags = 1 << 1
	purFlagsDefined       PURFlags = 1<<2 - 1
)

var purFlagNames = []string{"UE_PURGED_IN_MME", "UE_PURGED_IN_SGSN"}

func (f PURFlags) String() string { return tgpp.BitNames(uint32(f), purFlagNames...) }

func (f PURFlags) Known() bool { return f&^purFlagsDefined == 0 }

type PUAFlags uint32

const (
	PUAFlagFreezeMTMSI PUAFlags = 1 << 0
	PUAFlagFreezePTMSI PUAFlags = 1 << 1
	puaFlagsDefined    PUAFlags = 1<<2 - 1
)

var puaFlagNames = []string{"FREEZE_M_TMSI", "FREEZE_P_TMSI"}

func (f PUAFlags) String() string { return tgpp.BitNames(uint32(f), puaFlagNames...) }

func (f PUAFlags) Known() bool { return f&^puaFlagsDefined == 0 }

type NORFlags uint32

const (
	NORFlagSingleRegistrationIndication       NORFlags = 1 << 0
	NORFlagSGSNAreaRestricted                 NORFlags = 1 << 1
	NORFlagReadyForSMFromSGSN                 NORFlags = 1 << 2
	NORFlagUEReachableFromMME                 NORFlags = 1 << 3
	NORFlagUEReachableFromSGSN                NORFlags = 1 << 5
	NORFlagReadyForSMFromMME                  NORFlags = 1 << 6
	NORFlagHomogeneousSupportOfIMSVoiceOverPS NORFlags = 1 << 7
	NORFlagS6aS6dIndicator                    NORFlags = 1 << 8
	NORFlagRemovalOfMMERegistrationForSMS     NORFlags = 1 << 9
	norFlagsDefined                           NORFlags = (1<<10 - 1) &^ (1 << 4)
)

var norFlagNames = []string{
	"SINGLE_REGISTRATION_INDICATION", "SGSN_AREA_RESTRICTED", "READY_FOR_SM_FROM_SGSN", "UE_REACHABLE_FROM_MME",
	"RESERVED", "UE_REACHABLE_FROM_SGSN", "READY_FOR_SM_FROM_MME", "HOMOGENEOUS_SUPPORT_OF_IMS_VOICE_OVER_PS_SESSIONS",
	"S6A_S6D_INDICATOR", "REMOVAL_OF_MME_REGISTRATION_FOR_SMS",
}

func (f NORFlags) String() string { return tgpp.BitNames(uint32(f), norFlagNames...) }

func (f NORFlags) Known() bool { return f&^norFlagsDefined == 0 }

type OperatorDeterminedBarring uint32

const (
	ODBAllPacketOrientedServices                        OperatorDeterminedBarring = 1 << 0
	ODBRoamerAccessHPLMNAP                              OperatorDeterminedBarring = 1 << 1
	ODBRoamerAccessVPLMNAP                              OperatorDeterminedBarring = 1 << 2
	ODBAllOutgoingCalls                                 OperatorDeterminedBarring = 1 << 3
	ODBAllOutgoingInternationalCalls                    OperatorDeterminedBarring = 1 << 4
	ODBAllOutgoingInternationalCallsExceptHPLMNCountry  OperatorDeterminedBarring = 1 << 5
	ODBAllOutgoingInterzonalCalls                       OperatorDeterminedBarring = 1 << 6
	ODBAllOutgoingInterzonalCallsExceptHPLMNCountry     OperatorDeterminedBarring = 1 << 7
	ODBAllOutgoingInternationalExceptHPLMNAndInterzonal OperatorDeterminedBarring = 1 << 8
	odbDefined                                          OperatorDeterminedBarring = 1<<9 - 1
)

var odbNames = []string{
	"ALL_PACKET_ORIENTED_SERVICES_BARRED", "ROAMER_ACCESS_HPLMN_AP_BARRED", "ROAMER_ACCESS_VPLMN_AP_BARRED",
	"BARRING_OF_ALL_OUTGOING_CALLS", "BARRING_OF_ALL_OUTGOING_INTERNATIONAL_CALLS",
	"BARRING_OF_ALL_OUTGOING_INTERNATIONAL_CALLS_EXCEPT_HPLMN_COUNTRY", "BARRING_OF_ALL_OUTGOING_INTERZONAL_CALLS",
	"BARRING_OF_ALL_OUTGOING_INTERZONAL_CALLS_EXCEPT_HPLMN_COUNTRY",
	"BARRING_OF_ALL_OUTGOING_INTERNATIONAL_EXCEPT_HPLMN_COUNTRY_AND_INTERZONAL",
}

func (b OperatorDeterminedBarring) String() string { return tgpp.BitNames(uint32(b), odbNames...) }

func (b OperatorDeterminedBarring) Known() bool { return b&^odbDefined == 0 }

type HPLMNODB uint32

const (
	HPLMNODBType1   HPLMNODB = 1 << 0
	HPLMNODBType2   HPLMNODB = 1 << 1
	HPLMNODBType3   HPLMNODB = 1 << 2
	HPLMNODBType4   HPLMNODB = 1 << 3
	hplmnODBDefined HPLMNODB = 1<<4 - 1
)

var hplmnODBNames = []string{"HPLMN_SPECIFIC_BARRING_TYPE_1", "HPLMN_SPECIFIC_BARRING_TYPE_2", "HPLMN_SPECIFIC_BARRING_TYPE_3", "HPLMN_SPECIFIC_BARRING_TYPE_4"}

func (b HPLMNODB) String() string { return tgpp.BitNames(uint32(b), hplmnODBNames...) }

func (b HPLMNODB) Known() bool { return b&^hplmnODBDefined == 0 }

type AccessRestrictionData uint32

const (
	ARDUTRANNotAllowed                AccessRestrictionData = 1 << 0
	ARDGERANNotAllowed                AccessRestrictionData = 1 << 1
	ARDGANNotAllowed                  AccessRestrictionData = 1 << 2
	ARDIHSPAEvolutionNotAllowed       AccessRestrictionData = 1 << 3
	ARDWBEUTRANNotAllowed             AccessRestrictionData = 1 << 4
	ARDHOToNon3GPPAccessNotAllowed    AccessRestrictionData = 1 << 5
	ARDNBIoTNotAllowed                AccessRestrictionData = 1 << 6
	ARDEnhancedCoverageNotAllowed     AccessRestrictionData = 1 << 7
	ARDNRAsSecondaryRATNotAllowed     AccessRestrictionData = 1 << 8
	ARDUnlicensedSpectrumSecondaryRAT AccessRestrictionData = 1 << 9
	ARDNRIn5GSNotAllowed              AccessRestrictionData = 1 << 10
	ARDLTEMNotAllowed                 AccessRestrictionData = 1 << 11
	ARDWBEUTRANExceptLTEMNotAllowed   AccessRestrictionData = 1 << 12
	ARDWBEUTRANLEONotAllowed          AccessRestrictionData = 1 << 13
	ARDWBEUTRANMEONotAllowed          AccessRestrictionData = 1 << 14
	ARDWBEUTRANGEONotAllowed          AccessRestrictionData = 1 << 15
	ARDWBEUTRANOtherSatNotAllowed     AccessRestrictionData = 1 << 16
	ARDNBIoTLEONotAllowed             AccessRestrictionData = 1 << 17
	ARDNBIoTMEONotAllowed             AccessRestrictionData = 1 << 18
	ARDNBIoTGEONotAllowed             AccessRestrictionData = 1 << 19
	ARDNBIoTOtherSatNotAllowed        AccessRestrictionData = 1 << 20
	ARDLTEMLEONotAllowed              AccessRestrictionData = 1 << 21
	ARDLTEMMEONotAllowed              AccessRestrictionData = 1 << 22
	ARDLTEMGEONotAllowed              AccessRestrictionData = 1 << 23
	ARDLTEMOtherSatNotAllowed         AccessRestrictionData = 1 << 24
	ARDNRLEONotAllowed                AccessRestrictionData = 1 << 25
	ARDNRMEONotAllowed                AccessRestrictionData = 1 << 26
	ARDNRGEONotAllowed                AccessRestrictionData = 1 << 27
	ARDNROtherSatNotAllowed           AccessRestrictionData = 1 << 28
	ARDNRRedCapNotAllowed             AccessRestrictionData = 1 << 29
	ARDNREnhancedRedCapNotAllowed     AccessRestrictionData = 1 << 30
	ardDefined                        AccessRestrictionData = 1<<31 - 1
)

var ardNames = []string{
	"UTRAN_NOT_ALLOWED", "GERAN_NOT_ALLOWED", "GAN_NOT_ALLOWED", "I_HSPA_EVOLUTION_NOT_ALLOWED", "WB_E_UTRAN_NOT_ALLOWED",
	"HO_TO_NON_3GPP_ACCESS_NOT_ALLOWED", "NB_IOT_NOT_ALLOWED", "ENHANCED_COVERAGE_NOT_ALLOWED",
	"NR_AS_SECONDARY_RAT_NOT_ALLOWED", "UNLICENSED_SPECTRUM_AS_SECONDARY_RAT_NOT_ALLOWED", "NR_IN_5GS_NOT_ALLOWED",
	"LTE_M_NOT_ALLOWED", "WB_E_UTRAN_EXCEPT_LTE_M_NOT_ALLOWED", "WB_E_UTRAN_LEO_NOT_ALLOWED", "WB_E_UTRAN_MEO_NOT_ALLOWED",
	"WB_E_UTRAN_GEO_NOT_ALLOWED", "WB_E_UTRAN_OTHERSAT_NOT_ALLOWED", "NB_IOT_LEO_NOT_ALLOWED", "NB_IOT_MEO_NOT_ALLOWED",
	"NB_IOT_GEO_NOT_ALLOWED", "NB_IOT_OTHERSAT_NOT_ALLOWED", "LTE_M_LEO_NOT_ALLOWED", "LTE_M_MEO_NOT_ALLOWED",
	"LTE_M_GEO_NOT_ALLOWED", "LTE_M_OTHERSAT_NOT_ALLOWED", "NR_LEO_NOT_ALLOWED", "NR_MEO_NOT_ALLOWED", "NR_GEO_NOT_ALLOWED",
	"NR_OTHERSAT_NOT_ALLOWED", "NR_REDCAP_NOT_ALLOWED", "NR_EREDCAP_NOT_ALLOWED",
}

func (d AccessRestrictionData) String() string { return tgpp.BitNames(uint32(d), ardNames...) }

func (d AccessRestrictionData) Known() bool { return d&^ardDefined == 0 }

type FeatureList1 uint32

const (
	FeatureODBAllAPN                              FeatureList1 = 1 << 0
	FeatureODBHPLMNAPN                            FeatureList1 = 1 << 1
	FeatureODBVPLMNAPN                            FeatureList1 = 1 << 2
	FeatureODBAllOG                               FeatureList1 = 1 << 3
	FeatureODBAllInternationalOG                  FeatureList1 = 1 << 4
	FeatureODBAllInternationalOGNotToHPLMNCountry FeatureList1 = 1 << 5
	FeatureODBAllInterzonalOG                     FeatureList1 = 1 << 6
	FeatureODBAllInterzonalOGNotToHPLMNCountry    FeatureList1 = 1 << 7
	FeatureODBAllInterzonalOGAndInternationalOG   FeatureList1 = 1 << 8
	FeatureRegSub                                 FeatureList1 = 1 << 9
	FeatureTrace                                  FeatureList1 = 1 << 10
	FeatureLCSAllPrivExcep                        FeatureList1 = 1 << 11
	FeatureLCSUniversal                           FeatureList1 = 1 << 12
	FeatureLCSCallSessionRelated                  FeatureList1 = 1 << 13
	FeatureLCSCallSessionUnrelated                FeatureList1 = 1 << 14
	FeatureLCSPLMNOperator                        FeatureList1 = 1 << 15
	FeatureLCSServiceType                         FeatureList1 = 1 << 16
	FeatureLCSAllMOLRSS                           FeatureList1 = 1 << 17
	FeatureLCSBasicSelfLocation                   FeatureList1 = 1 << 18
	FeatureLCSAutonomousSelfLocation              FeatureList1 = 1 << 19
	FeatureLCSTransferToThirdParty                FeatureList1 = 1 << 20
	FeatureSMMOPP                                 FeatureList1 = 1 << 21
	FeatureBarringOutgoingCalls                   FeatureList1 = 1 << 22
	FeatureBAOC                                   FeatureList1 = 1 << 23
	FeatureBOIC                                   FeatureList1 = 1 << 24
	FeatureBOICExHC                               FeatureList1 = 1 << 25
	FeatureUEReachabilityNotification             FeatureList1 = 1 << 26
	FeatureTADSDataRetrieval                      FeatureList1 = 1 << 27
	FeatureStateLocationInformationRetrieval      FeatureList1 = 1 << 28
	FeaturePartialPurge                           FeatureList1 = 1 << 29
	FeatureLocalTimeZoneRetrieval                 FeatureList1 = 1 << 30
	FeatureAdditionalMSISDN                       FeatureList1 = 1 << 31
)

var featureList1Names = []string{
	"ODB-all-APN", "ODB-HPLMN-APN", "ODB-VPLMN-APN", "ODB-all-OG", "ODB-all-InternationalOG",
	"ODB-all-InternationalOGNotToHPLMN-Country", "ODB-all-InterzonalOG", "ODB-all-InterzonalOGNotToHPLMN-Country",
	"ODB-all-InterzonalOGAndInternationalOGNotToHPLMN-Country", "RegSub", "Trace", "LCS-all-PrivExcep", "LCS-Universal",
	"LCS-CallSessionRelated", "LCS-CallSessionUnrelated", "LCS-PLMNOperator", "LCS-ServiceType", "LCS-all-MOLR-SS",
	"LCS-BasicSelfLocation", "LCS-AutonomousSelfLocation", "LCS-TransferToThirdParty", "SM-MO-PP",
	"Barring-OutgoingCalls", "BAOC", "BOIC", "BOICExHC", "UE-Reachability-Notification", "T-ADS Data Retrieval",
	"State/Location-Information-Retrieval", "Partial Purge", "Local Time Zone Retrieval", "Additional MSISDN",
}

func (f FeatureList1) String() string { return tgpp.BitNames(uint32(f), featureList1Names...) }

type FeatureList2 uint32

const (
	FeatureSMSInMME                       FeatureList2 = 1 << 0
	FeatureSMSInSGSN                      FeatureList2 = 1 << 1
	FeatureDiaLCSAllPrivExcep             FeatureList2 = 1 << 2
	FeatureDiaLCSUniversal                FeatureList2 = 1 << 3
	FeatureDiaLCSCallSessionRelated       FeatureList2 = 1 << 4
	FeatureDiaLCSCallSessionUnrelated     FeatureList2 = 1 << 5
	FeatureDiaLCSPLMNOperator             FeatureList2 = 1 << 6
	FeatureDiaLCSServiceType              FeatureList2 = 1 << 7
	FeatureDiaLCSAllMOLRSS                FeatureList2 = 1 << 8
	FeatureDiaLCSBasicSelfLocation        FeatureList2 = 1 << 9
	FeatureDiaLCSAutonomousSelfLocation   FeatureList2 = 1 << 10
	FeatureDiaLCSTransferToThirdParty     FeatureList2 = 1 << 11
	FeatureGddInSGSN                      FeatureList2 = 1 << 12
	FeatureOptimizedLCSProcSupport        FeatureList2 = 1 << 13
	FeatureSGSNCAMELCapability            FeatureList2 = 1 << 14
	FeatureProSeCapability                FeatureList2 = 1 << 15
	FeaturePCSCFRestoration               FeatureList2 = 1 << 16
	FeatureResetIDs                       FeatureList2 = 1 << 17
	FeatureCommunicationPattern           FeatureList2 = 1 << 18
	FeatureMonitoringEvent                FeatureList2 = 1 << 19
	FeatureDedicatedCoreNetworks          FeatureList2 = 1 << 20
	FeatureNonIPPDNTypeAPNs               FeatureList2 = 1 << 21
	FeatureNonIPPDPTypeAPNs               FeatureList2 = 1 << 22
	FeatureRemovalOfMSISDN                FeatureList2 = 1 << 23
	FeatureEmergencyServiceContinuity     FeatureList2 = 1 << 24
	FeatureV2XCapability                  FeatureList2 = 1 << 25
	FeatureExternalIdentifier             FeatureList2 = 1 << 26
	FeatureNRAsSecondaryRAT               FeatureList2 = 1 << 27
	FeatureUnlicensedSpectrumSecondaryRAT FeatureList2 = 1 << 28
	FeatureEthernetPDNTypeAPNs            FeatureList2 = 1 << 29
	FeatureExtendedReferenceIDs           FeatureList2 = 1 << 30
	FeatureSatelliteStoreAndForward       FeatureList2 = 1 << 31
)

var featureList2Names = []string{
	"SMS in MME", "SMS in SGSN", "Dia-LCS-all-PrivExcep", "Dia-LCS-Universal", "Dia-LCS-CallSessionRelated",
	"Dia-LCS-CallSessionUnrelated", "Dia-LCS-PLMNOperator", "Dia-LCS-ServiceType", "Dia-LCS-all-MOLR-SS",
	"Dia-LCS-BasicSelfLocation", "Dia-LCS-AutonomousSelfLocation", "Dia-LCS-TransferToThirdParty", "Gdd-in-SGSN",
	"Optimized-LCS-Proc-Support", "SGSN CAMEL Capability", "ProSe Capability", "P-CSCF Restoration", "Reset-IDs",
	"Communication-Pattern", "Monitoring-Event", "Dedicated Core Networks", "Non-IP PDN Type APNs",
	"Non-IP PDP Type APNs", "Removal of MSISDN", "Emergency Service Continuity", "V2X Capability",
	"External-Identifier", "NR as Secondary RAT", "Unlicensed Spectrum as Secondary RAT", "Ethernet PDN Type APNs",
	"Extended Reference IDs", "Satellite S&F",
}

func (f FeatureList2) String() string { return tgpp.BitNames(uint32(f), featureList2Names...) }
