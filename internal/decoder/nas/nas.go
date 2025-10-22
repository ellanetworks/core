package nas

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
)

type GmmHeader struct {
	MessageType utils.EnumField[uint8] `json:"message_type"`
}

type GmmMessage struct {
	GmmHeader GmmHeader `json:"gmm_header"`
	Error     string    `json:"error,omitempty"`

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
	MessageType utils.EnumField[uint8] `json:"message_type"`
}

type GsmMessage struct {
	GsmHeader GsmHeader `json:"gsm_header"`
	Error     string    `json:"error,omitempty"`

	PDUSessionEstablishmentRequest *PDUSessionEstablishmentRequest `json:"pdu_session_establishment_request,omitempty"`
	PDUSessionEstablishmentAccept  *PDUSessionEstablishmentAccept  `json:"pdu_session_establishment_accept,omitempty"`
}

type SecurityHeader struct {
	ProtocolDiscriminator     utils.EnumField[uint8] `json:"protocol_discriminator"`
	SecurityHeaderType        utils.EnumField[uint8] `json:"security_header_type"`
	MessageAuthenticationCode uint32                 `json:"authentication_code,omitempty"`
	SequenceNumber            uint8                  `json:"sequence_number"`
}

type NASMessage struct {
	SecurityHeader SecurityHeader `json:"security_header"`
	GmmMessage     *GmmMessage    `json:"gmm_message,omitempty"`
	GsmMessage     *GsmMessage    `json:"gsm_message,omitempty"`

	Error string `json:"error,omitempty"` // Reserved field for decoding errors
}

type NasContextInfo struct {
	Direction   Direction
	AMFUENGAPID int64
}

func DecodeNASMessage(raw []byte, nasContextInfo *NasContextInfo) *NASMessage {
	msg, err := decodeNAS(raw, nasContextInfo)
	if err != nil {
		return &NASMessage{
			Error: fmt.Sprintf("failed to decode NAS message: %v", err),
		}
	}

	nasMsg := &NASMessage{
		SecurityHeader: SecurityHeader{
			SecurityHeaderType:        securityHeaderTypeToEnum(msg.SecurityHeaderType),
			MessageAuthenticationCode: msg.MessageAuthenticationCode,
			SequenceNumber:            msg.SequenceNumber,
		},
	}

	epd := nas.GetEPD(raw)
	switch epd {
	case nasMessage.Epd5GSMobilityManagementMessage:
		nasMsg.GmmMessage = buildGmmMessage(msg.GmmMessage)
		nasMsg.SecurityHeader.ProtocolDiscriminator = utils.MakeEnum(epd, "5GSMobilityManagementMessage", false)
	case nasMessage.Epd5GSSessionManagementMessage:
		nasMsg.GsmMessage = buildGsmMessage(msg.GsmMessage)
		nasMsg.SecurityHeader.ProtocolDiscriminator = utils.MakeEnum(epd, "5GSSessionManagementMessage", false)
	default:
		nasMsg.SecurityHeader.ProtocolDiscriminator = utils.MakeEnum(epd, "", true)
	}

	return nasMsg
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
		gmmMessage.Error = fmt.Sprintf("GMM message type %d not implemented", msg.GetMessageType())
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

func getGsmMessageType(msg *nas.GsmMessage) utils.EnumField[uint8] {
	switch msg.GetMessageType() {
	case nas.MsgTypePDUSessionEstablishmentRequest:
		return utils.MakeEnum(nas.MsgTypePDUSessionEstablishmentRequest, "PDUSessionEstablishmentRequest", false)
	case nas.MsgTypePDUSessionEstablishmentAccept:
		return utils.MakeEnum(nas.MsgTypePDUSessionEstablishmentAccept, "PDUSessionEstablishmentAccept", false)
	case nas.MsgTypePDUSessionEstablishmentReject:
		return utils.MakeEnum(nas.MsgTypePDUSessionEstablishmentReject, "PDUSessionEstablishmentReject", false)
	case nas.MsgTypePDUSessionAuthenticationCommand:
		return utils.MakeEnum(nas.MsgTypePDUSessionAuthenticationCommand, "PDUSessionAuthenticationCommand", false)
	case nas.MsgTypePDUSessionAuthenticationComplete:
		return utils.MakeEnum(nas.MsgTypePDUSessionAuthenticationComplete, "PDUSessionAuthenticationComplete", false)
	case nas.MsgTypePDUSessionAuthenticationResult:
		return utils.MakeEnum(nas.MsgTypePDUSessionAuthenticationResult, "PDUSessionAuthenticationResult", false)
	case nas.MsgTypePDUSessionModificationRequest:
		return utils.MakeEnum(nas.MsgTypePDUSessionModificationRequest, "PDUSessionModificationRequest", false)
	case nas.MsgTypePDUSessionModificationReject:
		return utils.MakeEnum(nas.MsgTypePDUSessionModificationReject, "PDUSessionModificationReject", false)
	case nas.MsgTypePDUSessionModificationCommand:
		return utils.MakeEnum(nas.MsgTypePDUSessionModificationCommand, "PDUSessionModificationCommand", false)
	case nas.MsgTypePDUSessionModificationComplete:
		return utils.MakeEnum(nas.MsgTypePDUSessionModificationComplete, "PDUSessionModificationComplete", false)
	case nas.MsgTypePDUSessionModificationCommandReject:
		return utils.MakeEnum(nas.MsgTypePDUSessionModificationCommandReject, "PDUSessionModificationCommandReject", false)
	case nas.MsgTypePDUSessionReleaseRequest:
		return utils.MakeEnum(nas.MsgTypePDUSessionReleaseRequest, "PDUSessionReleaseRequest", false)
	case nas.MsgTypePDUSessionReleaseReject:
		return utils.MakeEnum(nas.MsgTypePDUSessionReleaseReject, "PDUSessionReleaseReject", false)
	case nas.MsgTypePDUSessionReleaseCommand:
		return utils.MakeEnum(nas.MsgTypePDUSessionReleaseCommand, "PDUSessionReleaseCommand", false)
	case nas.MsgTypePDUSessionReleaseComplete:
		return utils.MakeEnum(nas.MsgTypePDUSessionReleaseComplete, "PDUSessionReleaseComplete", false)
	case nas.MsgTypeStatus5GSM:
		return utils.MakeEnum(nas.MsgTypeStatus5GSM, "Status5GSM", false)
	default:
		return utils.MakeEnum(msg.GetMessageType(), "", false)
	}
}

func getGmmMessageType(msg *nas.GmmMessage) utils.EnumField[uint8] {
	switch msg.GetMessageType() {
	case nas.MsgTypeRegistrationRequest:
		return utils.MakeEnum(nas.MsgTypeRegistrationRequest, "RegistrationRequest", false)
	case nas.MsgTypeRegistrationAccept:
		return utils.MakeEnum(nas.MsgTypeRegistrationAccept, "RegistrationAccept", false)
	case nas.MsgTypeRegistrationComplete:
		return utils.MakeEnum(nas.MsgTypeRegistrationComplete, "RegistrationComplete", false)
	case nas.MsgTypeRegistrationReject:
		return utils.MakeEnum(nas.MsgTypeRegistrationReject, "RegistrationReject", false)
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return utils.MakeEnum(nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration, "DeregistrationRequestUEOriginatingDeregistration", false)
	case nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration:
		return utils.MakeEnum(nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration, "DeregistrationAcceptUEOriginatingDeregistration", false)
	case nas.MsgTypeDeregistrationRequestUETerminatedDeregistration:
		return utils.MakeEnum(nas.MsgTypeDeregistrationRequestUETerminatedDeregistration, "DeregistrationRequestUETerminatedDeregistration", false)
	case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
		return utils.MakeEnum(nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration, "DeregistrationAcceptUETerminatedDeregistration", false)
	case nas.MsgTypeServiceRequest:
		return utils.MakeEnum(nas.MsgTypeServiceRequest, "ServiceRequest", false)
	case nas.MsgTypeServiceReject:
		return utils.MakeEnum(nas.MsgTypeServiceReject, "ServiceReject", false)
	case nas.MsgTypeServiceAccept:
		return utils.MakeEnum(nas.MsgTypeServiceAccept, "ServiceAccept", false)
	case nas.MsgTypeConfigurationUpdateCommand:
		return utils.MakeEnum(nas.MsgTypeConfigurationUpdateCommand, "ConfigurationUpdateCommand", false)
	case nas.MsgTypeConfigurationUpdateComplete:
		return utils.MakeEnum(nas.MsgTypeConfigurationUpdateComplete, "ConfigurationUpdateComplete", false)
	case nas.MsgTypeAuthenticationRequest:
		return utils.MakeEnum(nas.MsgTypeAuthenticationRequest, "AuthenticationRequest", false)
	case nas.MsgTypeAuthenticationResponse:
		return utils.MakeEnum(nas.MsgTypeAuthenticationResponse, "AuthenticationResponse", false)
	case nas.MsgTypeAuthenticationReject:
		return utils.MakeEnum(nas.MsgTypeAuthenticationReject, "AuthenticationReject", false)
	case nas.MsgTypeAuthenticationFailure:
		return utils.MakeEnum(nas.MsgTypeAuthenticationFailure, "AuthenticationFailure", false)
	case nas.MsgTypeAuthenticationResult:
		return utils.MakeEnum(nas.MsgTypeAuthenticationResult, "AuthenticationResult", false)
	case nas.MsgTypeIdentityRequest:
		return utils.MakeEnum(nas.MsgTypeIdentityRequest, "IdentityRequest", false)
	case nas.MsgTypeIdentityResponse:
		return utils.MakeEnum(nas.MsgTypeIdentityResponse, "IdentityResponse", false)
	case nas.MsgTypeSecurityModeCommand:
		return utils.MakeEnum(nas.MsgTypeSecurityModeCommand, "SecurityModeCommand", false)
	case nas.MsgTypeSecurityModeComplete:
		return utils.MakeEnum(nas.MsgTypeSecurityModeComplete, "SecurityModeComplete", false)
	case nas.MsgTypeSecurityModeReject:
		return utils.MakeEnum(nas.MsgTypeSecurityModeReject, "SecurityModeReject", false)
	case nas.MsgTypeStatus5GMM:
		return utils.MakeEnum(nas.MsgTypeStatus5GMM, "Status5GMM", false)
	case nas.MsgTypeNotification:
		return utils.MakeEnum(nas.MsgTypeNotification, "Notification", false)
	case nas.MsgTypeNotificationResponse:
		return utils.MakeEnum(nas.MsgTypeNotificationResponse, "NotificationResponse", false)
	case nas.MsgTypeULNASTransport:
		return utils.MakeEnum(nas.MsgTypeULNASTransport, "ULNASTransport", false)
	case nas.MsgTypeDLNASTransport:
		return utils.MakeEnum(nas.MsgTypeDLNASTransport, "DLNASTransport", false)
	default:
		return utils.MakeEnum(msg.GetMessageType(), "", true)
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
			return nil, fmt.Errorf("cannot find ue in amf")
		}

		if ranUE.AmfUe == nil {
			return nil, fmt.Errorf("ue decryption keys are not available")
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
			return nil, fmt.Errorf("cannot find ue in amf")
		}

		if ranUE.AmfUe == nil {
			return nil, fmt.Errorf("ue decryption keys are not available")
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
			return nil, fmt.Errorf("cannot find ue in amf")
		}

		if ranUE.AmfUe == nil {
			return nil, fmt.Errorf("ue decryption keys are not available")
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

func securityHeaderTypeToEnum(msgType uint8) utils.EnumField[uint8] {
	switch msgType {
	case nas.SecurityHeaderTypePlainNas:
		return utils.MakeEnum(msgType, "Plain NAS", false)
	case nas.SecurityHeaderTypeIntegrityProtected:
		return utils.MakeEnum(msgType, "Integrity Protected", false)
	case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
		return utils.MakeEnum(msgType, "Integrity Protected and Ciphered", false)
	case nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext:
		return utils.MakeEnum(msgType, "Integrity Protected with New 5G NAS Security Context", false)
	case nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext:
		return utils.MakeEnum(msgType, "Integrity Protected and Ciphered with New 5G NAS Security Context", false)
	default:
		return utils.MakeEnum(msgType, "", true)
	}
}
