// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"fmt"
	"strings"

	"github.com/ellanetworks/core/diameter/tgpp"
)

type AbortCause uint32

const (
	AbortBearerReleased                      AbortCause = 0
	AbortInsufficientServerResources         AbortCause = 1
	AbortInsufficientBearerResources         AbortCause = 2
	AbortPSToCSHandover                      AbortCause = 3
	AbortSponsoredDataConnectivityDisallowed AbortCause = 4
	AbortPCEFFailure                         AbortCause = 5
)

var abortCauseNames = tgpp.EnumNames{
	"BEARER_RELEASED", "INSUFFICIENT_SERVER_RESOURCES", "INSUFFICIENT_BEARER_RESOURCES", "PS_TO_CS_HANDOVER",
	"SPONSORED_DATA_CONNECTIVITY_DISALLOWED", "PCEF_FAILURE",
}

func (c AbortCause) String() string { return abortCauseNames.Name("AbortCause", uint32(c)) }

func (c AbortCause) valid() bool { return abortCauseNames.Has(uint32(c)) }

type TerminationCause uint32

const (
	TerminationLogout                  TerminationCause = 1
	TerminationServiceNotProvided      TerminationCause = 2
	TerminationBadAnswer               TerminationCause = 3
	TerminationAdministrative          TerminationCause = 4
	TerminationLinkBroken              TerminationCause = 5
	TerminationAuthExpired             TerminationCause = 6
	TerminationUserMoved               TerminationCause = 7
	TerminationSessionTimeout          TerminationCause = 8
	TerminationUserRequest             TerminationCause = 11
	TerminationLostCarrier             TerminationCause = 12
	TerminationLostService             TerminationCause = 13
	TerminationIdleTimeout             TerminationCause = 14
	TerminationNASSessionTimeout       TerminationCause = 15
	TerminationAdminReset              TerminationCause = 16
	TerminationAdminReboot             TerminationCause = 17
	TerminationPortError               TerminationCause = 18
	TerminationNASError                TerminationCause = 19
	TerminationNASRequest              TerminationCause = 20
	TerminationNASReboot               TerminationCause = 21
	TerminationPortUnneeded            TerminationCause = 22
	TerminationPortPreempted           TerminationCause = 23
	TerminationPortSuspended           TerminationCause = 24
	TerminationServiceUnavailable      TerminationCause = 25
	TerminationCallback                TerminationCause = 26
	TerminationUserError               TerminationCause = 27
	TerminationHostRequest             TerminationCause = 28
	TerminationSupplicantRestart       TerminationCause = 29
	TerminationReauthenticationFailure TerminationCause = 30
	TerminationPortReinit              TerminationCause = 31
	TerminationPortDisabled            TerminationCause = 32
)

var terminationCauseNames = tgpp.EnumNames{
	"", "DIAMETER_LOGOUT", "DIAMETER_SERVICE_NOT_PROVIDED", "DIAMETER_BAD_ANSWER", "DIAMETER_ADMINISTRATIVE",
	"DIAMETER_LINK_BROKEN", "DIAMETER_AUTH_EXPIRED", "DIAMETER_USER_MOVED", "DIAMETER_SESSION_TIMEOUT", "", "",
	"USER_REQUEST", "LOST_CARRIER", "LOST_SERVICE", "IDLE_TIMEOUT", "SESSION_TIMEOUT", "ADMIN_RESET", "ADMIN_REBOOT",
	"PORT_ERROR", "NAS_ERROR", "NAS_REQUEST", "NAS_REBOOT", "PORT_UNNEEDED", "PORT_PREEMPTED", "PORT_SUSPENDED",
	"SERVICE_UNAVAILABLE", "CALLBACK", "USER_ERROR", "HOST_REQUEST", "SUPPLICANT_RESTART", "REAUTHENTICATION_FAILURE",
	"PORT_REINIT", "PORT_DISABLED",
}

func (c TerminationCause) String() string {
	return terminationCauseNames.Name("TerminationCause", uint32(c))
}

func (c TerminationCause) valid() bool { return terminationCauseNames.Has(uint32(c)) }

type SpecificAction uint32

const (
	ActionChargingCorrelationExchange                SpecificAction = 1
	ActionIndicationOfLossOfBearer                   SpecificAction = 2
	ActionIndicationOfRecoveryOfBearer               SpecificAction = 3
	ActionIndicationOfReleaseOfBearer                SpecificAction = 4
	ActionIPCANChange                                SpecificAction = 6
	ActionIndicationOfOutOfCredit                    SpecificAction = 7
	ActionIndicationOfSuccessfulResourcesAllocation  SpecificAction = 8
	ActionIndicationOfFailedResourcesAllocation      SpecificAction = 9
	ActionIndicationOfLimitedPCCDeployment           SpecificAction = 10
	ActionUsageReport                                SpecificAction = 11
	ActionAccessNetworkInfoReport                    SpecificAction = 12
	ActionIndicationOfRecoveryFromLimitedPCC         SpecificAction = 13
	ActionIndicationOfAccessNetworkInfoReportFailure SpecificAction = 14
	ActionIndicationOfTransferPolicyExpired          SpecificAction = 15
	ActionPLMNChange                                 SpecificAction = 16
	ActionEPSFallback                                SpecificAction = 17
	ActionIndicationOfReallocationOfCredit           SpecificAction = 18
	ActionSuccessfulQoSUpdate                        SpecificAction = 19
	ActionFailedQoSUpdate                            SpecificAction = 20
	ActionCNHealthMonitor                            SpecificAction = 21
)

var specificActionNames = tgpp.EnumNames{
	"", "CHARGING_CORRELATION_EXCHANGE", "INDICATION_OF_LOSS_OF_BEARER", "INDICATION_OF_RECOVERY_OF_BEARER",
	"INDICATION_OF_RELEASE_OF_BEARER", "", "IP-CAN_CHANGE", "INDICATION_OF_OUT_OF_CREDIT",
	"INDICATION_OF_SUCCESSFUL_RESOURCES_ALLOCATION", "INDICATION_OF_FAILED_RESOURCES_ALLOCATION",
	"INDICATION_OF_LIMITED_PCC_DEPLOYMENT", "USAGE_REPORT", "ACCESS_NETWORK_INFO_REPORT",
	"INDICATION_OF_RECOVERY_FROM_LIMITED_PCC_DEPLOYMENT", "INDICATION_OF_ACCESS_NETWORK_INFO_REPORTING_FAILURE",
	"INDICATION_OF_TRANSFER_POLICY_EXPIRED", "PLMN_CHANGE", "EPS_FALLBACK", "INDICATION_OF_REALLOCATION_OF_CREDIT",
	"SUCCESSFUL_QOS_UPDATE", "FAILED_QOS_UPDATE", "CN_HEALTH_MONITOR",
}

func (a SpecificAction) String() string { return specificActionNames.Name("SpecificAction", uint32(a)) }

func (a SpecificAction) valid() bool { return specificActionNames.Has(uint32(a)) }

func (a SpecificAction) void() bool { return a == 0 || a == 5 }

type RequestType uint32

const (
	RequestInitial          RequestType = 0
	RequestUpdate           RequestType = 1
	RequestPCSCFRestoration RequestType = 2
)

var requestTypeNames = tgpp.EnumNames{"INITIAL_REQUEST", "UPDATE_REQUEST", "PCSCF_RESTORATION"}

func (t RequestType) String() string { return requestTypeNames.Name("RequestType", uint32(t)) }

func (t RequestType) valid() bool { return requestTypeNames.Has(uint32(t)) }

type ServiceInfoStatus uint32

const (
	ServiceInfoFinal       ServiceInfoStatus = 0
	ServiceInfoPreliminary ServiceInfoStatus = 1
)

var serviceInfoStatusNames = tgpp.EnumNames{"FINAL_SERVICE_INFORMATION", "PRELIMINARY_SERVICE_INFORMATION"}

func (s ServiceInfoStatus) String() string {
	return serviceInfoStatusNames.Name("ServiceInfoStatus", uint32(s))
}

func (s ServiceInfoStatus) valid() bool { return serviceInfoStatusNames.Has(uint32(s)) }

type SIPForkingIndication uint32

const (
	ForkingSingleDialogue   SIPForkingIndication = 0
	ForkingSeveralDialogues SIPForkingIndication = 1
)

var sipForkingIndicationNames = tgpp.EnumNames{"SINGLE_DIALOGUE", "SEVERAL_DIALOGUES"}

func (f SIPForkingIndication) String() string {
	return sipForkingIndicationNames.Name("SIPForkingIndication", uint32(f))
}

func (f SIPForkingIndication) valid() bool { return sipForkingIndicationNames.Has(uint32(f)) }

type MediaType uint32

const (
	MediaAudio       MediaType = 0
	MediaVideo       MediaType = 1
	MediaData        MediaType = 2
	MediaApplication MediaType = 3
	MediaControl     MediaType = 4
	MediaText        MediaType = 5
	MediaMessage     MediaType = 6
	MediaOther       MediaType = 0xffffffff
)

var mediaTypeNames = tgpp.EnumNames{"AUDIO", "VIDEO", "DATA", "APPLICATION", "CONTROL", "TEXT", "MESSAGE"}

func (t MediaType) String() string {
	if t == MediaOther {
		return "OTHER"
	}

	return mediaTypeNames.Name("MediaType", uint32(t))
}

func (t MediaType) valid() bool { return t == MediaOther || mediaTypeNames.Has(uint32(t)) }

type FlowStatus uint32

const (
	FlowStatusEnabledUplink   FlowStatus = 0
	FlowStatusEnabledDownlink FlowStatus = 1
	FlowStatusEnabled         FlowStatus = 2
	FlowStatusDisabled        FlowStatus = 3
	FlowStatusRemoved         FlowStatus = 4
)

var flowStatusNames = tgpp.EnumNames{"ENABLED-UPLINK", "ENABLED-DOWNLINK", "ENABLED", "DISABLED", "REMOVED"}

func (s FlowStatus) String() string { return flowStatusNames.Name("FlowStatus", uint32(s)) }

func (s FlowStatus) valid() bool { return flowStatusNames.Has(uint32(s)) }

type FlowUsage uint32

const (
	FlowUsageNoInformation FlowUsage = 0
	FlowUsageRTCP          FlowUsage = 1
	FlowUsageAFSignalling  FlowUsage = 2
)

var flowUsageNames = tgpp.EnumNames{"NO_INFORMATION", "RTCP", "AF_SIGNALLING"}

func (u FlowUsage) String() string { return flowUsageNames.Name("FlowUsage", uint32(u)) }

func (u FlowUsage) valid() bool { return flowUsageNames.Has(uint32(u)) }

type AFSignallingProtocol uint32

const (
	SignallingProtocolNoInformation AFSignallingProtocol = 0
	SignallingProtocolSIP           AFSignallingProtocol = 1
)

var signallingProtocolNames = tgpp.EnumNames{"NO_INFORMATION", "SIP"}

func (p AFSignallingProtocol) String() string {
	return signallingProtocolNames.Name("AFSignallingProtocol", uint32(p))
}

func (p AFSignallingProtocol) valid() bool { return signallingProtocolNames.Has(uint32(p)) }

type SubscriptionIDType uint32

const (
	SubscriptionIDE164    SubscriptionIDType = 0
	SubscriptionIDIMSI    SubscriptionIDType = 1
	SubscriptionIDSIPURI  SubscriptionIDType = 2
	SubscriptionIDNAI     SubscriptionIDType = 3
	SubscriptionIDPrivate SubscriptionIDType = 4
)

var subscriptionIDTypeNames = tgpp.EnumNames{"END_USER_E164", "END_USER_IMSI", "END_USER_SIP_URI", "END_USER_NAI", "END_USER_PRIVATE"}

func (t SubscriptionIDType) String() string {
	return subscriptionIDTypeNames.Name("SubscriptionIDType", uint32(t))
}

func (t SubscriptionIDType) valid() bool { return subscriptionIDTypeNames.Has(uint32(t)) }

type CodecDirection uint8

const (
	CodecUplink   CodecDirection = 0
	CodecDownlink CodecDirection = 1
)

var codecDirectionNames = tgpp.EnumNames{"uplink", "downlink"}

func (d CodecDirection) String() string { return codecDirectionNames.Name("CodecDirection", uint32(d)) }

func (d CodecDirection) valid() bool { return codecDirectionNames.Has(uint32(d)) }

type CodecKind uint8

const (
	CodecOffer       CodecKind = 0
	CodecAnswer      CodecKind = 1
	CodecDescription CodecKind = 2
)

var codecKindNames = tgpp.EnumNames{"offer", "answer", "description"}

func (k CodecKind) String() string { return codecKindNames.Name("CodecKind", uint32(k)) }

func (k CodecKind) valid() bool { return codecKindNames.Has(uint32(k)) }

type FlowDirection uint8

const (
	FlowDirectionIn  FlowDirection = 0
	FlowDirectionOut FlowDirection = 1
)

var flowDirectionNames = tgpp.EnumNames{"in", "out"}

func (d FlowDirection) String() string { return flowDirectionNames.Name("FlowDirection", uint32(d)) }

func (d FlowDirection) valid() bool { return flowDirectionNames.Has(uint32(d)) }

type Features uint64

const (
	FeatureRel8                     Features = 1 << 0
	FeatureRel9                     Features = 1 << 1
	FeatureProvAFSignalFlow         Features = 1 << 2
	FeatureSponsoredConnectivity    Features = 1 << 3
	FeatureRel10                    Features = 1 << 4
	FeatureNetLoc                   Features = 1 << 5
	FeatureExtendedFilter           Features = 1 << 6
	FeatureSCTimeBasedUM            Features = 1 << 7
	FeatureNetLocTrustedWLAN        Features = 1 << 8
	FeatureRANNASCause              Features = 1 << 9
	FeatureGroupComService          Features = 1 << 10
	FeatureResShare                 Features = 1 << 11
	FeatureDeferredService          Features = 1 << 12
	FeatureDSCP                     Features = 1 << 13
	FeatureSponsorChange            Features = 1 << 14
	FeatureE2EQoSMTSI               Features = 1 << 15
	FeatureNetLocUntrustedWLAN      Features = 1 << 16
	FeatureMCPTT                    Features = 1 << 17
	FeaturePrioritySharing          Features = 1 << 18
	FeaturePLMNInfo                 Features = 1 << 19
	FeatureMediaComponentVersioning Features = 1 << 20
	FeatureMCPTTPreemption          Features = 1 << 21
	FeatureMCVideo                  Features = 1 << 22

	FeaturePCSCFRestorationEnhancement   Features = 1 << 32
	FeatureExtendedMaxRequestedBWNR      Features = 1 << 33
	FeatureExtendedMinRequestedBWNR      Features = 1 << 34
	FeatureExtendedBWE2EQoSMTSINR        Features = 1 << 35
	FeatureVBC                           Features = 1 << 36
	FeatureCHEM                          Features = 1 << 37
	FeatureVBCLTE                        Features = 1 << 38
	FeatureFLUS                          Features = 1 << 39
	FeatureEPSFallbackReport             Features = 1 << 40
	FeatureATSSS                         Features = 1 << 41
	FeatureQoSHint                       Features = 1 << 42
	FeatureReallocationOfCredit          Features = 1 << 43
	FeatureNetLocTrustedN3GA             Features = 1 << 44
	FeatureNetLocWireline                Features = 1 << 45
	FeatureMPSForDTS                     Features = 1 << 46
	FeatureUserEquipmentInfoExtension    Features = 1 << 47
	FeatureAuthorizationForMPSSignalling Features = 1 << 48
	FeatureMPSForMessaging               Features = 1 << 49
	FeatureUESatUEComm                   Features = 1 << 50
	FeaturePCEFFailureDetection          Features = 1 << 51
	FeatureCNHealthMonitor               Features = 1 << 52
)

const (
	featureList1 uint32 = 1
	featureList2 uint32 = 2
)

var featureNames = map[uint32][]string{
	featureList1: {
		"Rel8", "Rel9", "ProvAFsignalFlow", "SponsoredConnectivity", "Rel10", "NetLoc", "ExtendedFilter", "SCTimeBasedUM",
		"Netloc-Trusted-WLAN", "RAN-NAS-Cause", "GroupComService", "ResShare", "DeferredService", "DSCP", "SponsorChange",
		"E2EQOSMTSI", "NetLoc-Untrusted-WLAN", "MCPTT", "PrioritySharing", "PLMNInfo", "MediaComponentVersioning",
		"MCPTT-Preemption", "MCVideo",
	},
	featureList2: {
		"PCSCF-Restoration-Enhancement", "Extended-Max-Requested-BW-NR", "Extended-Min-Requested-BW-NR",
		"Extended-BW-E2EQOSMTSI-NR", "VBC", "CHEM", "VBCLTE", "FLUS", "EPSFallbackReport", "ATSSS", "QoSHint",
		"ReallocationOfCredit", "Netloc-Trusted-N3GA", "NetLoc-Wireline", "MPSforDTS", "User-Equipment-Info-Extension",
		"AuthorizationForMpsSignalling", "MPSforMessaging", "UeSatUeComm", "PcefFailureDetection", "CnHealthMonitor",
	},
}

func (f Features) list(id uint32) uint32 {
	switch id {
	case featureList1:
		return uint32(f)
	case featureList2:
		return uint32(f >> 32)
	default:
		return 0
	}
}

func listFeatures(id, list uint32) Features {
	switch id {
	case featureList1:
		return Features(list)
	case featureList2:
		return Features(list) << 32
	default:
		return 0
	}
}

func (f Features) String() string {
	if f == 0 {
		return "0"
	}

	var parts []string

	for _, id := range []uint32{featureList1, featureList2} {
		list := f.list(id)

		for bit, name := range featureNames[id] {
			if list&(1<<bit) != 0 {
				parts = append(parts, name)
				list &^= 1 << bit
			}
		}

		if list != 0 {
			parts = append(parts, fmt.Sprintf("list%d:%#x", id, list))
		}
	}

	return strings.Join(parts, "|")
}

type IPCANType uint32

const (
	IPCAN3GPPGPRS   IPCANType = 0
	IPCANDOCSIS     IPCANType = 1
	IPCANxDSL       IPCANType = 2
	IPCANWiMAX      IPCANType = 3
	IPCAN3GPP2      IPCANType = 4
	IPCAN3GPPEPS    IPCANType = 5
	IPCANNon3GPPEPS IPCANType = 6
	IPCANFBA        IPCANType = 7
	IPCAN3GPP5GS    IPCANType = 8
	IPCANNon3GPP5GS IPCANType = 9
)

var ipcanTypeNames = tgpp.EnumNames{
	"3GPP-GPRS", "DOCSIS", "xDSL", "WiMAX", "3GPP2", "3GPP-EPS", "Non-3GPP-EPS", "FBA", "3GPP-5GS", "Non-3GPP-5GS",
}

func (t IPCANType) String() string { return ipcanTypeNames.Name("IPCANType", uint32(t)) }

func (t IPCANType) valid() bool { return ipcanTypeNames.Has(uint32(t)) }

type RATType uint32

const (
	RATWLAN                RATType = 0
	RATVirtual             RATType = 1
	RATTrustedN3GA         RATType = 2
	RATWireline            RATType = 3
	RATWirelineCable       RATType = 4
	RATWirelineBBF         RATType = 5
	RATUTRAN               RATType = 1000
	RATGERAN               RATType = 1001
	RATGAN                 RATType = 1002
	RATHSPAEvolution       RATType = 1003
	RATEUTRAN              RATType = 1004
	RATEUTRANNBIoT         RATType = 1005
	RATNR                  RATType = 1006
	RATLTEM                RATType = 1007
	RATNRU                 RATType = 1008
	RATEUTRANLEO           RATType = 1011
	RATEUTRANMEO           RATType = 1012
	RATEUTRANGEO           RATType = 1013
	RATEUTRANOtherSat      RATType = 1014
	RATEUTRANNBIoTLEO      RATType = 1021
	RATEUTRANNBIoTMEO      RATType = 1022
	RATEUTRANNBIoTGEO      RATType = 1023
	RATEUTRANNBIoTOtherSat RATType = 1024
	RATLTEMLEO             RATType = 1031
	RATLTEMMEO             RATType = 1032
	RATLTEMGEO             RATType = 1033
	RATLTEMOtherSat        RATType = 1034
	RATNRLEO               RATType = 1035
	RATNRMEO               RATType = 1036
	RATNRGEO               RATType = 1037
	RATNROtherSat          RATType = 1038
	RATNRRedCap            RATType = 1039
	RATNREnhancedRedCap    RATType = 1040
	RATCDMA20001X          RATType = 2000
	RATHRPD                RATType = 2001
	RATUMB                 RATType = 2002
	RATEHRPD               RATType = 2003
)

var ratTypeNames = map[RATType]string{
	RATWLAN: "WLAN", RATVirtual: "VIRTUAL", RATTrustedN3GA: "TRUSTED-N3GA", RATWireline: "WIRELINE",
	RATWirelineCable: "WIRELINE-CABLE", RATWirelineBBF: "WIRELINE-BBF", RATUTRAN: "UTRAN", RATGERAN: "GERAN", RATGAN: "GAN",
	RATHSPAEvolution: "HSPA_EVOLUTION", RATEUTRAN: "EUTRAN", RATEUTRANNBIoT: "EUTRAN-NB-IoT", RATNR: "NR", RATLTEM: "LTE-M",
	RATNRU: "NR-U", RATEUTRANLEO: "EUTRAN(LEO)", RATEUTRANMEO: "EUTRAN(MEO)", RATEUTRANGEO: "EUTRAN(GEO)",
	RATEUTRANOtherSat: "EUTRAN(OTHERSAT)", RATEUTRANNBIoTLEO: "EUTRAN-NB-IoT(LEO)", RATEUTRANNBIoTMEO: "EUTRAN-NB-IoT(MEO)",
	RATEUTRANNBIoTGEO: "EUTRAN-NB-IoT(GEO)", RATEUTRANNBIoTOtherSat: "EUTRAN-NB-IoT(OTHERSAT)", RATLTEMLEO: "LTE-M(LEO)",
	RATLTEMMEO: "LTE-M(MEO)", RATLTEMGEO: "LTE-M(GEO)", RATLTEMOtherSat: "LTE-M(OTHERSAT)", RATNRLEO: "NR(LEO)",
	RATNRMEO: "NR(MEO)", RATNRGEO: "NR(GEO)", RATNROtherSat: "NR(OTHERSAT)", RATNRRedCap: "NR-REDCAP",
	RATNREnhancedRedCap: "NR-EREDCAP", RATCDMA20001X: "CDMA2000_1X", RATHRPD: "HRPD", RATUMB: "UMB", RATEHRPD: "EHRPD",
}

func (t RATType) String() string {
	if name, ok := ratTypeNames[t]; ok {
		return name
	}

	return fmt.Sprintf("RATType(%d)", uint32(t))
}

type ANTrusted uint32

const (
	ANTrustedTrusted   ANTrusted = 0
	ANTrustedUntrusted ANTrusted = 1
)

var anTrustedNames = tgpp.EnumNames{"TRUSTED", "UNTRUSTED"}

func (t ANTrusted) String() string { return anTrustedNames.Name("ANTrusted", uint32(t)) }

func (t ANTrusted) valid() bool { return anTrustedNames.Has(uint32(t)) }

type NetLocAccessSupport uint32

const NetLocAccessNotSupported NetLocAccessSupport = 0

var netLocAccessSupportNames = tgpp.EnumNames{"NETLOC_ACCESS_NOT_SUPPORTED"}

func (s NetLocAccessSupport) String() string {
	return netLocAccessSupportNames.Name("NetLocAccessSupport", uint32(s))
}

func (s NetLocAccessSupport) valid() bool { return netLocAccessSupportNames.Has(uint32(s)) }

type RequiredAccessInfo uint32

const (
	RequiredUserLocation RequiredAccessInfo = 0
	RequiredMSTimeZone   RequiredAccessInfo = 1
	RequiredUESatInfo    RequiredAccessInfo = 2
)

var requiredAccessInfoNames = tgpp.EnumNames{"USER_LOCATION", "MS_TIME_ZONE", "UE_SAT_INFO"}

func (r RequiredAccessInfo) String() string {
	return requiredAccessInfoNames.Name("RequiredAccessInfo", uint32(r))
}

func (r RequiredAccessInfo) valid() bool { return requiredAccessInfoNames.Has(uint32(r)) }

type PCSessionRecoveryStatus uint32

const (
	SessionRestorationRequest      PCSessionRecoveryStatus = 0
	SessionRestorationTriggered    PCSessionRecoveryStatus = 1
	SessionRestorationNotTriggered PCSessionRecoveryStatus = 2
	SessionNotFound                PCSessionRecoveryStatus = 3
)

var pcSessionRecoveryStatusNames = tgpp.EnumNames{
	"SESSION_RESTORATION_REQUEST", "SESSION_RESTORATION_TRIGGERED", "SESSION_RESTORATION_NOT_TRIGGERED", "SESSION_NOT_FOUND",
}

func (s PCSessionRecoveryStatus) String() string {
	return pcSessionRecoveryStatusNames.Name("PCSessionRecoveryStatus", uint32(s))
}

func (s PCSessionRecoveryStatus) valid() bool { return pcSessionRecoveryStatusNames.Has(uint32(s)) }

type FinalUnitAction uint32

const (
	FinalUnitTerminate      FinalUnitAction = 0
	FinalUnitRedirect       FinalUnitAction = 1
	FinalUnitRestrictAccess FinalUnitAction = 2
)

var finalUnitActionNames = tgpp.EnumNames{"TERMINATE", "REDIRECT", "RESTRICT_ACCESS"}

func (a FinalUnitAction) String() string {
	return finalUnitActionNames.Name("FinalUnitAction", uint32(a))
}

func (a FinalUnitAction) valid() bool { return finalUnitActionNames.Has(uint32(a)) }

type MediaComponentStatus uint32

const (
	MediaComponentActive   MediaComponentStatus = 0
	MediaComponentInactive MediaComponentStatus = 1
)

var mediaComponentStatusNames = tgpp.EnumNames{"ACTIVE", "INACTIVE"}

func (s MediaComponentStatus) String() string {
	return mediaComponentStatusNames.Name("MediaComponentStatus", uint32(s))
}

func (s MediaComponentStatus) valid() bool { return mediaComponentStatusNames.Has(uint32(s)) }
