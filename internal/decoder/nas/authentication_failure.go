package nas

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
)

type AuthenticationFailure struct {
	ExtendedProtocolDiscriminator       uint8                  `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                  `json:"spare_half_octet_and_security_header_type"`
	Cause5GMM                           utils.EnumField[uint8] `json:"cause"`

	AuthenticationFailureParameter *string `json:"authentication_failure_parameter,omitempty"`
}

func buildAuthenticationFailure(msg *nasMessage.AuthenticationFailure) *AuthenticationFailure {
	if msg == nil {
		return nil
	}

	authFailure := &AuthenticationFailure{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		Cause5GMM:                           cause5GMMToEnum(msg.Cause5GMM.GetCauseValue()),
	}

	if msg.AuthenticationFailureParameter != nil {
		authFailParam := buildAuthenticationFailureParameter(msg.AuthenticationFailureParameter)
		authFailure.AuthenticationFailureParameter = &authFailParam
	}

	return authFailure
}

func buildAuthenticationFailureParameter(param *nasType.AuthenticationFailureParameter) string {
	auts := param.GetAuthenticationFailureParameter()
	return hex.EncodeToString(auts[:])
}

func cause5GMMToEnum(cause uint8) utils.EnumField[uint8] {
	switch cause {
	case nasMessage.Cause5GMMIllegalUE:
		return utils.MakeEnum(cause, "Illegal UE", false)
	case nasMessage.Cause5GMMPEINotAccepted:
		return utils.MakeEnum(cause, "PEI not accepted", false)
	case nasMessage.Cause5GMMIllegalME:
		return utils.MakeEnum(cause, "Illegal ME", false)
	case nasMessage.Cause5GMM5GSServicesNotAllowed:
		return utils.MakeEnum(cause, "5GS services not allowed", false)
	case nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork:
		return utils.MakeEnum(cause, "UE identity cannot be derived by the network", false)
	case nasMessage.Cause5GMMImplicitlyDeregistered:
		return utils.MakeEnum(cause, "Implicitly deregistered", false)
	case nasMessage.Cause5GMMPLMNNotAllowed:
		return utils.MakeEnum(cause, "PLMN not allowed", false)
	case nasMessage.Cause5GMMTrackingAreaNotAllowed:
		return utils.MakeEnum(cause, "Tracking area not allowed", false)
	case nasMessage.Cause5GMMRoamingNotAllowedInThisTrackingArea:
		return utils.MakeEnum(cause, "Roaming not allowed in this tracking area", false)
	case nasMessage.Cause5GMMNoSuitableCellsInTrackingArea:
		return utils.MakeEnum(cause, "No suitable cells in tracking area", false)
	case nasMessage.Cause5GMMMACFailure:
		return utils.MakeEnum(cause, "MAC failure", false)
	case nasMessage.Cause5GMMSynchFailure:
		return utils.MakeEnum(cause, "Synch failure", false)
	case nasMessage.Cause5GMMCongestion:
		return utils.MakeEnum(cause, "Congestion", false)
	case nasMessage.Cause5GMMUESecurityCapabilitiesMismatch:
		return utils.MakeEnum(cause, "UE security capabilities mismatch", false)
	case nasMessage.Cause5GMMSecurityModeRejectedUnspecified:
		return utils.MakeEnum(cause, "Security mode rejected, unspecified", false)
	case nasMessage.Cause5GMMNon5GAuthenticationUnacceptable:
		return utils.MakeEnum(cause, "Non 5G authentication unacceptable", false)
	case nasMessage.Cause5GMMN1ModeNotAllowed:
		return utils.MakeEnum(cause, "N1 mode not allowed", false)
	case nasMessage.Cause5GMMRestrictedServiceArea:
		return utils.MakeEnum(cause, "Restricted service area", false)
	case nasMessage.Cause5GMMLADNNotAvailable:
		return utils.MakeEnum(cause, "LADN not available", false)
	case nasMessage.Cause5GMMMaximumNumberOfPDUSessionsReached:
		return utils.MakeEnum(cause, "Maximum number of PDU sessions reached", false)
	case nasMessage.Cause5GMMInsufficientResourcesForSpecificSliceAndDNN:
		return utils.MakeEnum(cause, "Insufficient resources for specific slice and DNN", false)
	case nasMessage.Cause5GMMInsufficientResourcesForSpecificSlice:
		return utils.MakeEnum(cause, "Insufficient resources for specific slice", false)
	case nasMessage.Cause5GMMngKSIAlreadyInUse:
		return utils.MakeEnum(cause, "ngKSI already in use", false)
	case nasMessage.Cause5GMMNon3GPPAccessTo5GCNNotAllowed:
		return utils.MakeEnum(cause, "Non-3GPP access to 5GCN not allowed", false)
	case nasMessage.Cause5GMMServingNetworkNotAuthorized:
		return utils.MakeEnum(cause, "Serving network not authorized", false)
	case nasMessage.Cause5GMMPayloadWasNotForwarded:
		return utils.MakeEnum(cause, "Payload was not forwarded", false)
	case nasMessage.Cause5GMMDNNNotSupportedOrNotSubscribedInTheSlice:
		return utils.MakeEnum(cause, "DNN not supported or not subscribed in the slice", false)
	case nasMessage.Cause5GMMInsufficientUserPlaneResourcesForThePDUSession:
		return utils.MakeEnum(cause, "Insufficient user plane resources for the PDU session", false)
	case nasMessage.Cause5GMMSemanticallyIncorrectMessage:
		return utils.MakeEnum(cause, "Semantically incorrect message", false)
	case nasMessage.Cause5GMMInvalidMandatoryInformation:
		return utils.MakeEnum(cause, "Invalid mandatory information", false)
	case nasMessage.Cause5GMMMessageTypeNonExistentOrNotImplemented:
		return utils.MakeEnum(cause, "Message type non existent or not implemented", false)
	case nasMessage.Cause5GMMMessageTypeNotCompatibleWithTheProtocolState:
		return utils.MakeEnum(cause, "Message type not compatible with the protocol state", false)
	case nasMessage.Cause5GMMInformationElementNonExistentOrNotImplemented:
		return utils.MakeEnum(cause, "Information element non existent or not implemented", false)
	case nasMessage.Cause5GMMConditionalIEError:
		return utils.MakeEnum(cause, "Conditional IE error", false)
	case nasMessage.Cause5GMMMessageNotCompatibleWithTheProtocolState:
		return utils.MakeEnum(cause, "Message not compatible with the protocol state", false)
	case nasMessage.Cause5GMMProtocolErrorUnspecified:
		return utils.MakeEnum(cause, "Protocol error unspecified", false)
	default:
		return utils.MakeEnum(cause, "", true)
	}
}
