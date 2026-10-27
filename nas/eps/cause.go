// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "fmt"

// EMMCause is an EMM cause value (TS 24.301 §9.9.3.9, table 9.9.3.9.1). It is
// the EPS counterpart of the 5GS 5GMM cause.
type EMMCause uint8

// EMM cause values (TS 24.301 §9.9.3.9, table 9.9.3.9.1).
const (
	EMMCauseIMSIUnknownInHSS                           EMMCause = 2
	EMMCauseIllegalUE                                  EMMCause = 3
	EMMCauseIMEINotAccepted                            EMMCause = 5
	EMMCauseIllegalME                                  EMMCause = 6
	EMMCauseEPSServicesNotAllowed                      EMMCause = 7
	EMMCauseEPSAndNonEPSServicesNotAllowed             EMMCause = 8
	EMMCauseUEIdentityCannotBeDerived                  EMMCause = 9
	EMMCauseImplicitlyDetached                         EMMCause = 10
	EMMCausePLMNNotAllowed                             EMMCause = 11
	EMMCauseTrackingAreaNotAllowed                     EMMCause = 12
	EMMCauseRoamingNotAllowedInThisTA                  EMMCause = 13
	EMMCauseEPSServicesNotAllowedInThisPLMN            EMMCause = 14
	EMMCauseNoSuitableCellsInTrackingArea              EMMCause = 15
	EMMCauseMSCTemporarilyNotReachable                 EMMCause = 16
	EMMCauseNetworkFailure                             EMMCause = 17
	EMMCauseCSDomainNotAvailable                       EMMCause = 18
	EMMCauseESMFailure                                 EMMCause = 19
	EMMCauseMACFailure                                 EMMCause = 20
	EMMCauseSynchFailure                               EMMCause = 21
	EMMCauseCongestion                                 EMMCause = 22
	EMMCauseUESecurityCapabilitiesMismatch             EMMCause = 23
	EMMCauseSecurityModeRejectedUnspecified            EMMCause = 24
	EMMCauseNotAuthorizedForThisCSG                    EMMCause = 25
	EMMCauseNonEPSAuthenticationUnacceptable           EMMCause = 26
	EMMCauseRedirectionTo5GCNRequired                  EMMCause = 31
	EMMCauseServiceOptionNotAuthorizedInPLMN           EMMCause = 35
	EMMCauseCSServiceTemporarilyNotAvailable           EMMCause = 39
	EMMCauseNoEPSBearerContextActivated                EMMCause = 40
	EMMCauseSevereNetworkFailure                       EMMCause = 42
	EMMCauseSemanticallyIncorrectMessage               EMMCause = 95
	EMMCauseInvalidMandatoryInformation                EMMCause = 96
	EMMCauseMessageTypeNonExistent                     EMMCause = 97
	EMMCauseMessageTypeNotCompatible                   EMMCause = 98
	EMMCauseIENonExistent                              EMMCause = 99
	EMMCauseConditionalIEError                         EMMCause = 100
	EMMCauseMessageNotCompatible                       EMMCause = 101
	EMMCauseProtocolErrorUnspecified                   EMMCause = 111
	EMMCauseIABNodeOperationNotAuthorized              EMMCause = 36
	EMMCausePLMNNotAllowedToOperateAtPresentUELocation EMMCause = 78
)

// #nosec G101 -- gosec matches "bearer" as an HTTP Bearer token; these name the 3GPP EPS bearer.
var emmCauseNames = map[EMMCause]string{
	EMMCauseIMSIUnknownInHSS:                           "IMSI unknown in HSS",
	EMMCauseIllegalUE:                                  "Illegal UE",
	EMMCauseIMEINotAccepted:                            "IMEI not accepted",
	EMMCauseIllegalME:                                  "Illegal ME",
	EMMCauseEPSServicesNotAllowed:                      "EPS services not allowed",
	EMMCauseEPSAndNonEPSServicesNotAllowed:             "EPS services and non-EPS services not allowed",
	EMMCauseUEIdentityCannotBeDerived:                  "UE identity cannot be derived by the network",
	EMMCauseImplicitlyDetached:                         "Implicitly detached",
	EMMCausePLMNNotAllowed:                             "PLMN not allowed",
	EMMCauseTrackingAreaNotAllowed:                     "Tracking area not allowed",
	EMMCauseRoamingNotAllowedInThisTA:                  "Roaming not allowed in this tracking area",
	EMMCauseEPSServicesNotAllowedInThisPLMN:            "EPS services not allowed in this PLMN",
	EMMCauseNoSuitableCellsInTrackingArea:              "No suitable cells in tracking area",
	EMMCauseMSCTemporarilyNotReachable:                 "MSC temporarily not reachable",
	EMMCauseNetworkFailure:                             "Network failure",
	EMMCauseCSDomainNotAvailable:                       "CS domain not available",
	EMMCauseESMFailure:                                 "ESM failure",
	EMMCauseMACFailure:                                 "MAC failure",
	EMMCauseSynchFailure:                               "Synch failure",
	EMMCauseCongestion:                                 "Congestion",
	EMMCauseUESecurityCapabilitiesMismatch:             "UE security capabilities mismatch",
	EMMCauseSecurityModeRejectedUnspecified:            "Security mode rejected, unspecified",
	EMMCauseNotAuthorizedForThisCSG:                    "Not authorized for this CSG",
	EMMCauseNonEPSAuthenticationUnacceptable:           "Non-EPS authentication unacceptable",
	EMMCauseRedirectionTo5GCNRequired:                  "Redirection to 5GCN required",
	EMMCauseServiceOptionNotAuthorizedInPLMN:           "Requested service option not authorized in this PLMN",
	EMMCauseCSServiceTemporarilyNotAvailable:           "CS service temporarily not available",
	EMMCauseNoEPSBearerContextActivated:                "No EPS bearer context activated",
	EMMCauseSevereNetworkFailure:                       "Severe network failure",
	EMMCauseSemanticallyIncorrectMessage:               "Semantically incorrect message",
	EMMCauseInvalidMandatoryInformation:                "Invalid mandatory information",
	EMMCauseMessageTypeNonExistent:                     "Message type non-existent or not implemented",
	EMMCauseMessageTypeNotCompatible:                   "Message type not compatible with the protocol state",
	EMMCauseIENonExistent:                              "Information element non-existent or not implemented",
	EMMCauseConditionalIEError:                         "Conditional IE error",
	EMMCauseMessageNotCompatible:                       "Message not compatible with the protocol state",
	EMMCauseProtocolErrorUnspecified:                   "Protocol error, unspecified",
	EMMCauseIABNodeOperationNotAuthorized:              "IAB-node operation not authorized",
	EMMCausePLMNNotAllowedToOperateAtPresentUELocation: "PLMN not allowed to operate at the present UE location",
}

// Name returns the cause's spec description, or the empty string when the value
// is not one TS 24.301 assigns.
func (c EMMCause) Name() string { return emmCauseNames[c] }

func (c EMMCause) String() string { return causeString("EMM", uint8(c), emmCauseNames[c]) }

// ESMCause is an ESM cause value (TS 24.301 §9.9.4.4, table 9.9.4.4.1). It is the
// EPS counterpart of the 5GS 5GSM cause.
type ESMCause uint8

// ESM cause values (TS 24.301 §9.9.4.4, table 9.9.4.4.1).
const (
	ESMCauseOperatorDeterminedBarring                   ESMCause = 8
	ESMCauseInsufficientResources                       ESMCause = 26
	ESMCauseMissingOrUnknownAPN                         ESMCause = 27
	ESMCauseUnknownPDNType                              ESMCause = 28
	ESMCauseUserAuthenticationOrAuthorizationFailed     ESMCause = 29
	ESMCauseRequestRejectedByServingGW                  ESMCause = 30
	ESMCauseRequestRejectedUnspecified                  ESMCause = 31
	ESMCauseServiceOptionNotSupported                   ESMCause = 32
	ESMCauseServiceOptionNotSubscribed                  ESMCause = 33
	ESMCauseServiceOptionTemporarilyOut                 ESMCause = 34
	ESMCausePTIAlreadyInUse                             ESMCause = 35
	ESMCauseRegularDeactivation                         ESMCause = 36
	ESMCauseEPSQoSNotAccepted                           ESMCause = 37
	ESMCauseNetworkFailure                              ESMCause = 38
	ESMCauseReactivationRequested                       ESMCause = 39
	ESMCauseSemanticErrorInTFT                          ESMCause = 41
	ESMCauseSyntacticalErrorInTFT                       ESMCause = 42
	ESMCauseInvalidEPSBearerIdentity                    ESMCause = 43
	ESMCauseSemanticErrorInPacketFilter                 ESMCause = 44
	ESMCauseSyntacticalErrorInFilter                    ESMCause = 45
	ESMCausePTIMismatch                                 ESMCause = 47
	ESMCauseLastPDNDisconnectionNotAllow                ESMCause = 49
	ESMCausePDNTypeIPv4OnlyAllowed                      ESMCause = 50
	ESMCausePDNTypeIPv6OnlyAllowed                      ESMCause = 51
	ESMCauseSingleAddressBearersOnly                    ESMCause = 52
	ESMCauseESMInformationNotReceived                   ESMCause = 53
	ESMCausePDNConnectionDoesNotExist                   ESMCause = 54
	ESMCauseMultiplePDNNotAllowed                       ESMCause = 55
	ESMCauseMaxEPSBearersReached                        ESMCause = 65
	ESMCauseInvalidPTIValue                             ESMCause = 81
	ESMCauseSemanticallyIncorrectMessage                ESMCause = 95
	ESMCauseInvalidMandatoryInfo                        ESMCause = 96
	ESMCauseMessageTypeNonExistent                      ESMCause = 97
	ESMCauseMessageTypeNotCompatible                    ESMCause = 98
	ESMCauseIENonExistent                               ESMCause = 99
	ESMCauseConditionalIEError                          ESMCause = 100
	ESMCauseMessageNotCompatible                        ESMCause = 101
	ESMCauseProtocolErrorUnspecified                    ESMCause = 111
	ESMCauseCollisionWithNetworkInitiatedRequest        ESMCause = 56
	ESMCausePDNTypeIPv4v6OnlyAllowed                    ESMCause = 57
	ESMCausePDNTypeNonIPOnlyAllowed                     ESMCause = 58
	ESMCauseUnsupportedQCIValue                         ESMCause = 59
	ESMCauseBearerHandlingNotSupported                  ESMCause = 60
	ESMCausePDNTypeEthernetOnlyAllowed                  ESMCause = 61
	ESMCauseRequestedAPNNotSupportedInCurrentRATAndPLMN ESMCause = 66
	ESMCauseAPNRestrictionValueIncompatible             ESMCause = 112
	ESMCauseMultipleAccessesToAPDNConnectionNotAllowed  ESMCause = 113
)

// #nosec G101 -- gosec matches "bearer" as an HTTP Bearer token; these name the 3GPP EPS bearer.
var esmCauseNames = map[ESMCause]string{
	ESMCauseOperatorDeterminedBarring:                   "Operator determined barring",
	ESMCauseInsufficientResources:                       "Insufficient resources",
	ESMCauseMissingOrUnknownAPN:                         "Missing or unknown APN",
	ESMCauseUnknownPDNType:                              "Unknown PDN type",
	ESMCauseUserAuthenticationOrAuthorizationFailed:     "User authentication or authorization failed",
	ESMCauseRequestRejectedByServingGW:                  "Request rejected by Serving GW or PDN GW",
	ESMCauseRequestRejectedUnspecified:                  "Request rejected, unspecified",
	ESMCauseServiceOptionNotSupported:                   "Service option not supported",
	ESMCauseServiceOptionNotSubscribed:                  "Requested service option not subscribed",
	ESMCauseServiceOptionTemporarilyOut:                 "Service option temporarily out of order",
	ESMCausePTIAlreadyInUse:                             "PTI already in use",
	ESMCauseRegularDeactivation:                         "Regular deactivation",
	ESMCauseEPSQoSNotAccepted:                           "EPS QoS not accepted",
	ESMCauseNetworkFailure:                              "Network failure",
	ESMCauseReactivationRequested:                       "Reactivation requested",
	ESMCauseSemanticErrorInTFT:                          "Semantic error in the TFT operation",
	ESMCauseSyntacticalErrorInTFT:                       "Syntactical error in the TFT operation",
	ESMCauseInvalidEPSBearerIdentity:                    "Invalid EPS bearer identity",
	ESMCauseSemanticErrorInPacketFilter:                 "Semantic errors in packet filter(s)",
	ESMCauseSyntacticalErrorInFilter:                    "Syntactical errors in packet filter(s)",
	ESMCausePTIMismatch:                                 "PTI mismatch",
	ESMCauseLastPDNDisconnectionNotAllow:                "Last PDN disconnection not allowed",
	ESMCausePDNTypeIPv4OnlyAllowed:                      "PDN type IPv4 only allowed",
	ESMCausePDNTypeIPv6OnlyAllowed:                      "PDN type IPv6 only allowed",
	ESMCauseSingleAddressBearersOnly:                    "Single address bearers only allowed",
	ESMCauseESMInformationNotReceived:                   "ESM information not received",
	ESMCausePDNConnectionDoesNotExist:                   "PDN connection does not exist",
	ESMCauseMultiplePDNNotAllowed:                       "Multiple PDN connections for a given APN not allowed",
	ESMCauseMaxEPSBearersReached:                        "Maximum number of EPS bearers reached",
	ESMCauseInvalidPTIValue:                             "Invalid PTI value",
	ESMCauseSemanticallyIncorrectMessage:                "Semantically incorrect message",
	ESMCauseInvalidMandatoryInfo:                        "Invalid mandatory information",
	ESMCauseMessageTypeNonExistent:                      "Message type non-existent or not implemented",
	ESMCauseMessageTypeNotCompatible:                    "Message type not compatible with the protocol state",
	ESMCauseIENonExistent:                               "Information element non-existent or not implemented",
	ESMCauseConditionalIEError:                          "Conditional IE error",
	ESMCauseMessageNotCompatible:                        "Message not compatible with the protocol state",
	ESMCauseProtocolErrorUnspecified:                    "Protocol error, unspecified",
	ESMCauseCollisionWithNetworkInitiatedRequest:        "Collision with network initiated request",
	ESMCausePDNTypeIPv4v6OnlyAllowed:                    "PDN type IPv4v6 only allowed",
	ESMCausePDNTypeNonIPOnlyAllowed:                     "PDN type non IP only allowed",
	ESMCauseUnsupportedQCIValue:                         "Unsupported QCI value",
	ESMCauseBearerHandlingNotSupported:                  "Bearer handling not supported",
	ESMCausePDNTypeEthernetOnlyAllowed:                  "PDN type Ethernet only allowed",
	ESMCauseRequestedAPNNotSupportedInCurrentRATAndPLMN: "Requested APN not supported in current RAT and PLMN combination",
	ESMCauseAPNRestrictionValueIncompatible:             "APN restriction value incompatible with active EPS bearer context",
	ESMCauseMultipleAccessesToAPDNConnectionNotAllowed:  "Multiple accesses to a PDN connection not allowed",
}

// Name returns the cause's spec description, or the empty string when the value
// is not one TS 24.301 assigns.
func (c ESMCause) Name() string { return esmCauseNames[c] }

func (c ESMCause) String() string { return causeString("ESM", uint8(c), esmCauseNames[c]) }

func causeString(kind string, value uint8, name string) string {
	if name == "" {
		return fmt.Sprintf("unknown %s cause (%d)", kind, value)
	}

	return fmt.Sprintf("%s (%d)", name, value)
}
