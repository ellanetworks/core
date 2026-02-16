package ue

import (
	"fmt"

	"github.com/free5gc/nas/nasMessage"
)

func cause5GMMToString(cause uint8) string {
	switch cause {
	case nasMessage.Cause5GMMIllegalUE:
		return "Illegal UE"
	case nasMessage.Cause5GMMPEINotAccepted:
		return "PEI Not Accepted"
	case nasMessage.Cause5GMMIllegalME:
		return "Illegal ME"
	case nasMessage.Cause5GMM5GSServicesNotAllowed:
		return "5GS Services Not Allowed"
	case nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork:
		return "UE Identity Cannot Be Derived By The Network"
	case nasMessage.Cause5GMMImplicitlyDeregistered:
		return "Implicitly Deregistered"
	case nasMessage.Cause5GMMPLMNNotAllowed:
		return "PLMN Not Allowed"
	case nasMessage.Cause5GMMTrackingAreaNotAllowed:
		return "Tracking Area Not Allowed"
	case nasMessage.Cause5GMMRoamingNotAllowedInThisTrackingArea:
		return "Roaming Not Allowed In This Tracking Area"
	case nasMessage.Cause5GMMNoSuitableCellsInTrackingArea:
		return "No Suitable Cells In Tracking Area"
	case nasMessage.Cause5GMMMACFailure:
		return "MAC Failure"
	case nasMessage.Cause5GMMSynchFailure:
		return "Synch Failure"
	case nasMessage.Cause5GMMCongestion:
		return "Congestion"
	case nasMessage.Cause5GMMUESecurityCapabilitiesMismatch:
		return "UE Security Capabilities Mismatch"
	case nasMessage.Cause5GMMSecurityModeRejectedUnspecified:
		return "Security Mode Rejected Unspecified"
	case nasMessage.Cause5GMMNon5GAuthenticationUnacceptable:
		return "Non-5G Authentication Unacceptable"
	case nasMessage.Cause5GMMN1ModeNotAllowed:
		return "N1 Mode Not Allowed"
	case nasMessage.Cause5GMMRestrictedServiceArea:
		return "Restricted Service Area"
	case nasMessage.Cause5GMMLADNNotAvailable:
		return "LADN Not Available"
	case nasMessage.Cause5GMMMaximumNumberOfPDUSessionsReached:
		return "Maximum Number Of PDU Sessions Reached"
	case nasMessage.Cause5GMMInsufficientResourcesForSpecificSliceAndDNN:
		return "Insufficient Resources For Specific Slice And DNN"
	case nasMessage.Cause5GMMInsufficientResourcesForSpecificSlice:
		return "Insufficient Resources For Specific Slice"
	case nasMessage.Cause5GMMngKSIAlreadyInUse:
		return "ngKSI Already In Use"
	case nasMessage.Cause5GMMNon3GPPAccessTo5GCNNotAllowed:
		return "Non-3GPP Access To 5GCN Not Allowed"
	case nasMessage.Cause5GMMServingNetworkNotAuthorized:
		return "Serving Network Not Authorized"
	case nasMessage.Cause5GMMPayloadWasNotForwarded:
		return "Payload Was Not Forwarded"
	case nasMessage.Cause5GMMDNNNotSupportedOrNotSubscribedInTheSlice:
		return "DNN Not Supported Or Not Subscribed In The Slice"
	case nasMessage.Cause5GMMInsufficientUserPlaneResourcesForThePDUSession:
		return "Insufficient User Plane Resources For The PDU Session"
	case nasMessage.Cause5GMMSemanticallyIncorrectMessage:
		return "Semantically Incorrect Message"
	case nasMessage.Cause5GMMInvalidMandatoryInformation:
		return "Invalid Mandatory Information"
	case nasMessage.Cause5GMMMessageTypeNonExistentOrNotImplemented:
		return "Message Type Non Existent Or Not Implemented"
	case nasMessage.Cause5GMMMessageTypeNotCompatibleWithTheProtocolState:
		return "Message Type Not Compatible With The Protocol State"
	case nasMessage.Cause5GMMInformationElementNonExistentOrNotImplemented:
		return "Information Element Non Existent Or Not Implemented"
	case nasMessage.Cause5GMMConditionalIEError:
		return "Conditional IE Error"
	case nasMessage.Cause5GMMMessageNotCompatibleWithTheProtocolState:
		return "Message Not Compatible With The Protocol State"
	case nasMessage.Cause5GMMProtocolErrorUnspecified:
		return "Protocol Error Unspecified"
	default:
		return fmt.Sprintf("Unknown Cause (%d)", cause)
	}
}

func cause5GSMToString(cause uint8) string {
	switch cause {
	case nasMessage.Cause5GSMInsufficientResources:
		return "Insufficient Resources"
	case nasMessage.Cause5GSMMissingOrUnknownDNN:
		return "Missing Or Unknown DNN"
	case nasMessage.Cause5GSMUnknownPDUSessionType:
		return "Unknown PDU Session Type"
	case nasMessage.Cause5GSMUserAuthenticationOrAuthorizationFailed:
		return "User Authentication Or Authorization Failed"
	case nasMessage.Cause5GSMRequestRejectedUnspecified:
		return "Request Rejected Unspecified"
	case nasMessage.Cause5GSMServiceOptionTemporarilyOutOfOrder:
		return "Service Option Temporarily Out Of Order"
	case nasMessage.Cause5GSMPTIAlreadyInUse:
		return "PTI Already In Use"
	case nasMessage.Cause5GSMRegularDeactivation:
		return "Regular Deactivation"
	case nasMessage.Cause5GSMNetworkFailure:
		return "Network Failure"
	case nasMessage.Cause5GSMReactivationRequested:
		return "Reactivation Requested"
	case nasMessage.Cause5GSMInvalidPDUSessionIdentity:
		return "Invalid PDU Session Identity"
	case nasMessage.Cause5GSMSemanticErrorsInPacketFilter:
		return "Semantic Errors In Packet Filter"
	case nasMessage.Cause5GSMSyntacticalErrorInPacketFilter:
		return "Syntactical Error In Packet Filter"
	case nasMessage.Cause5GSMOutOfLADNServiceArea:
		return "Out Of LADN Service Area"
	case nasMessage.Cause5GSMPTIMismatch:
		return "PTI Mismatch"
	case nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed:
		return "PDU Session Type IPv4 Only Allowed"
	case nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed:
		return "PDU Session Type IPv6 Only Allowed"
	case nasMessage.Cause5GSMPDUSessionDoesNotExist:
		return "PDU Session Does Not Exist"
	case nasMessage.Cause5GSMInsufficientResourcesForSpecificSliceAndDNN:
		return "Insufficient Resources For Specific Slice And DNN"
	case nasMessage.Cause5GSMNotSupportedSSCMode:
		return "Not Supported SSC Mode"
	case nasMessage.Cause5GSMInsufficientResourcesForSpecificSlice:
		return "Insufficient Resources For Specific Slice"
	case nasMessage.Cause5GSMMissingOrUnknownDNNInASlice:
		return "Missing Or Unknown DNN In A Slice"
	case nasMessage.Cause5GSMInvalidPTIValue:
		return "Invalid PTI Value"
	case nasMessage.Cause5GSMMaximumDataRatePerUEForUserPlaneIntegrityProtectionIsTooLow:
		return "Maximum Data Rate Per UE For User Plane Integrity Protection Is Too Low"
	case nasMessage.Cause5GSMSemanticErrorInTheQoSOperation:
		return "Semantic Error In The QoS Operation"
	case nasMessage.Cause5GSMSyntacticalErrorInTheQoSOperation:
		return "Syntactical Error In The QoS Operation"
	case nasMessage.Cause5GSMInvalidMappedEPSBearerIdentity:
		return "Invalid Mapped EPS Bearer Identity"
	case nasMessage.Cause5GSMSemanticallyIncorrectMessage:
		return "Semantically Incorrect Message"
	case nasMessage.Cause5GSMInvalidMandatoryInformation:
		return "Invalid Mandatory Information"
	case nasMessage.Cause5GSMMessageTypeNonExistentOrNotImplemented:
		return "Message Type Non Existent Or Not Implemented"
	case nasMessage.Cause5GSMMessageTypeNotCompatibleWithTheProtocolState:
		return "Message Type Not Compatible With The Protocol State"
	case nasMessage.Cause5GSMInformationElementNonExistentOrNotImplemented:
		return "Information Element Non Existent Or Not Implemented"
	case nasMessage.Cause5GSMConditionalIEError:
		return "Conditional IE Error"
	case nasMessage.Cause5GSMMessageNotCompatibleWithTheProtocolState:
		return "Message Not Compatible With The Protocol State"
	case nasMessage.Cause5GSMProtocolErrorUnspecified:
		return "Protocol Error Unspecified"
	default:
		return fmt.Sprintf("Unknown Cause (%d)", cause)
	}
}
