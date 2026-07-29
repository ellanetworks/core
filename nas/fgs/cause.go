// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "fmt"

// GMMCause is a 5GMM cause value (TS 24.501 §9.11.3.2, table 9.11.3.2.1). It is
// the 5GS counterpart of the EPS EMM cause.
type GMMCause uint8

// 5GMM cause values (TS 24.501 §9.11.3.2, table 9.11.3.2.1).
const (
	GMMCauseIllegalUE                                  GMMCause = 3
	GMMCausePEINotAccepted                             GMMCause = 5
	GMMCauseIllegalME                                  GMMCause = 6
	GMMCauseServicesNotAllowed                         GMMCause = 7
	GMMCauseUEIdentityCannotBeDerived                  GMMCause = 9
	GMMCauseImplicitlyDeregistered                     GMMCause = 10
	GMMCausePLMNNotAllowed                             GMMCause = 11
	GMMCauseTrackingAreaNotAllowed                     GMMCause = 12
	GMMCauseRoamingNotAllowedInThisTA                  GMMCause = 13
	GMMCauseNoSuitableCellsInTrackingArea              GMMCause = 15
	GMMCauseMACFailure                                 GMMCause = 20
	GMMCauseSynchFailure                               GMMCause = 21
	GMMCauseCongestion                                 GMMCause = 22
	GMMCauseUESecurityCapabilitiesMismatch             GMMCause = 23
	GMMCauseSecurityModeRejectedUnspecified            GMMCause = 24
	GMMCauseNon5GAuthenticationUnacceptable            GMMCause = 26
	GMMCauseN1ModeNotAllowed                           GMMCause = 27
	GMMCauseRestrictedServiceArea                      GMMCause = 28
	GMMCauseLADNNotAvailable                           GMMCause = 43
	GMMCauseMaxPDUSessionsReached                      GMMCause = 65
	GMMCauseInsufficientResourcesForSliceAndDNN        GMMCause = 67
	GMMCauseInsufficientResourcesForSlice              GMMCause = 69
	GMMCauseNgKSIAlreadyInUse                          GMMCause = 71
	GMMCauseNon3GPPAccessNotAllowed                    GMMCause = 72
	GMMCauseServingNetworkNotAuthorized                GMMCause = 73
	GMMCausePayloadNotForwarded                        GMMCause = 90
	GMMCauseDNNNotSupportedInSlice                     GMMCause = 91
	GMMCauseInsufficientUserPlaneResources             GMMCause = 92
	GMMCauseSemanticallyIncorrectMessage               GMMCause = 95
	GMMCauseInvalidMandatoryInformation                GMMCause = 96
	GMMCauseMessageTypeNonExistent                     GMMCause = 97
	GMMCauseMessageTypeNotCompatible                   GMMCause = 98
	GMMCauseIENonExistent                              GMMCause = 99
	GMMCauseConditionalIEError                         GMMCause = 100
	GMMCauseMessageNotCompatible                       GMMCause = 101
	GMMCauseProtocolErrorUnspecified                   GMMCause = 111
	GMMCauseRedirectionToEPCRequired                   GMMCause = 31
	GMMCauseIABNodeOperationNotAuthorized              GMMCause = 36
	GMMCauseNoNetworkSlicesAvailable                   GMMCause = 62
	GMMCauseTemporarilyNotAuthorizedForThisSNPN        GMMCause = 74
	GMMCausePermanentlyNotAuthorizedForThisSNPN        GMMCause = 75
	GMMCauseNotAuthorizedForThisCAG                    GMMCause = 76
	GMMCauseWirelineAccessAreaNotAllowed               GMMCause = 77
	GMMCausePLMNNotAllowedToOperateAtPresentUELocation GMMCause = 78
	GMMCauseUASServicesNotAllowed                      GMMCause = 79
	GMMCauseDisasterRoamingNotAllowed                  GMMCause = 80
	GMMCauseSelectedN3IWFNotCompatible                 GMMCause = 81
	GMMCauseSelectedTNGFNotCompatible                  GMMCause = 82
	GMMCauseOnboardingServicesTerminated               GMMCause = 93
	GMMCauseUserPlanePositioningNotAuthorized          GMMCause = 94
)

var cause5GMMNames = map[GMMCause]string{
	GMMCauseIllegalUE:                                  "Illegal UE",
	GMMCausePEINotAccepted:                             "PEI not accepted",
	GMMCauseIllegalME:                                  "Illegal ME",
	GMMCauseServicesNotAllowed:                         "5GS services not allowed",
	GMMCauseUEIdentityCannotBeDerived:                  "UE identity cannot be derived by the network",
	GMMCauseImplicitlyDeregistered:                     "Implicitly de-registered",
	GMMCausePLMNNotAllowed:                             "PLMN not allowed",
	GMMCauseTrackingAreaNotAllowed:                     "Tracking area not allowed",
	GMMCauseRoamingNotAllowedInThisTA:                  "Roaming not allowed in this tracking area",
	GMMCauseNoSuitableCellsInTrackingArea:              "No suitable cells in tracking area",
	GMMCauseMACFailure:                                 "MAC failure",
	GMMCauseSynchFailure:                               "Synch failure",
	GMMCauseCongestion:                                 "Congestion",
	GMMCauseUESecurityCapabilitiesMismatch:             "UE security capabilities mismatch",
	GMMCauseSecurityModeRejectedUnspecified:            "Security mode rejected, unspecified",
	GMMCauseNon5GAuthenticationUnacceptable:            "Non-5G authentication unacceptable",
	GMMCauseN1ModeNotAllowed:                           "N1 mode not allowed",
	GMMCauseRestrictedServiceArea:                      "Restricted service area",
	GMMCauseLADNNotAvailable:                           "LADN not available",
	GMMCauseMaxPDUSessionsReached:                      "Maximum number of PDU sessions reached",
	GMMCauseInsufficientResourcesForSliceAndDNN:        "Insufficient resources for specific slice and DNN",
	GMMCauseInsufficientResourcesForSlice:              "Insufficient resources for specific slice",
	GMMCauseNgKSIAlreadyInUse:                          "ngKSI already in use",
	GMMCauseNon3GPPAccessNotAllowed:                    "Non-3GPP access to 5GCN not allowed",
	GMMCauseServingNetworkNotAuthorized:                "Serving network not authorized",
	GMMCausePayloadNotForwarded:                        "Payload was not forwarded",
	GMMCauseDNNNotSupportedInSlice:                     "DNN not supported or not subscribed in the slice",
	GMMCauseInsufficientUserPlaneResources:             "Insufficient user-plane resources for the PDU session",
	GMMCauseSemanticallyIncorrectMessage:               "Semantically incorrect message",
	GMMCauseInvalidMandatoryInformation:                "Invalid mandatory information",
	GMMCauseMessageTypeNonExistent:                     "Message type non-existent or not implemented",
	GMMCauseMessageTypeNotCompatible:                   "Message type not compatible with the protocol state",
	GMMCauseIENonExistent:                              "Information element non-existent or not implemented",
	GMMCauseConditionalIEError:                         "Conditional IE error",
	GMMCauseMessageNotCompatible:                       "Message not compatible with the protocol state",
	GMMCauseProtocolErrorUnspecified:                   "Protocol error, unspecified",
	GMMCauseRedirectionToEPCRequired:                   "Redirection to EPC required",
	GMMCauseIABNodeOperationNotAuthorized:              "IAB-node operation not authorized",
	GMMCauseNoNetworkSlicesAvailable:                   "No network slices available",
	GMMCauseTemporarilyNotAuthorizedForThisSNPN:        "Temporarily not authorized for this SNPN",
	GMMCausePermanentlyNotAuthorizedForThisSNPN:        "Permanently not authorized for this SNPN",
	GMMCauseNotAuthorizedForThisCAG:                    "Not authorized for this CAG or authorized for CAG cells only",
	GMMCauseWirelineAccessAreaNotAllowed:               "Wireline access area not allowed",
	GMMCausePLMNNotAllowedToOperateAtPresentUELocation: "PLMN not allowed to operate at the present UE location",
	GMMCauseUASServicesNotAllowed:                      "UAS services not allowed",
	GMMCauseDisasterRoamingNotAllowed:                  "Disaster roaming for the determined PLMN with disaster condition not allowed",
	GMMCauseSelectedN3IWFNotCompatible:                 "Selected N3IWF is not compatible with the allowed NSSAI",
	GMMCauseSelectedTNGFNotCompatible:                  "Selected TNGF is not compatible with the allowed NSSAI",
	GMMCauseOnboardingServicesTerminated:               "Onboarding services terminated",
	GMMCauseUserPlanePositioningNotAuthorized:          "User plane positioning not authorized",
}

// Name returns the cause's spec description, or the empty string when the value
// is not one TS 24.501 assigns.
func (c GMMCause) Name() string { return cause5GMMNames[c] }

func (c GMMCause) String() string { return causeString("5GMM", uint8(c), cause5GMMNames[c]) }

// GSMCause is a 5GSM cause value (TS 24.501 §9.11.4.2, table 9.11.4.2.1). It is
// the 5GS counterpart of the EPS ESM cause.
type GSMCause uint8

// 5GSM cause values (TS 24.501 §9.11.4.2, table 9.11.4.2.1).
const (
	GSMCauseInsufficientResources                           GSMCause = 0x1A
	GSMCauseMissingOrUnknownDNN                             GSMCause = 0x1B
	GSMCauseUnknownPDUSessionType                           GSMCause = 0x1C
	GSMCauseRequestRejectedUnspecified                      GSMCause = 0x1F
	GSMCauseRegularDeactivation                             GSMCause = 0x24
	GSMCauseReactivationRequested                           GSMCause = 0x27
	GSMCauseInvalidPDUSessionIdentity                       GSMCause = 0x2B
	GSMCausePTIMismatch                                     GSMCause = 0x2F
	GSMCausePDUSessionTypeIPv4OnlyAllowed                   GSMCause = 0x32
	GSMCausePDUSessionTypeIPv6OnlyAllowed                   GSMCause = 0x33
	GSMCauseMissingOrUnknownDNNInASlice                     GSMCause = 0x46
	GSMCauseInvalidPTIValue                                 GSMCause = 0x51
	GSMCauseMessageTypeNonExistentOrNotImplemented          GSMCause = 0x61
	GSMCauseMessageTypeNotCompatibleWithTheProtocolState    GSMCause = 0x62
	GSMCauseProtocolErrorUnspecified                        GSMCause = 0x6F
	GSMCauseOperatorDeterminedBarring                       GSMCause = 8
	GSMCauseUserAuthenticationOrAuthorizationFailed         GSMCause = 29
	GSMCauseServiceOptionNotSupported                       GSMCause = 32
	GSMCauseRequestedServiceOptionNotSubscribed             GSMCause = 33
	GSMCausePTIAlreadyInUse                                 GSMCause = 35
	GSMCauseFiveGSQoSNotAccepted                            GSMCause = 37
	GSMCauseNetworkFailure                                  GSMCause = 38
	GSMCauseSemanticErrorInTheTFTOperation                  GSMCause = 41
	GSMCauseSyntacticalErrorInTheTFTOperation               GSMCause = 42
	GSMCauseSemanticErrorsInPacketFilters                   GSMCause = 44
	GSMCauseSyntacticalErrorInPacketFilters                 GSMCause = 45
	GSMCauseOutOfLADNServiceArea                            GSMCause = 46
	GSMCausePDUSessionDoesNotExist                          GSMCause = 54
	GSMCausePDUSessionTypeIPv4v6OnlyAllowed                 GSMCause = 57
	GSMCausePDUSessionTypeUnstructuredOnlyAllowed           GSMCause = 58
	GSMCauseUnsupported5QIValue                             GSMCause = 59
	GSMCausePDUSessionTypeEthernetOnlyAllowed               GSMCause = 61
	GSMCauseInsufficientResourcesForSpecificSliceAndDNN     GSMCause = 67
	GSMCauseNotSupportedSSCMode                             GSMCause = 68
	GSMCauseInsufficientResourcesForSpecificSlice           GSMCause = 69
	GSMCauseMaximumDataRatePerUEForUserPlaneIntegrityTooLow GSMCause = 82
	GSMCauseSemanticErrorInTheQoSOperation                  GSMCause = 83
	GSMCauseSyntacticalErrorInTheQoSOperation               GSMCause = 84
	GSMCauseInvalidMappedEPSBearerIdentity                  GSMCause = 85
	GSMCauseUASServicesNotAllowed                           GSMCause = 86
	GSMCauseSemanticallyIncorrectMessage                    GSMCause = 95
	GSMCauseInvalidMandatoryInformation                     GSMCause = 96
	GSMCauseInformationElementNonExistentOrNotImplemented   GSMCause = 99
	GSMCauseConditionalIEError                              GSMCause = 100
	GSMCauseMessageNotCompatibleWithTheProtocolState        GSMCause = 101
)

var cause5GSMNames = map[GSMCause]string{
	GSMCauseInsufficientResources:                           "Insufficient resources",
	GSMCauseMissingOrUnknownDNN:                             "Missing or unknown DNN",
	GSMCauseUnknownPDUSessionType:                           "Unknown PDU session type",
	GSMCauseRequestRejectedUnspecified:                      "Request rejected, unspecified",
	GSMCauseRegularDeactivation:                             "Regular deactivation",
	GSMCauseReactivationRequested:                           "Reactivation requested",
	GSMCauseInvalidPDUSessionIdentity:                       "Invalid PDU session identity",
	GSMCausePTIMismatch:                                     "PTI mismatch",
	GSMCausePDUSessionTypeIPv4OnlyAllowed:                   "PDU session type IPv4 only allowed",
	GSMCausePDUSessionTypeIPv6OnlyAllowed:                   "PDU session type IPv6 only allowed",
	GSMCauseMissingOrUnknownDNNInASlice:                     "Missing or unknown DNN in a slice",
	GSMCauseInvalidPTIValue:                                 "Invalid PTI value",
	GSMCauseMessageTypeNonExistentOrNotImplemented:          "Message type non-existent or not implemented",
	GSMCauseMessageTypeNotCompatibleWithTheProtocolState:    "Message type not compatible with the protocol state",
	GSMCauseProtocolErrorUnspecified:                        "Protocol error, unspecified",
	GSMCauseOperatorDeterminedBarring:                       "Operator determined barring",
	GSMCauseUserAuthenticationOrAuthorizationFailed:         "User authentication or authorization failed",
	GSMCauseServiceOptionNotSupported:                       "Service option not supported",
	GSMCauseRequestedServiceOptionNotSubscribed:             "Requested service option not subscribed",
	GSMCausePTIAlreadyInUse:                                 "PTI already in use",
	GSMCauseFiveGSQoSNotAccepted:                            "5GS QoS not accepted",
	GSMCauseNetworkFailure:                                  "Network failure",
	GSMCauseSemanticErrorInTheTFTOperation:                  "Semantic error in the TFT operation",
	GSMCauseSyntacticalErrorInTheTFTOperation:               "Syntactical error in the TFT operation",
	GSMCauseSemanticErrorsInPacketFilters:                   "Semantic errors in packet filter(s)",
	GSMCauseSyntacticalErrorInPacketFilters:                 "Syntactical error in packet filter(s)",
	GSMCauseOutOfLADNServiceArea:                            "Out of LADN service area",
	GSMCausePDUSessionDoesNotExist:                          "PDU session does not exist",
	GSMCausePDUSessionTypeIPv4v6OnlyAllowed:                 "PDU session type IPv4v6 only allowed",
	GSMCausePDUSessionTypeUnstructuredOnlyAllowed:           "PDU session type Unstructured only allowed",
	GSMCauseUnsupported5QIValue:                             "Unsupported 5QI value",
	GSMCausePDUSessionTypeEthernetOnlyAllowed:               "PDU session type Ethernet only allowed",
	GSMCauseInsufficientResourcesForSpecificSliceAndDNN:     "Insufficient resources for specific slice and DNN",
	GSMCauseNotSupportedSSCMode:                             "Not supported SSC mode",
	GSMCauseInsufficientResourcesForSpecificSlice:           "Insufficient resources for specific slice",
	GSMCauseMaximumDataRatePerUEForUserPlaneIntegrityTooLow: "Maximum data rate per UE for user-plane integrity protection is too low",
	GSMCauseSemanticErrorInTheQoSOperation:                  "Semantic error in the QoS operation",
	GSMCauseSyntacticalErrorInTheQoSOperation:               "Syntactical error in the QoS operation",
	GSMCauseInvalidMappedEPSBearerIdentity:                  "Invalid mapped EPS bearer identity",
	GSMCauseUASServicesNotAllowed:                           "UAS services not allowed",
	GSMCauseSemanticallyIncorrectMessage:                    "Semantically incorrect message",
	GSMCauseInvalidMandatoryInformation:                     "Invalid mandatory information",
	GSMCauseInformationElementNonExistentOrNotImplemented:   "Information element non-existent or not implemented",
	GSMCauseConditionalIEError:                              "Conditional IE error",
	GSMCauseMessageNotCompatibleWithTheProtocolState:        "Message not compatible with the protocol state",
}

// Name returns the cause's spec description, or the empty string when the value
// is not one TS 24.501 assigns.
func (c GSMCause) Name() string { return cause5GSMNames[c] }

func (c GSMCause) String() string { return causeString("5GSM", uint8(c), cause5GSMNames[c]) }

func causeString(kind string, value uint8, name string) string {
	if name == "" {
		return fmt.Sprintf("unknown %s cause (%d)", kind, value)
	}

	return fmt.Sprintf("%s (%d)", name, value)
}
