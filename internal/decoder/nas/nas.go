package nas

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
	"go.uber.org/zap"
)

type SecurityHeader struct {
	ProtocolDiscriminator     uint8  `json:"protocol_discriminator"`
	SecurityHeaderType        string `json:"security_header_type"`
	MessageAuthenticationCode uint32 `json:"message_authentication_code,omitempty"`
	SequenceNumber            uint8  `json:"sequence_number"`
}

type GmmHeader struct {
	MessageType string `json:"message_type"`
}

type GmmMessage struct {
	GmmHeader              GmmHeader               `json:"gmm_header"`
	RegistrationRequest    *RegistrationRequest    `json:"registration_request,omitempty"`
	RegistrationAccept     *RegistrationAccept     `json:"registration_accept,omitempty"`
	RegistrationReject     *RegistrationReject     `json:"registration_reject,omitempty"`
	RegistrationComplete   *RegistrationComplete   `json:"registration_complete,omitempty"`
	AuthenticationRequest  *AuthenticationRequest  `json:"authentication_request,omitempty"`
	AuthenticationFailure  *AuthenticationFailure  `json:"authentication_failure,omitempty"`
	AuthenticationReject   *AuthenticationReject   `json:"authentication_reject,omitempty"`
	AuthenticationResponse *AuthenticationResponse `json:"authentication_response,omitempty"`
	ULNASTransport         *ULNASTransport         `json:"ul_nas_transport,omitempty"`
	DLNASTransport         *DLNASTransport         `json:"dl_nas_transport,omitempty"`
	SecurityModeCommand    *SecurityModeCommand    `json:"security_mode_command,omitempty"`
	SecurityModeComplete   *SecurityModeComplete   `json:"security_mode_complete,omitempty"`
	ServiceRequest         *ServiceRequest         `json:"service_request,omitempty"`
	ServiceAccept          *ServiceAccept          `json:"service_accept,omitempty"`
	ServiceReject          *ServiceReject          `json:"service_reject,omitempty"`
}

type GsmHeader struct {
	MessageType string `json:"message_type"`
}

type GsmMessage struct {
	GsmHeader                      GsmHeader                       `json:"gsm_header"`
	PDUSessionEstablishmentRequest *PDUSessionEstablishmentRequest `json:"pdu_session_establishment_request,omitempty"`
	PDUSessionEstablishmentAccept  *PDUSessionEstablishmentAccept  `json:"pdu_session_establishment_accept,omitempty"`
}

type NASMessage struct {
	SecurityHeader SecurityHeader `json:"security_header"`
	GmmMessage     *GmmMessage    `json:"gmm_message,omitempty"`
	GsmMessage     *GsmMessage    `json:"gsm_message,omitempty"`
}

type NasContextInfo struct {
	Direction   Direction
	AMFUENGAPID int64
}

func DecodeNASMessage(raw []byte, nasContextInfo *NasContextInfo) (*NASMessage, error) {
	nasMsg := new(NASMessage)

	msg, err := decodeNAS(raw, nasContextInfo)
	if err != nil {
		return nil, err
	}

	nasMsg.SecurityHeader = buildSecurityHeader(msg)

	epd := nas.GetEPD(raw)
	switch epd {
	case nasMessage.Epd5GSMobilityManagementMessage:
		nasMsg.GmmMessage = buildGmmMessage(msg.GmmMessage)
	case nasMessage.Epd5GSSessionManagementMessage:
		nasMsg.GsmMessage = buildGsmMessage(msg.GsmMessage)
	default:
		return nil, fmt.Errorf("unsupported EPD: %d", epd)
	}

	return nasMsg, nil
}

func buildGmmMessage(msg *nas.GmmMessage) *GmmMessage {
	if msg == nil {
		return nil
	}
	gmmMessage := &GmmMessage{
		GmmHeader: GmmHeader{
			MessageType: getGmmMessageType(msg),
		},
	}

	switch msg.GetMessageType() {
	case nas.MsgTypeRegistrationRequest:
		gmmMessage.RegistrationRequest = buildRegistrationRequest(msg.RegistrationRequest)
		return gmmMessage
	case nas.MsgTypeRegistrationAccept:
		gmmMessage.RegistrationAccept = buildRegistrationAccept(msg.RegistrationAccept)
		return gmmMessage
	case nas.MsgTypeRegistrationReject:
		gmmMessage.RegistrationReject = buildRegistrationReject(msg.RegistrationReject)
		return gmmMessage
	case nas.MsgTypeRegistrationComplete:
		gmmMessage.RegistrationComplete = buildRegistrationComplete(msg.RegistrationComplete)
		return gmmMessage
	case nas.MsgTypeAuthenticationRequest:
		gmmMessage.AuthenticationRequest = buildAuthenticationRequest(msg.AuthenticationRequest)
		return gmmMessage
	case nas.MsgTypeAuthenticationFailure:
		gmmMessage.AuthenticationFailure = buildAuthenticationFailure(msg.AuthenticationFailure)
		return gmmMessage
	case nas.MsgTypeAuthenticationReject:
		gmmMessage.AuthenticationReject = buildAuthenticationReject(msg.AuthenticationReject)
		return gmmMessage
	case nas.MsgTypeAuthenticationResponse:
		gmmMessage.AuthenticationResponse = buildAuthenticationResponse(msg.AuthenticationResponse)
		return gmmMessage
	case nas.MsgTypeULNASTransport:
		gmmMessage.ULNASTransport = buildULNASTransport(msg.ULNASTransport)
		return gmmMessage
	case nas.MsgTypeDLNASTransport:
		gmmMessage.DLNASTransport = buildDLNASTransport(msg.DLNASTransport)
		return gmmMessage
	case nas.MsgTypeSecurityModeCommand:
		gmmMessage.SecurityModeCommand = buildSecurityModeCommand(msg.SecurityModeCommand)
		return gmmMessage
	case nas.MsgTypeSecurityModeComplete:
		gmmMessage.SecurityModeComplete = buildSecurityModeComplete(msg.SecurityModeComplete)
		return gmmMessage
	case nas.MsgTypeServiceRequest:
		gmmMessage.ServiceRequest = buildServiceRequest(msg.ServiceRequest)
		return gmmMessage
	case nas.MsgTypeServiceAccept:
		gmmMessage.ServiceAccept = buildServiceAccept(msg.ServiceAccept)
		return gmmMessage
	case nas.MsgTypeServiceReject:
		gmmMessage.ServiceReject = buildServiceReject(msg.ServiceReject)
		return gmmMessage
	default:
		logger.EllaLog.Warn("GMM message type not fully implemented", zap.String("message_type", gmmMessage.GmmHeader.MessageType))
		return gmmMessage
	}
}

func buildGsmMessage(msg *nas.GsmMessage) *GsmMessage {
	if msg == nil {
		return nil
	}

	return &GsmMessage{
		GsmHeader: GsmHeader{
			MessageType: getGsmMessageType(msg),
		},
	}
}

func getGsmMessageType(msg *nas.GsmMessage) string {
	switch msg.GetMessageType() {
	case nas.MsgTypePDUSessionEstablishmentRequest:
		return fmt.Sprintf("PDUSessionEstablishmentRequest (%v)", nas.MsgTypePDUSessionEstablishmentRequest)
	case nas.MsgTypePDUSessionEstablishmentAccept:
		return fmt.Sprintf("PDUSessionEstablishmentAccept (%v)", nas.MsgTypePDUSessionEstablishmentAccept)
	case nas.MsgTypePDUSessionEstablishmentReject:
		return fmt.Sprintf("PDUSessionEstablishmentReject (%v)", nas.MsgTypePDUSessionEstablishmentReject)
	case nas.MsgTypePDUSessionAuthenticationCommand:
		return fmt.Sprintf("PDUSessionAuthenticationCommand (%v)", nas.MsgTypePDUSessionAuthenticationCommand)
	case nas.MsgTypePDUSessionAuthenticationComplete:
		return fmt.Sprintf("PDUSessionAuthenticationComplete (%v)", nas.MsgTypePDUSessionAuthenticationComplete)
	case nas.MsgTypePDUSessionAuthenticationResult:
		return fmt.Sprintf("PDUSessionAuthenticationResult (%v)", nas.MsgTypePDUSessionAuthenticationResult)
	case nas.MsgTypePDUSessionModificationRequest:
		return fmt.Sprintf("PDUSessionModificationRequest (%v)", nas.MsgTypePDUSessionModificationRequest)
	case nas.MsgTypePDUSessionModificationReject:
		return fmt.Sprintf("PDUSessionModificationReject (%v)", nas.MsgTypePDUSessionModificationReject)
	case nas.MsgTypePDUSessionModificationCommand:
		return fmt.Sprintf("PDUSessionModificationCommand (%v)", nas.MsgTypePDUSessionModificationCommand)
	case nas.MsgTypePDUSessionModificationComplete:
		return fmt.Sprintf("PDUSessionModificationComplete (%v)", nas.MsgTypePDUSessionModificationComplete)
	case nas.MsgTypePDUSessionModificationCommandReject:
		return fmt.Sprintf("PDUSessionModificationCommandReject (%v)", nas.MsgTypePDUSessionModificationCommandReject)
	case nas.MsgTypePDUSessionReleaseRequest:
		return fmt.Sprintf("PDUSessionReleaseRequest (%v)", nas.MsgTypePDUSessionReleaseRequest)
	case nas.MsgTypePDUSessionReleaseReject:
		return fmt.Sprintf("PDUSessionReleaseReject (%v)", nas.MsgTypePDUSessionReleaseReject)
	case nas.MsgTypePDUSessionReleaseCommand:
		return fmt.Sprintf("PDUSessionReleaseCommand (%v)", nas.MsgTypePDUSessionReleaseCommand)
	case nas.MsgTypePDUSessionReleaseComplete:
		return fmt.Sprintf("PDUSessionReleaseComplete (%v)", nas.MsgTypePDUSessionReleaseComplete)
	case nas.MsgTypeStatus5GSM:
		return fmt.Sprintf("Status5GSM (%v)", nas.MsgTypeStatus5GSM)
	default:
		return fmt.Sprintf("Unknown (%v)", msg.GetMessageType())
	}
}

func getGmmMessageType(msg *nas.GmmMessage) string {
	switch msg.GetMessageType() {
	case nas.MsgTypeRegistrationRequest:
		return fmt.Sprintf("RegistrationRequest (%v)", nas.MsgTypeRegistrationRequest)
	case nas.MsgTypeRegistrationAccept:
		return fmt.Sprintf("RegistrationAccept (%v)", nas.MsgTypeRegistrationAccept)
	case nas.MsgTypeRegistrationComplete:
		return fmt.Sprintf("RegistrationComplete (%v)", nas.MsgTypeRegistrationComplete)
	case nas.MsgTypeRegistrationReject:
		return fmt.Sprintf("RegistrationReject (%v)", nas.MsgTypeRegistrationReject)
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return fmt.Sprintf("DeregistrationRequestUEOriginatingDeregistration (%v)", nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration)
	case nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration:
		return fmt.Sprintf("DeregistrationAcceptUEOriginatingDeregistration (%v)", nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration)
	case nas.MsgTypeDeregistrationRequestUETerminatedDeregistration:
		return fmt.Sprintf("DeregistrationRequestUETerminatedDeregistration (%v)", nas.MsgTypeDeregistrationRequestUETerminatedDeregistration)
	case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
		return fmt.Sprintf("DeregistrationAcceptUETerminatedDeregistration (%v)", nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration)
	case nas.MsgTypeServiceRequest:
		return fmt.Sprintf("ServiceRequest (%v)", nas.MsgTypeServiceRequest)
	case nas.MsgTypeServiceReject:
		return fmt.Sprintf("ServiceReject (%v)", nas.MsgTypeServiceReject)
	case nas.MsgTypeServiceAccept:
		return fmt.Sprintf("ServiceAccept (%v)", nas.MsgTypeServiceAccept)
	case nas.MsgTypeConfigurationUpdateCommand:
		return fmt.Sprintf("ConfigurationUpdateCommand (%v)", nas.MsgTypeConfigurationUpdateCommand)
	case nas.MsgTypeConfigurationUpdateComplete:
		return fmt.Sprintf("ConfigurationUpdateComplete (%v)", nas.MsgTypeConfigurationUpdateComplete)
	case nas.MsgTypeAuthenticationRequest:
		return fmt.Sprintf("AuthenticationRequest (%v)", nas.MsgTypeAuthenticationRequest)
	case nas.MsgTypeAuthenticationResponse:
		return fmt.Sprintf("AuthenticationResponse (%v)", nas.MsgTypeAuthenticationResponse)
	case nas.MsgTypeAuthenticationReject:
		return fmt.Sprintf("AuthenticationReject (%v)", nas.MsgTypeAuthenticationReject)
	case nas.MsgTypeAuthenticationFailure:
		return fmt.Sprintf("AuthenticationFailure (%v)", nas.MsgTypeAuthenticationFailure)
	case nas.MsgTypeAuthenticationResult:
		return fmt.Sprintf("AuthenticationResult (%v)", nas.MsgTypeAuthenticationResult)
	case nas.MsgTypeIdentityRequest:
		return fmt.Sprintf("IdentityRequest (%v)", nas.MsgTypeIdentityRequest)
	case nas.MsgTypeIdentityResponse:
		return fmt.Sprintf("IdentityResponse (%v)", nas.MsgTypeIdentityResponse)
	case nas.MsgTypeSecurityModeCommand:
		return fmt.Sprintf("SecurityModeCommand (%v)", nas.MsgTypeSecurityModeCommand)
	case nas.MsgTypeSecurityModeComplete:
		return fmt.Sprintf("SecurityModeComplete (%v)", nas.MsgTypeSecurityModeComplete)
	case nas.MsgTypeSecurityModeReject:
		return fmt.Sprintf("SecurityModeReject (%v)", nas.MsgTypeSecurityModeReject)
	case nas.MsgTypeStatus5GMM:
		return fmt.Sprintf("Status5GMM (%v)", nas.MsgTypeStatus5GMM)
	case nas.MsgTypeNotification:
		return fmt.Sprintf("Notification (%v)", nas.MsgTypeNotification)
	case nas.MsgTypeNotificationResponse:
		return fmt.Sprintf("NotificationResponse (%v)", nas.MsgTypeNotificationResponse)
	case nas.MsgTypeULNASTransport:
		return fmt.Sprintf("ULNASTransport (%v)", nas.MsgTypeULNASTransport)
	case nas.MsgTypeDLNASTransport:
		return fmt.Sprintf("DLNASTransport (%v)", nas.MsgTypeDLNASTransport)
	default:
		return fmt.Sprintf("Unknown (%v)", msg.GetMessageType())
	}
}

func decodeNAS(raw []byte, nasContextInfo *NasContextInfo) (*nas.Message, error) {
	msg := new(nas.Message)
	msg.SecurityHeaderType = nas.GetSecurityHeaderType(raw) & 0x0f

	switch msg.SecurityHeaderType {
	case nas.SecurityHeaderTypePlainNas:
		if err := msg.PlainNasDecode(&raw); err != nil {
			return nil, fmt.Errorf("failed to decode NAS message: %w", err)
		}
	case nas.SecurityHeaderTypeIntegrityProtected:
		p := raw[7:]
		if err := msg.PlainNasDecode(&p); err != nil {
			return nil, fmt.Errorf("failed to decode NAS message: %w", err)
		}
	case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
		if nasContextInfo == nil {
			return nil, fmt.Errorf("nas context info is nil")
		}

		amf := context.AMFSelf()

		ranUE := amf.RanUeFindByAmfUeNgapID(nasContextInfo.AMFUENGAPID)
		if ranUE == nil {
			return nil, fmt.Errorf("ran ue is nil")
		}

		if ranUE.AmfUe == nil {
			return nil, fmt.Errorf("amf ue is nil")
		}

		decrypted, err := DecryptNASMessage(ranUE.AmfUe, nasContextInfo.Direction, raw)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt NAS message: %w", err)
		}

		err = msg.PlainNasDecode(&decrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decode NAS message: %w", err)
		}
	case nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext:
		if nasContextInfo == nil {
			return nil, fmt.Errorf("nas context info is nil")
		}

		amf := context.AMFSelf()

		ranUE := amf.RanUeFindByAmfUeNgapID(nasContextInfo.AMFUENGAPID)
		if ranUE == nil {
			return nil, fmt.Errorf("ran ue is nil")
		}

		if ranUE.AmfUe == nil {
			return nil, fmt.Errorf("amf ue is nil")
		}

		decrypted, err := DecryptNASMessage(ranUE.AmfUe, nasContextInfo.Direction, raw)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt NAS message: %w", err)
		}

		err = msg.PlainNasDecode(&decrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decode NAS message: %w", err)
		}
	case nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext:
		if nasContextInfo == nil {
			return nil, fmt.Errorf("nas context info is nil")
		}

		amf := context.AMFSelf()

		ranUE := amf.RanUeFindByAmfUeNgapID(nasContextInfo.AMFUENGAPID)
		if ranUE == nil {
			return nil, fmt.Errorf("ran ue is nil")
		}

		if ranUE.AmfUe == nil {
			return nil, fmt.Errorf("amf ue is nil")
		}

		decrypted, err := DecryptNASMessage(ranUE.AmfUe, nasContextInfo.Direction, raw)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt NAS message: %w", err)
		}

		err = msg.PlainNasDecode(&decrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decode NAS message: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported security header type: %d", msg.SecurityHeaderType)
	}

	return msg, nil
}

func buildSecurityHeader(msg *nas.Message) SecurityHeader {
	securityHeaderType := ""
	switch msg.SecurityHeaderType {
	case nas.SecurityHeaderTypePlainNas:
		securityHeaderType = "Plain NAS"
	case nas.SecurityHeaderTypeIntegrityProtected:
		securityHeaderType = "Integrity Protected"
	case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
		securityHeaderType = "Integrity Protected and Ciphered"
	case nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext:
		securityHeaderType = "Integrity Protected with New 5G NAS Security Context"
	case nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext:
		securityHeaderType = "Integrity Protected and Ciphered with New 5G NAS Security Context"
	default:
		securityHeaderType = "Unknown"
	}

	return SecurityHeader{
		ProtocolDiscriminator:     msg.ProtocolDiscriminator,
		SecurityHeaderType:        securityHeaderType,
		MessageAuthenticationCode: msg.MessageAuthenticationCode,
		SequenceNumber:            msg.SequenceNumber,
	}
}
