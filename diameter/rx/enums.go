// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"fmt"
	"strings"
)

type AbortCause uint32

const (
	AbortBearerReleased                      AbortCause = 0
	AbortInsufficientServerResources         AbortCause = 1
	AbortInsufficientBearerResources         AbortCause = 2
	AbortPSToCSHandover                      AbortCause = 3
	AbortSponsoredDataConnectivityDisallowed AbortCause = 4
	AbortPCEFFailure                         AbortCause = 5
	maxAbortCause                                       = AbortPCEFFailure
)

func (c AbortCause) String() string {
	return enumName("AbortCause", uint32(c),
		"BEARER_RELEASED", "INSUFFICIENT_SERVER_RESOURCES", "INSUFFICIENT_BEARER_RESOURCES", "PS_TO_CS_HANDOVER",
		"SPONSORED_DATA_CONNECTIVITY_DISALLOWED", "PCEF_FAILURE")
}

type TerminationCause uint32

const (
	TerminationLogout             TerminationCause = 1
	TerminationServiceNotProvided TerminationCause = 2
	TerminationBadAnswer          TerminationCause = 3
	TerminationAdministrative     TerminationCause = 4
	TerminationLinkBroken         TerminationCause = 5
	TerminationAuthExpired        TerminationCause = 6
	TerminationUserMoved          TerminationCause = 7
	TerminationSessionTimeout     TerminationCause = 8
	maxTerminationCause                            = TerminationSessionTimeout
)

func (c TerminationCause) String() string {
	return enumName("TerminationCause", uint32(c),
		"", "DIAMETER_LOGOUT", "DIAMETER_SERVICE_NOT_PROVIDED", "DIAMETER_BAD_ANSWER", "DIAMETER_ADMINISTRATIVE",
		"DIAMETER_LINK_BROKEN", "DIAMETER_AUTH_EXPIRED", "DIAMETER_USER_MOVED", "DIAMETER_SESSION_TIMEOUT")
}

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
	maxSpecificAction                                               = ActionCNHealthMonitor
)

func (a SpecificAction) String() string {
	return enumName("SpecificAction", uint32(a),
		"", "CHARGING_CORRELATION_EXCHANGE", "INDICATION_OF_LOSS_OF_BEARER", "INDICATION_OF_RECOVERY_OF_BEARER",
		"INDICATION_OF_RELEASE_OF_BEARER", "", "IP-CAN_CHANGE", "INDICATION_OF_OUT_OF_CREDIT",
		"INDICATION_OF_SUCCESSFUL_RESOURCES_ALLOCATION", "INDICATION_OF_FAILED_RESOURCES_ALLOCATION",
		"INDICATION_OF_LIMITED_PCC_DEPLOYMENT", "USAGE_REPORT", "ACCESS_NETWORK_INFO_REPORT",
		"INDICATION_OF_RECOVERY_FROM_LIMITED_PCC_DEPLOYMENT", "INDICATION_OF_ACCESS_NETWORK_INFO_REPORTING_FAILURE",
		"INDICATION_OF_TRANSFER_POLICY_EXPIRED", "PLMN_CHANGE", "EPS_FALLBACK", "INDICATION_OF_REALLOCATION_OF_CREDIT",
		"SUCCESSFUL_QOS_UPDATE", "FAILED_QOS_UPDATE", "CN_HEALTH_MONITOR")
}

type RequestType uint32

const (
	RequestInitial          RequestType = 0
	RequestUpdate           RequestType = 1
	RequestPCSCFRestoration RequestType = 2
	maxRequestType                      = RequestPCSCFRestoration
)

func (t RequestType) String() string {
	return enumName("RequestType", uint32(t), "INITIAL_REQUEST", "UPDATE_REQUEST", "PCSCF_RESTORATION")
}

type ServiceInfoStatus uint32

const (
	ServiceInfoFinal       ServiceInfoStatus = 0
	ServiceInfoPreliminary ServiceInfoStatus = 1
	maxServiceInfoStatus                     = ServiceInfoPreliminary
)

func (s ServiceInfoStatus) String() string {
	return enumName("ServiceInfoStatus", uint32(s), "FINAL_SERVICE_INFORMATION", "PRELIMINARY_SERVICE_INFORMATION")
}

type SIPForkingIndication uint32

const (
	ForkingSingleDialogue   SIPForkingIndication = 0
	ForkingSeveralDialogues SIPForkingIndication = 1
	maxSIPForkingIndication                      = ForkingSeveralDialogues
)

func (f SIPForkingIndication) String() string {
	return enumName("SIPForkingIndication", uint32(f), "SINGLE_DIALOGUE", "SEVERAL_DIALOGUES")
}

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

func (t MediaType) String() string {
	if t == MediaOther {
		return "OTHER"
	}

	return enumName("MediaType", uint32(t), "AUDIO", "VIDEO", "DATA", "APPLICATION", "CONTROL", "TEXT", "MESSAGE")
}

func (t MediaType) valid() bool {
	return t <= MediaMessage || t == MediaOther
}

type FlowStatus uint32

const (
	FlowEnabledUplink   FlowStatus = 0
	FlowEnabledDownlink FlowStatus = 1
	FlowEnabled         FlowStatus = 2
	FlowDisabled        FlowStatus = 3
	FlowRemoved         FlowStatus = 4
	maxFlowStatus                  = FlowRemoved
)

func (s FlowStatus) String() string {
	return enumName("FlowStatus", uint32(s), "ENABLED-UPLINK", "ENABLED-DOWNLINK", "ENABLED", "DISABLED", "REMOVED")
}

type FlowUsage uint32

const (
	FlowUsageNoInformation FlowUsage = 0
	FlowUsageRTCP          FlowUsage = 1
	FlowUsageAFSignalling  FlowUsage = 2
	maxFlowUsage                     = FlowUsageAFSignalling
)

func (u FlowUsage) String() string {
	return enumName("FlowUsage", uint32(u), "NO_INFORMATION", "RTCP", "AF_SIGNALLING")
}

type AFSignallingProtocol uint32

const (
	SignallingNoInformation AFSignallingProtocol = 0
	SignallingSIP           AFSignallingProtocol = 1
	maxAFSignallingProtocol                      = SignallingSIP
)

func (p AFSignallingProtocol) String() string {
	return enumName("AFSignallingProtocol", uint32(p), "NO_INFORMATION", "SIP")
}

type SubscriptionIDType uint32

const (
	SubscriptionE164      SubscriptionIDType = 0
	SubscriptionIMSI      SubscriptionIDType = 1
	SubscriptionSIPURI    SubscriptionIDType = 2
	SubscriptionNAI       SubscriptionIDType = 3
	SubscriptionPrivate   SubscriptionIDType = 4
	maxSubscriptionIDType                    = SubscriptionPrivate
)

func (t SubscriptionIDType) String() string {
	return enumName("SubscriptionIDType", uint32(t),
		"END_USER_E164", "END_USER_IMSI", "END_USER_SIP_URI", "END_USER_NAI", "END_USER_PRIVATE")
}

type CodecDirection uint8

const (
	CodecUplink   CodecDirection = 0
	CodecDownlink CodecDirection = 1
)

func (d CodecDirection) String() string {
	return enumName("CodecDirection", uint32(d), "uplink", "downlink")
}

type CodecKind uint8

const (
	CodecOffer       CodecKind = 0
	CodecAnswer      CodecKind = 1
	CodecDescription CodecKind = 2
)

func (k CodecKind) String() string {
	return enumName("CodecKind", uint32(k), "offer", "answer", "description")
}

type Direction uint8

const (
	DirectionIn  Direction = 0
	DirectionOut Direction = 1
)

func (d Direction) String() string {
	return enumName("Direction", uint32(d), "in", "out")
}

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
	FeatureE2EQOSMTSI               Features = 1 << 15
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
	FeatureExtendedBWE2EQOSMTSINR        Features = 1 << 35
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
	FeatureUeSatUeComm                   Features = 1 << 50
	FeaturePCEFFailureDetection          Features = 1 << 51
	FeatureCNHealthMonitor               Features = 1 << 52
)

var featureNames = [2][]string{
	{
		"Rel8", "Rel9", "ProvAFsignalFlow", "SponsoredConnectivity", "Rel10", "NetLoc", "ExtendedFilter", "SCTimeBasedUM",
		"Netloc-Trusted-WLAN", "RAN-NAS-Cause", "GroupComService", "ResShare", "DeferredService", "DSCP", "SponsorChange",
		"E2EQOSMTSI", "NetLoc-Untrusted-WLAN", "MCPTT", "PrioritySharing", "PLMNInfo", "MediaComponentVersioning",
		"MCPTT-Preemption", "MCVideo",
	},
	{
		"PCSCF-Restoration-Enhancement", "Extended-Max-Requested-BW-NR", "Extended-Min-Requested-BW-NR",
		"Extended-BW-E2EQOSMTSI-NR", "VBC", "CHEM", "VBCLTE", "FLUS", "EPSFallbackReport", "ATSSS", "QoSHint",
		"ReallocationOfCredit", "Netloc-Trusted-N3GA", "NetLoc-Wireline", "MPSforDTS", "User-Equipment-Info-Extension",
		"AuthorizationForMpsSignalling", "MPSforMessaging", "UeSatUeComm", "PcefFailureDetection", "CnHealthMonitor",
	},
}

func (f Features) list(id uint32) uint32 {
	return uint32(f >> (32 * (id - 1)))
}

func listFeatures(id, list uint32) Features {
	return Features(list) << (32 * (id - 1))
}

func (f Features) String() string {
	if f == 0 {
		return "0"
	}

	var parts []string

	for i, names := range featureNames {
		list := f.list(uint32(i + 1))

		for bit, name := range names {
			if list&(1<<bit) != 0 {
				parts = append(parts, name)
				list &^= 1 << bit
			}
		}

		if list != 0 {
			parts = append(parts, fmt.Sprintf("list%d:%#x", i+1, list))
		}
	}

	return strings.Join(parts, "|")
}

func enumName(typeName string, v uint32, names ...string) string {
	if int(v) < len(names) && names[v] != "" {
		return names[v]
	}

	return fmt.Sprintf("%s(%d)", typeName, v)
}
