package ue

import "github.com/free5gc/nas"

func getGSMMessageName(msgType uint8) string {
	switch msgType {
	case nas.MsgTypePDUSessionEstablishmentRequest:
		return "PDU Session Establishment Request"
	case nas.MsgTypePDUSessionEstablishmentAccept:
		return "PDU Session Establishment Accept"
	case nas.MsgTypePDUSessionEstablishmentReject:
		return "PDU Session Establishment Reject"
	case nas.MsgTypePDUSessionAuthenticationCommand:
		return "PDU Session Authentication Command"
	case nas.MsgTypePDUSessionAuthenticationComplete:
		return "PDU Session Authentication Complete"
	case nas.MsgTypePDUSessionAuthenticationResult:
		return "PDU Session Authentication Result"
	case nas.MsgTypePDUSessionModificationRequest:
		return "PDU Session Modification Request"
	case nas.MsgTypePDUSessionModificationReject:
		return "PDU Session Modification Reject"
	case nas.MsgTypePDUSessionModificationCommand:
		return "PDU Session Modification Command"
	case nas.MsgTypePDUSessionModificationComplete:
		return "PDU Session Modification Complete"
	case nas.MsgTypePDUSessionModificationCommandReject:
		return "PDU Session Modification Command Reject"
	case nas.MsgTypePDUSessionReleaseRequest:
		return "PDU Session Release Request"
	case nas.MsgTypePDUSessionReleaseReject:
		return "PDU Session Release Reject"
	case nas.MsgTypePDUSessionReleaseCommand:
		return "PDU Session Release Command"
	case nas.MsgTypePDUSessionReleaseComplete:
		return "PDU Session Release Complete"
	case nas.MsgTypeStatus5GSM:
		return "5GSM Status"
	default:
		return "Unknown Message Type"
	}
}

func getGMMMessageName(msgType uint8) string {
	switch msgType {
	case nas.MsgTypeRegistrationRequest:
		return "Registration Request"
	case nas.MsgTypeRegistrationAccept:
		return "Registration Accept"
	case nas.MsgTypeRegistrationComplete:
		return "Registration Complete"
	case nas.MsgTypeRegistrationReject:
		return "Registration Reject"
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return "Deregistration Request UE Originating Deregistration"
	case nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration:
		return "Deregistration Accept UE Originating Deregistration"
	case nas.MsgTypeDeregistrationRequestUETerminatedDeregistration:
		return "Deregistration Request UE Terminated Deregistration"
	case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
		return "Deregistration Accept UE Terminated Deregistration"
	case nas.MsgTypeServiceRequest:
		return "Service Request"
	case nas.MsgTypeServiceReject:
		return "Service Reject"
	case nas.MsgTypeServiceAccept:
		return "Service Accept"
	case nas.MsgTypeConfigurationUpdateCommand:
		return "Configuration Update Command"
	case nas.MsgTypeConfigurationUpdateComplete:
		return "Configuration Update Complete"
	case nas.MsgTypeAuthenticationRequest:
		return "Authentication Request"
	case nas.MsgTypeAuthenticationResponse:
		return "Authentication Response"
	case nas.MsgTypeAuthenticationReject:
		return "Authentication Reject"
	case nas.MsgTypeAuthenticationFailure:
		return "Authentication Failure"
	case nas.MsgTypeAuthenticationResult:
		return "Authentication Result"
	case nas.MsgTypeIdentityRequest:
		return "Identity Request"
	case nas.MsgTypeIdentityResponse:
		return "Identity Response"
	case nas.MsgTypeSecurityModeCommand:
		return "Security Mode Command"
	case nas.MsgTypeSecurityModeComplete:
		return "Security Mode Complete"
	case nas.MsgTypeSecurityModeReject:
		return "Security Mode Reject"
	case nas.MsgTypeStatus5GMM:
		return "5GMM Status"
	case nas.MsgTypeNotification:
		return "Notification"
	case nas.MsgTypeNotificationResponse:
		return "Notification Response"
	case nas.MsgTypeULNASTransport:
		return "UL NAS Transport"
	case nas.MsgTypeDLNASTransport:
		return "DL NAS Transport"
	default:
		return "Unknown Message Type"
	}
}
