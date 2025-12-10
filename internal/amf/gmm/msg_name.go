package gmm

import "github.com/free5gc/nas"

func getMessageName(msgType uint8) string {
	switch msgType {
	case nas.MsgTypeRegistrationRequest:
		return "RegistrationRequest"
	case nas.MsgTypeRegistrationAccept:
		return "RegistrationAccept"
	case nas.MsgTypeRegistrationComplete:
		return "RegistrationComplete"
	case nas.MsgTypeRegistrationReject:
		return "RegistrationReject"
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return "DeregistrationRequestUEOriginatingDeregistration"
	case nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration:
		return "DeregistrationAcceptUEOriginatingDeregistration"
	case nas.MsgTypeDeregistrationRequestUETerminatedDeregistration:
		return "DeregistrationRequestUETerminatedDeregistration"
	case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
		return "DeregistrationAcceptUETerminatedDeregistration"
	case nas.MsgTypeServiceRequest:
		return "ServiceRequest"
	case nas.MsgTypeServiceReject:
		return "ServiceReject"
	case nas.MsgTypeServiceAccept:
		return "ServiceAccept"
	case nas.MsgTypeConfigurationUpdateCommand:
		return "ConfigurationUpdateCommand"
	case nas.MsgTypeConfigurationUpdateComplete:
		return "ConfigurationUpdateComplete"
	case nas.MsgTypeAuthenticationRequest:
		return "AuthenticationRequest"
	case nas.MsgTypeAuthenticationResponse:
		return "AuthenticationResponse"
	case nas.MsgTypeAuthenticationReject:
		return "AuthenticationReject"
	case nas.MsgTypeAuthenticationFailure:
		return "AuthenticationFailure"
	case nas.MsgTypeAuthenticationResult:
		return "AuthenticationResult"
	case nas.MsgTypeIdentityRequest:
		return "IdentityRequest"
	case nas.MsgTypeIdentityResponse:
		return "IdentityResponse"
	case nas.MsgTypeSecurityModeCommand:
		return "SecurityModeCommand"
	case nas.MsgTypeSecurityModeComplete:
		return "SecurityModeComplete"
	case nas.MsgTypeSecurityModeReject:
		return "SecurityModeReject"
	case nas.MsgTypeStatus5GMM:
		return "Status5GMM"
	case nas.MsgTypeNotification:
		return "Notification"
	case nas.MsgTypeNotificationResponse:
		return "NotificationResponse"
	case nas.MsgTypeULNASTransport:
		return "ULNASTransport"
	case nas.MsgTypeDLNASTransport:
		return "DLNASTransport"
	default:
		return "Unknown"
	}
}
