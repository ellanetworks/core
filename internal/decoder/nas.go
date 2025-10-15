package decoder

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
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

type MobileIdentity5GS struct {
	Identity string
	PLMNID   *PLMNID `json:"plmn_id,omitempty"`
	SUCI     *string `json:"suci,omitempty"`
	GUTI     *string `json:"guti,omitempty"`
	STMSI    *string `json:"s_tmsi,omitempty"`
	IMEI     *string `json:"imei,omitempty"`
	IMEISV   *string `json:"imeisv,omitempty"`
}

type IntegrityAlgorithm struct {
	EEA0_5G     bool `json:"eea0_5g"`
	EEA1_128_5G bool `json:"eea1_128_5g"`
	EEA2_128_5G bool `json:"eea2_128_5g"`
	EEA3_128_5G bool `json:"eea3_128_5g"`
}

type CipheringAlgorithm struct {
	EIA0_5G     bool `json:"eia0_5g"`
	EIA1_128_5G bool `json:"eia1_128_5g"`
	EIA2_128_5G bool `json:"eia2_128_5g"`
	EIA3_128_5G bool `json:"eia3_128_5g"`
}

type UESecurityCapability struct {
	IntegrityAlgorithm IntegrityAlgorithm `json:"integrity_algorithm"`
	CipheringAlgorithm CipheringAlgorithm `json:"ciphering_algorithm"`
}

type AuthenticationRequest struct {
	ExtendedProtocolDiscriminator        uint8     `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType  uint8     `json:"spare_half_octet_and_security_header_type"`
	AuthenticationRequestMessageIdentity string    `json:"authentication_request_message_identity"`
	SpareHalfOctetAndNgksi               uint8     `json:"spare_half_octet_and_ngksi"`
	ABBA                                 []uint8   `json:"abba"`
	AuthenticationParameterAUTN          [16]uint8 `json:"authentication_parameter_autn,omitempty"`
	AuthenticationParameterRAND          [16]uint8 `json:"authentication_parameter_rand,omitempty"`
}

// nasType.ExtendedProtocolDiscriminator
// 	nasType.SpareHalfOctetAndSecurityHeaderType
// 	nasType.AuthenticationRequestMessageIdentity
// 	nasType.SpareHalfOctetAndNgksi
// 	nasType.ABBA
// 	*nasType.AuthenticationParameterRAND
// 	*nasType.AuthenticationParameterAUTN
// 	*nasType.EAPMessage

type RegistrationRequest struct {
	ExtendedProtocolDiscriminator       uint8                 `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                 `json:"spare_half_octet_and_security_header_type"`
	RegistrationRequestMessageIdentity  string                `json:"registration_request_message_identity"`
	NgksiAndRegistrationType5GS         uint8                 `json:"ngksi_and_registration_type_5gs"`
	MobileIdentity5GS                   MobileIdentity5GS     `json:"mobile_identity_5gs"`
	UESecurityCapability                *UESecurityCapability `json:"ue_security_capability,omitempty"`
}

type AuthenticationFailure struct {
	ExtendedProtocolDiscriminator        uint8  `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType  uint8  `json:"spare_half_octet_and_security_header_type"`
	AuthenticationFailureMessageIdentity string `json:"authentication_failure_message_identity"`
	Cause5GMM                            string `json:"cause"`
}

type AuthenticationReject struct {
	ExtendedProtocolDiscriminator       uint8  `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8  `json:"spare_half_octet_and_security_header_type"`
	AuthenticationRejectMessageIdentity string `json:"authentication_reject_message_identity"`
}

type GmmMessage struct {
	GmmHeader             GmmHeader              `json:"gmm_header"`
	RegistrationRequest   *RegistrationRequest   `json:"registration_request,omitempty"`
	AuthenticationRequest *AuthenticationRequest `json:"authentication_request,omitempty"`
	AuthenticationFailure *AuthenticationFailure `json:"authentication_failure,omitempty"`
	AuthenticationReject  *AuthenticationReject  `json:"authentication_reject,omitempty"`
}

type GsmHeader struct {
	MessageType string `json:"message_type"`
}

type GsmMessage struct {
	GsmHeader GsmHeader `json:"gsm_header"`
}

type NASMessage struct {
	SecurityHeader SecurityHeader `json:"security_header"`
	GmmMessage     *GmmMessage    `json:"gmm_message,omitempty"`
	GsmMessage     *GsmMessage    `json:"gsm_message,omitempty"`
}

func DecodeNASMessage(raw []byte) (*NASMessage, error) {
	nasMsg := new(NASMessage)

	msg, err := decodeNAS(raw)
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
	case nas.MsgTypeAuthenticationRequest:
		gmmMessage.AuthenticationRequest = buildAuthenticationRequest(msg.AuthenticationRequest)
		return gmmMessage
	case nas.MsgTypeAuthenticationFailure:
		gmmMessage.AuthenticationFailure = buildAuthenticationFailure(msg.AuthenticationFailure)
		return gmmMessage
	case nas.MsgTypeAuthenticationReject:
		gmmMessage.AuthenticationReject = buildAuthenticationReject(msg.AuthenticationReject)
		return gmmMessage
	default:
		logger.EllaLog.Warn("GMM message type not fully implemented", zap.String("message_type", gmmMessage.GmmHeader.MessageType))
		return gmmMessage
	}
}

func buildAuthenticationReject(msg *nasMessage.AuthenticationReject) *AuthenticationReject {
	if msg == nil {
		return nil
	}

	authReject := &AuthenticationReject{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		AuthenticationRejectMessageIdentity: nas.MessageName(msg.AuthenticationRejectMessageIdentity.Octet),
	}

	if msg.EAPMessage != nil {
		logger.EllaLog.Warn("EAPMessage not yet implemented")
	}

	return authReject
}

func buildAuthenticationFailure(msg *nasMessage.AuthenticationFailure) *AuthenticationFailure {
	if msg == nil {
		return nil
	}

	authFailure := &AuthenticationFailure{
		ExtendedProtocolDiscriminator:        msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType:  msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		AuthenticationFailureMessageIdentity: nas.MessageName(msg.AuthenticationFailureMessageIdentity.Octet),
		Cause5GMM:                            nasMessage.Cause5GMMToString(msg.Cause5GMM.GetCauseValue()),
	}

	if msg.AuthenticationFailureParameter != nil {
		logger.EllaLog.Warn("AuthenticationFailureParameter not yet implemented")
	}

	return authFailure
}

func buildAuthenticationRequest(msg *nasMessage.AuthenticationRequest) *AuthenticationRequest {
	if msg == nil {
		return nil
	}

	authenticationRequest := &AuthenticationRequest{
		ExtendedProtocolDiscriminator:        msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType:  msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		AuthenticationRequestMessageIdentity: nas.MessageName(msg.AuthenticationRequestMessageIdentity.Octet),
		SpareHalfOctetAndNgksi:               msg.SpareHalfOctetAndNgksi.Octet,
		ABBA:                                 msg.ABBA.GetABBAContents(),
	}

	if msg.AuthenticationParameterRAND != nil {
		authenticationRequest.AuthenticationParameterRAND = msg.AuthenticationParameterRAND.GetRANDValue()
	}

	if msg.AuthenticationParameterAUTN != nil {
		authenticationRequest.AuthenticationParameterAUTN = msg.AuthenticationParameterAUTN.GetAUTN()
	}

	if msg.EAPMessage != nil {
		logger.EllaLog.Warn("EAPMessage not yet implemented")
	}

	return authenticationRequest
}

func buildRegistrationRequest(msg *nasMessage.RegistrationRequest) *RegistrationRequest {
	if msg == nil {
		return nil
	}

	registrationRequest := &RegistrationRequest{
		MobileIdentity5GS:                  getMobileIdentity5GS(msg.MobileIdentity5GS),
		ExtendedProtocolDiscriminator:      msg.ExtendedProtocolDiscriminator.Octet,
		NgksiAndRegistrationType5GS:        msg.NgksiAndRegistrationType5GS.Octet,
		RegistrationRequestMessageIdentity: nas.MessageName(msg.RegistrationRequestMessageIdentity.Octet),
	}

	if msg.NoncurrentNativeNASKeySetIdentifier != nil {
		logger.EllaLog.Warn("NoncurrentNativeNASKeySetIdentifier not yet implemented")
	}

	if msg.Capability5GMM != nil {
		logger.EllaLog.Warn("Capability5GMM not yet implemented")
	}

	if msg.UESecurityCapability != nil {
		registrationRequest.UESecurityCapability = buildUESecurityCapability(*msg.UESecurityCapability)
	}

	if msg.RequestedNSSAI != nil {
		logger.EllaLog.Warn("RequestedNSSAI not yet implemented")
	}

	if msg.LastVisitedRegisteredTAI != nil {
		logger.EllaLog.Warn("LastVisitedRegisteredTAI not yet implemented")
	}

	if msg.S1UENetworkCapability != nil {
		logger.EllaLog.Warn("S1UENetworkCapability not yet implemented")
	}

	if msg.UplinkDataStatus != nil {
		logger.EllaLog.Warn("UplinkDataStatus not yet implemented")
	}

	if msg.PDUSessionStatus != nil {
		logger.EllaLog.Warn("PDUSessionStatus not yet implemented")
	}

	if msg.MICOIndication != nil {
		logger.EllaLog.Warn("MICOIndication not yet implemented")
	}

	if msg.UEStatus != nil {
		logger.EllaLog.Warn("UEStatus not yet implemented")
	}

	if msg.AdditionalGUTI != nil {
		logger.EllaLog.Warn("AdditionalGUTI not yet implemented")
	}

	if msg.AllowedPDUSessionStatus != nil {
		logger.EllaLog.Warn("AllowedPDUSessionStatus not yet implemented")
	}

	if msg.UesUsageSetting != nil {
		logger.EllaLog.Warn("UesUsageSetting not yet implemented")
	}

	if msg.RequestedDRXParameters != nil {
		logger.EllaLog.Warn("RequestedDRXParameters not yet implemented")
	}

	if msg.EPSNASMessageContainer != nil {
		logger.EllaLog.Warn("EPSNASMessageContainer not yet implemented")
	}

	if msg.LADNIndication != nil {
		logger.EllaLog.Warn("LADNIndication not yet implemented")
	}

	if msg.PayloadContainer != nil {
		logger.EllaLog.Warn("PayloadContainer not yet implemented")
	}

	if msg.NetworkSlicingIndication != nil {
		logger.EllaLog.Warn("NetworkSlicingIndication not yet implemented")
	}

	if msg.UpdateType5GS != nil {
		logger.EllaLog.Warn("UpdateType5GS not yet implemented")
	}

	if msg.NASMessageContainer != nil {
		logger.EllaLog.Warn("NASMessageContainer not yet implemented")
	}

	return registrationRequest
}

func buildUESecurityCapability(ueSecurityCapability nasType.UESecurityCapability) *UESecurityCapability {
	ueSecCap := &UESecurityCapability{
		IntegrityAlgorithm: IntegrityAlgorithm{},
		CipheringAlgorithm: CipheringAlgorithm{},
	}

	if ueSecurityCapability.GetIA0_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.EEA0_5G = true
	}

	if ueSecurityCapability.GetIA1_128_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.EEA1_128_5G = true
	}

	if ueSecurityCapability.GetIA2_128_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.EEA2_128_5G = true
	}

	if ueSecurityCapability.GetIA3_128_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.EEA3_128_5G = true
	}

	if ueSecurityCapability.GetEA0_5G() == 1 {
		ueSecCap.CipheringAlgorithm.EIA0_5G = true
	}

	if ueSecurityCapability.GetEA1_128_5G() == 1 {
		ueSecCap.CipheringAlgorithm.EIA1_128_5G = true
	}

	if ueSecurityCapability.GetEA2_128_5G() == 1 {
		ueSecCap.CipheringAlgorithm.EIA2_128_5G = true
	}

	if ueSecurityCapability.GetEA3_128_5G() == 1 {
		ueSecCap.CipheringAlgorithm.EIA3_128_5G = true
	}

	return ueSecCap
}

func getMobileIdentity5GS(mobileIdentity5GS nasType.MobileIdentity5GS) MobileIdentity5GS {
	mobileIdentity5GSContents := mobileIdentity5GS.GetMobileIdentity5GSContents()
	identityTypeUsedForRegistration := nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0])
	switch identityTypeUsedForRegistration {
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		return MobileIdentity5GS{
			Identity: "No Identity",
		}
	case nasMessage.MobileIdentity5GSTypeSuci:
		suci, plmnID := nasConvert.SuciToString(mobileIdentity5GSContents)
		plmnIDModel := PlmnIDStringToModels(plmnID)
		return MobileIdentity5GS{
			Identity: "SUCI",
			SUCI:     &suci,
			PLMNID:   &plmnIDModel,
		}
	case nasMessage.MobileIdentity5GSType5gGuti:
		_, guti := util.GutiToString(mobileIdentity5GSContents)
		return MobileIdentity5GS{
			GUTI:     &guti,
			Identity: "5G-GUTI",
		}
	case nasMessage.MobileIdentity5GSTypeImei:
		imei := nasConvert.PeiToString(mobileIdentity5GSContents)
		return MobileIdentity5GS{
			Identity: "IMEI",
			IMEI:     &imei,
		}
	case nasMessage.MobileIdentity5GSType5gSTmsi:
		sTmsi := hex.EncodeToString(mobileIdentity5GSContents[1:])
		return MobileIdentity5GS{
			STMSI:    &sTmsi,
			Identity: "5G-S-TMSI",
		}
	case nasMessage.MobileIdentity5GSTypeImeisv:
		imeisv := nasConvert.PeiToString(mobileIdentity5GSContents)
		return MobileIdentity5GS{
			Identity: "IMEISV",
			IMEISV:   &imeisv,
		}
	default:
		logger.EllaLog.Warn("MobileIdentity5GS type not fully implemented", zap.String("identity_type", fmt.Sprintf("%v", identityTypeUsedForRegistration)))
		return MobileIdentity5GS{
			Identity: "Unknown",
		}
	}
}

func PlmnIDStringToModels(plmnIDStr string) PLMNID {
	var plmnID PLMNID
	plmnID.Mcc = plmnIDStr[:3]
	plmnID.Mnc = plmnIDStr[3:]
	return plmnID
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

func decodeNAS(raw []byte) (*nas.Message, error) {
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
		return nil, fmt.Errorf("not yet implemented: cannot decode ciphered NAS message (IntegrityProtectedAndCiphered)")
	case nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext:
		return nil, fmt.Errorf("not yet implemented: cannot decode ciphered NAS message (IntegrityProtectedWithNew5gNasSecurityContext)")
	case nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext:
		return nil, fmt.Errorf("not yet implemented: cannot decode ciphered NAS message (IntegrityProtectedAndCipheredWithNew5gNasSecurityContext)")
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
