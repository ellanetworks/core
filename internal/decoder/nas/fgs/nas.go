// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"encoding/binary"

	"github.com/ellanetworks/core/internal/decoder/utils"
	naslib "github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type GmmHeader struct {
	MessageType utils.EnumField `json:"message_type"`
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
	IdentityRequest        *IdentityRequest        `json:"identity_request,omitempty"`
	IdentityResponse       *IdentityResponse       `json:"identity_response,omitempty"`

	ConfigurationUpdateCommand         *ConfigurationUpdateCommand         `json:"configuration_update_command,omitempty"`
	ConfigurationUpdateComplete        *ConfigurationUpdateComplete        `json:"configuration_update_complete,omitempty"`
	DeregistrationRequestUEOriginating *DeregistrationRequestUEOriginating `json:"deregistration_request_ue_originating,omitempty"`
	DeregistrationAcceptUEOriginating  *DeregistrationAccept               `json:"deregistration_accept_ue_originating,omitempty"`
	DeregistrationRequestUETerminated  *DeregistrationRequestUETerminated  `json:"deregistration_request_ue_terminated,omitempty"`
	DeregistrationAcceptUETerminated   *DeregistrationAccept               `json:"deregistration_accept_ue_terminated,omitempty"`
	GMMStatus                          *GMMCauseOnly                       `json:"gmm_status,omitempty"`
	SecurityModeReject                 *GMMCauseOnly                       `json:"security_mode_reject,omitempty"`
	NotificationResponse               *NotificationResponse               `json:"notification_response,omitempty"`
}

type GsmHeader struct {
	MessageType  utils.EnumField `json:"message_type"`
	PDUSessionID uint8           `json:"pdu_session_id"`
	PTI          uint8           `json:"pti"`
}

type GsmMessage struct {
	GsmHeader GsmHeader `json:"gsm_header"`
	Error     string    `json:"error,omitempty"`

	PDUSessionEstablishmentRequest *PDUSessionEstablishmentRequest `json:"pdu_session_establishment_request,omitempty"`
	PDUSessionEstablishmentAccept  *PDUSessionEstablishmentAccept  `json:"pdu_session_establishment_accept,omitempty"`

	PDUSessionAuthenticationComplete *PDUSessionAuthenticationComplete `json:"pdu_session_authentication_complete,omitempty"`

	GSMStatus                           *GSMCauseOnly                   `json:"gsm_status,omitempty"`
	PDUSessionEstablishmentReject       *GSMCauseOnly                   `json:"pdu_session_establishment_reject,omitempty"`
	PDUSessionReleaseCommand            *GSMCauseOnly                   `json:"pdu_session_release_command,omitempty"`
	PDUSessionModificationCommandReject *GSMCauseOnly                   `json:"pdu_session_modification_command_reject,omitempty"`
	PDUSessionReleaseRequest            *GSMOptionalCause               `json:"pdu_session_release_request,omitempty"`
	PDUSessionReleaseComplete           *GSMOptionalCause               `json:"pdu_session_release_complete,omitempty"`
	PDUSessionModificationRequest       *PDUSessionModificationRequest  `json:"pdu_session_modification_request,omitempty"`
	PDUSessionModificationCommand       *PDUSessionModificationCommand  `json:"pdu_session_modification_command,omitempty"`
	PDUSessionModificationReject        *PDUSessionModificationReject   `json:"pdu_session_modification_reject,omitempty"`
	PDUSessionModificationComplete      *PDUSessionModificationComplete `json:"pdu_session_modification_complete,omitempty"`
}

type SecurityHeader struct {
	ProtocolDiscriminator     utils.EnumField `json:"protocol_discriminator"`
	SecurityHeaderType        utils.EnumField `json:"security_header_type"`
	MessageAuthenticationCode uint32          `json:"authentication_code,omitempty"`
	SequenceNumber            uint8           `json:"sequence_number"`
}

type NASMessage struct {
	SecurityHeader SecurityHeader `json:"security_header"`
	GmmMessage     *GmmMessage    `json:"gmm_message,omitempty"`
	GsmMessage     *GsmMessage    `json:"gsm_message,omitempty"`

	Encrypted bool   `json:"encrypted"`       // Indicates if the message was encrypted
	Error     string `json:"error,omitempty"` // Reserved field for decoding errors
}

func DecodeNASMessage(raw []byte) *NASMessage {
	// This diagnostic decoder runs on untrusted captured bytes; the lib accessors
	// hide the header octet placement and guard the length.
	epd, err := fgs.PeekProtocolDiscriminator(raw)
	if err != nil {
		return &NASMessage{Error: "NAS message too short"}
	}

	sht, err := fgs.PeekSecurityHeaderType(raw)
	if err != nil {
		return &NASMessage{Error: "NAS message too short"}
	}

	nasMsg := &NASMessage{
		SecurityHeader: SecurityHeader{
			SecurityHeaderType:    securityHeaderTypeToEnum(sht),
			ProtocolDiscriminator: epdToEnum(epd),
		},
	}

	if sht == fgs.SHTPlain {
		return decodePlainNAS(nasMsg, raw)
	}

	spm, err := fgs.ParseSecurityProtectedMessage(raw)
	if err != nil {
		// A reserved security header type names no protection the codec will frame
		// (TS 24.501 table 9.3.1), so the payload cannot be read; anything else here
		// is a truncated wrapper.
		if nasMsg.SecurityHeader.SecurityHeaderType.Unknown {
			nasMsg.Encrypted = true
			return nasMsg
		}

		nasMsg.Error = "security-protected NAS message too short"

		return nasMsg
	}

	nasMsg.SecurityHeader.MessageAuthenticationCode = binary.BigEndian.Uint32(spm.MAC[:])
	nasMsg.SecurityHeader.SequenceNumber = spm.SequenceNumber

	switch sht {
	case fgs.SHTIntegrityProtected, fgs.SHTIntegrityProtectedNewContext:
		// Integrity-protected but NOT ciphered — the inner NAS is plaintext.
		return decodePlainNAS(nasMsg, spm.UnverifiedPayload)
	case fgs.SHTIntegrityProtectedCiphered, fgs.SHTIntegrityProtectedCipheredNewContext:
		// The security header does not name the cipher, so decode the payload and keep
		// the result only if it resolves to a recognized message: under null ciphering
		// (NEA0) the payload is plaintext, whereas a real cipher does not frame as a
		// known message and the PDU is reported encrypted. Symmetric with the 4G decoder.
		decoded := decodePlainNAS(nasMsg, spm.UnverifiedPayload)
		if looksLikePlaintext(decoded) {
			return decoded
		}

		nasMsg.GmmMessage = nil
		nasMsg.GsmMessage = nil
		nasMsg.Error = ""
		nasMsg.Encrypted = true

		return nasMsg
	default:
		// Reserved security header type — unknown format, cannot decode.
		nasMsg.Encrypted = true
		return nasMsg
	}
}

// looksLikePlaintext reports whether a decode attempt resolved to a recognized
// NAS message, distinguishing a null-ciphered (NEA0) plaintext payload from a
// real ciphertext that only happens to frame.
func looksLikePlaintext(m *NASMessage) bool {
	if m.GmmMessage != nil && !m.GmmMessage.GmmHeader.MessageType.Unknown {
		return true
	}

	if m.GsmMessage != nil && !m.GsmMessage.GsmHeader.MessageType.Unknown {
		return true
	}

	return false
}

func decodePlainNAS(nasMsg *NASMessage, raw []byte) *NASMessage {
	if len(raw) < 1 {
		nasMsg.Error = "failed to decode NAS message: message too short"
		return nasMsg
	}

	switch fgs.ProtocolDiscriminator(raw[0]) {
	case fgs.EPD5GMM:
		gmm := buildGmmMessage(raw)
		if gmm == nil {
			nasMsg.Error = "failed to decode NAS message: 5GMM message too short"
			return nasMsg
		}

		nasMsg.GmmMessage = gmm
	case fgs.EPD5GSM:
		gsm := buildGsmMessage(raw)
		if gsm == nil {
			nasMsg.Error = "failed to decode NAS message: 5GSM message too short"
			return nasMsg
		}

		nasMsg.GsmMessage = gsm
	default:
		nasMsg.Error = "failed to decode NAS message: unknown protocol discriminator"
	}

	return nasMsg
}

// buildGmmMessage dispatches on the 5GMM message type (raw[2], after the EPD and
// security-header octets). It always renders the header (with the message type,
// flagged Unknown when unrecognized) and decodes the body of a recognized type;
// an unrecognized or undecoded type yields the header alone, matching the 4G
// decoder. It returns nil only on a truncated header, so the caller reports a
// decode failure (and the ciphered path marks the PDU encrypted).
func buildGmmMessage(raw []byte) *GmmMessage {
	if len(raw) < 3 {
		return nil
	}

	gmmMessage := &GmmMessage{GmmHeader: GmmHeader{MessageType: getGmmMessageType(raw[2])}}

	// A message this decoder cannot decode still renders its header, so a capture
	// shows what arrived.
	msg, err := fgs.ParseMessage(raw)
	if err != nil && !naslib.SoftOnly(err) {
		return gmmMessage
	}

	switch msg := msg.(type) {
	case *fgs.RegistrationRequest:
		gmmMessage.RegistrationRequest = buildRegistrationRequest(msg)
	case *fgs.RegistrationAccept:
		gmmMessage.RegistrationAccept = buildRegistrationAccept(msg)
	case *fgs.RegistrationReject:
		gmmMessage.RegistrationReject = buildRegistrationReject(msg)
	case *fgs.RegistrationComplete:
		gmmMessage.RegistrationComplete = buildRegistrationComplete(msg)
	case *fgs.AuthenticationRequest:
		gmmMessage.AuthenticationRequest = buildAuthenticationRequest(msg)
	case *fgs.AuthenticationFailure:
		gmmMessage.AuthenticationFailure = buildAuthenticationFailure(msg)
	case *fgs.AuthenticationReject:
		gmmMessage.AuthenticationReject = buildAuthenticationReject(msg)
	case *fgs.AuthenticationResponse:
		gmmMessage.AuthenticationResponse = buildAuthenticationResponse(msg)
	case *fgs.ULNASTransport:
		gmmMessage.ULNASTransport = buildULNASTransport(msg)
	case *fgs.DLNASTransport:
		gmmMessage.DLNASTransport = buildDLNASTransport(msg)
	case *fgs.SecurityModeCommand:
		gmmMessage.SecurityModeCommand = buildSecurityModeCommand(msg)
	case *fgs.SecurityModeComplete:
		gmmMessage.SecurityModeComplete = buildSecurityModeComplete(msg)
	case *fgs.ServiceRequest:
		gmmMessage.ServiceRequest = buildServiceRequest(msg)
	case *fgs.ServiceAccept:
		gmmMessage.ServiceAccept = buildServiceAccept(msg)
	case *fgs.ServiceReject:
		gmmMessage.ServiceReject = buildServiceReject(msg)
	case *fgs.IdentityRequest:
		gmmMessage.IdentityRequest = buildIdentityRequest(msg)
	case *fgs.IdentityResponse:
		gmmMessage.IdentityResponse = buildIdentityResponse(msg)
	case *fgs.ConfigurationUpdateCommand:
		gmmMessage.ConfigurationUpdateCommand = buildConfigurationUpdateCommand(msg)
	case *fgs.ConfigurationUpdateComplete:
		gmmMessage.ConfigurationUpdateComplete = buildConfigurationUpdateComplete(msg)
	case *fgs.DeregistrationRequestUEOriginating:
		gmmMessage.DeregistrationRequestUEOriginating = buildDeregistrationRequestUEOriginating(msg)
	case *fgs.DeregistrationAcceptUEOriginating:
		gmmMessage.DeregistrationAcceptUEOriginating = buildDeregistrationAcceptUEOriginating(msg)
	case *fgs.DeregistrationRequestUETerminated:
		gmmMessage.DeregistrationRequestUETerminated = buildDeregistrationRequestUETerminated(msg)
	case *fgs.DeregistrationAcceptUETerminated:
		gmmMessage.DeregistrationAcceptUETerminated = buildDeregistrationAcceptUETerminated(msg)
	case *fgs.GMMStatus:
		gmmMessage.GMMStatus = gmmCauseOnly(msg.Cause, msg.Unrecognized)
	case *fgs.SecurityModeReject:
		gmmMessage.SecurityModeReject = gmmCauseOnly(msg.Cause, msg.Unrecognized)
	case *fgs.NotificationResponse:
		gmmMessage.NotificationResponse = buildNotificationResponse(msg)
	}

	return gmmMessage
}

// buildGsmMessage dispatches on the 5GSM message type (raw[3], after the EPD, PDU
// session identity and PTI octets).
func buildGsmMessage(raw []byte) *GsmMessage {
	if len(raw) < 4 {
		return nil
	}

	messageType := raw[3]

	// Plain 5GSM header (TS 24.007 §11.2.3.1a): EPD, PDU session identity, PTI,
	// message type. The header content lives here, mirroring the 4G esm_header.
	gsmMessage := &GsmMessage{GsmHeader: GsmHeader{
		MessageType:  getGsmMessageType(messageType),
		PDUSessionID: raw[1],
		PTI:          raw[2],
	}}

	msg, err := fgs.ParseMessage(raw)
	if err != nil && !naslib.SoftOnly(err) {
		return gsmMessage
	}

	// The header octets above are read straight off the wire so a message the
	// decoder cannot build still renders one. Where the codec did parse the
	// message, its values are authoritative.
	switch msg := msg.(type) {
	case *fgs.PDUSessionEstablishmentRequest:
		gsmMessage.GsmHeader.PDUSessionID = uint8(msg.PDUSessionID)
		gsmMessage.GsmHeader.PTI = uint8(msg.PTI)
		gsmMessage.PDUSessionEstablishmentRequest = buildPDUSessionEstablishmentRequest(msg)
	case *fgs.PDUSessionAuthenticationComplete:
		gsmMessage.GsmHeader.PDUSessionID = uint8(msg.PDUSessionID)
		gsmMessage.GsmHeader.PTI = uint8(msg.PTI)
		gsmMessage.PDUSessionAuthenticationComplete = buildPDUSessionAuthenticationComplete(msg)
	case *fgs.GSMStatus:
		gsmMessage.GsmHeader.PDUSessionID = uint8(msg.PDUSessionID)
		gsmMessage.GsmHeader.PTI = uint8(msg.PTI)
		gsmMessage.GSMStatus = gsmCauseOnly(msg.Cause, msg.Unrecognized)
	case *fgs.PDUSessionEstablishmentReject:
		gsmMessage.GsmHeader.PDUSessionID = uint8(msg.PDUSessionID)
		gsmMessage.GsmHeader.PTI = uint8(msg.PTI)
		gsmMessage.PDUSessionEstablishmentReject = gsmCauseOnly(msg.Cause, msg.Unrecognized)
	case *fgs.PDUSessionReleaseCommand:
		gsmMessage.GsmHeader.PDUSessionID = uint8(msg.PDUSessionID)
		gsmMessage.GsmHeader.PTI = uint8(msg.PTI)
		gsmMessage.PDUSessionReleaseCommand = gsmCauseOnly(msg.Cause, msg.Unrecognized)
	case *fgs.PDUSessionModificationCommandReject:
		gsmMessage.GsmHeader.PDUSessionID = uint8(msg.PDUSessionID)
		gsmMessage.GsmHeader.PTI = uint8(msg.PTI)
		gsmMessage.PDUSessionModificationCommandReject = gsmCauseOnly(msg.Cause, msg.Unrecognized)
	case *fgs.PDUSessionReleaseRequest:
		gsmMessage.GsmHeader.PDUSessionID = uint8(msg.PDUSessionID)
		gsmMessage.GsmHeader.PTI = uint8(msg.PTI)
		gsmMessage.PDUSessionReleaseRequest = gsmOptionalCause(msg.Cause, msg.Unrecognized)
	case *fgs.PDUSessionReleaseComplete:
		gsmMessage.GsmHeader.PDUSessionID = uint8(msg.PDUSessionID)
		gsmMessage.GsmHeader.PTI = uint8(msg.PTI)
		gsmMessage.PDUSessionReleaseComplete = gsmOptionalCause(msg.Cause, msg.Unrecognized)
	case *fgs.PDUSessionModificationRequest:
		gsmMessage.GsmHeader.PDUSessionID = uint8(msg.PDUSessionID)
		gsmMessage.GsmHeader.PTI = uint8(msg.PTI)
		gsmMessage.PDUSessionModificationRequest = buildPDUSessionModificationRequest(msg)
	case *fgs.PDUSessionModificationCommand:
		gsmMessage.GsmHeader.PDUSessionID = uint8(msg.PDUSessionID)
		gsmMessage.GsmHeader.PTI = uint8(msg.PTI)
		gsmMessage.PDUSessionModificationCommand = buildPDUSessionModificationCommand(msg)
	case *fgs.PDUSessionModificationReject:
		gsmMessage.GsmHeader.PDUSessionID = uint8(msg.PDUSessionID)
		gsmMessage.GsmHeader.PTI = uint8(msg.PTI)
		gsmMessage.PDUSessionModificationReject = buildPDUSessionModificationReject(msg)
	case *fgs.PDUSessionModificationComplete:
		gsmMessage.GsmHeader.PDUSessionID = uint8(msg.PDUSessionID)
		gsmMessage.GsmHeader.PTI = uint8(msg.PTI)
		gsmMessage.PDUSessionModificationComplete = buildPDUSessionModificationComplete(msg)
	case *fgs.PDUSessionEstablishmentAccept:
		gsmMessage.GsmHeader.PDUSessionID = uint8(msg.PDUSessionID)
		gsmMessage.GsmHeader.PTI = uint8(msg.PTI)
		gsmMessage.PDUSessionEstablishmentAccept = buildPDUSessionEstablishmentAccept(msg)
	}

	return gsmMessage
}

func getGsmMessageType(messageType uint8) utils.EnumField {
	return utils.NamedEnum(messageType, fgs.GSMMessageType(messageType).Name())
}

func getGmmMessageType(messageType uint8) utils.EnumField {
	return utils.NamedEnum(messageType, fgs.MessageType(messageType).Name())
}

func epdToEnum(epd fgs.ProtocolDiscriminator) utils.EnumField {
	return utils.NamedEnum(uint8(epd), epd.Name())
}

func securityHeaderTypeToEnum(sht fgs.SecurityHeaderType) utils.EnumField {
	return utils.NamedEnum(uint8(sht), sht.Name())
}
