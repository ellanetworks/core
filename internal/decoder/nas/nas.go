// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

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

	switch msg := msg.(type) {
	case *fgs.PDUSessionEstablishmentRequest:
		gsmMessage.PDUSessionEstablishmentRequest = buildPDUSessionEstablishmentRequest(msg)
	case *fgs.PDUSessionEstablishmentAccept:
		gsmMessage.PDUSessionEstablishmentAccept = buildPDUSessionEstablishmentAccept(msg)
	}

	return gsmMessage
}

func getGsmMessageType(messageType uint8) utils.EnumField {
	switch messageType {
	case 0xC1:
		return utils.MakeEnum(messageType, "PDUSessionEstablishmentRequest", false)
	case 0xC2:
		return utils.MakeEnum(messageType, "PDUSessionEstablishmentAccept", false)
	case 0xC3:
		return utils.MakeEnum(messageType, "PDUSessionEstablishmentReject", false)
	case 0xC5:
		return utils.MakeEnum(messageType, "PDUSessionAuthenticationCommand", false)
	case 0xC6:
		return utils.MakeEnum(messageType, "PDUSessionAuthenticationComplete", false)
	case 0xC7:
		return utils.MakeEnum(messageType, "PDUSessionAuthenticationResult", false)
	case 0xC9:
		return utils.MakeEnum(messageType, "PDUSessionModificationRequest", false)
	case 0xCA:
		return utils.MakeEnum(messageType, "PDUSessionModificationReject", false)
	case 0xCB:
		return utils.MakeEnum(messageType, "PDUSessionModificationCommand", false)
	case 0xCC:
		return utils.MakeEnum(messageType, "PDUSessionModificationComplete", false)
	case 0xCD:
		return utils.MakeEnum(messageType, "PDUSessionModificationCommandReject", false)
	case 0xD1:
		return utils.MakeEnum(messageType, "PDUSessionReleaseRequest", false)
	case 0xD2:
		return utils.MakeEnum(messageType, "PDUSessionReleaseReject", false)
	case 0xD3:
		return utils.MakeEnum(messageType, "PDUSessionReleaseCommand", false)
	case 0xD4:
		return utils.MakeEnum(messageType, "PDUSessionReleaseComplete", false)
	case 0xD6:
		return utils.MakeEnum(messageType, "Status5GSM", false)
	default:
		return utils.MakeEnum(messageType, "", true)
	}
}

func getGmmMessageType(messageType uint8) utils.EnumField {
	switch messageType {
	case 0x41:
		return utils.MakeEnum(messageType, "RegistrationRequest", false)
	case 0x42:
		return utils.MakeEnum(messageType, "RegistrationAccept", false)
	case 0x43:
		return utils.MakeEnum(messageType, "RegistrationComplete", false)
	case 0x44:
		return utils.MakeEnum(messageType, "RegistrationReject", false)
	case 0x45:
		return utils.MakeEnum(messageType, "DeregistrationRequestUEOriginatingDeregistration", false)
	case 0x46:
		return utils.MakeEnum(messageType, "DeregistrationAcceptUEOriginatingDeregistration", false)
	case 0x47:
		return utils.MakeEnum(messageType, "DeregistrationRequestUETerminatedDeregistration", false)
	case 0x48:
		return utils.MakeEnum(messageType, "DeregistrationAcceptUETerminatedDeregistration", false)
	case 0x4C:
		return utils.MakeEnum(messageType, "ServiceRequest", false)
	case 0x4D:
		return utils.MakeEnum(messageType, "ServiceReject", false)
	case 0x4E:
		return utils.MakeEnum(messageType, "ServiceAccept", false)
	case 0x54:
		return utils.MakeEnum(messageType, "ConfigurationUpdateCommand", false)
	case 0x55:
		return utils.MakeEnum(messageType, "ConfigurationUpdateComplete", false)
	case 0x56:
		return utils.MakeEnum(messageType, "AuthenticationRequest", false)
	case 0x57:
		return utils.MakeEnum(messageType, "AuthenticationResponse", false)
	case 0x58:
		return utils.MakeEnum(messageType, "AuthenticationReject", false)
	case 0x59:
		return utils.MakeEnum(messageType, "AuthenticationFailure", false)
	case 0x5A:
		return utils.MakeEnum(messageType, "AuthenticationResult", false)
	case 0x5B:
		return utils.MakeEnum(messageType, "IdentityRequest", false)
	case 0x5C:
		return utils.MakeEnum(messageType, "IdentityResponse", false)
	case 0x5D:
		return utils.MakeEnum(messageType, "SecurityModeCommand", false)
	case 0x5E:
		return utils.MakeEnum(messageType, "SecurityModeComplete", false)
	case 0x5F:
		return utils.MakeEnum(messageType, "SecurityModeReject", false)
	case 0x64:
		return utils.MakeEnum(messageType, "Status5GMM", false)
	case 0x65:
		return utils.MakeEnum(messageType, "Notification", false)
	case 0x66:
		return utils.MakeEnum(messageType, "NotificationResponse", false)
	case 0x67:
		return utils.MakeEnum(messageType, "ULNASTransport", false)
	case 0x68:
		return utils.MakeEnum(messageType, "DLNASTransport", false)
	default:
		return utils.MakeEnum(messageType, "", true)
	}
}

func epdToEnum(epd fgs.ProtocolDiscriminator) utils.EnumField {
	switch epd {
	case fgs.EPD5GMM:
		return utils.MakeEnum(uint8(epd), "5GSMobilityManagementMessage", false)
	case fgs.EPD5GSM:
		return utils.MakeEnum(uint8(epd), "5GSSessionManagementMessage", false)
	default:
		return utils.MakeEnum(uint8(epd), "", true)
	}
}

func securityHeaderTypeToEnum(sht fgs.SecurityHeaderType) utils.EnumField {
	v := uint8(sht)

	switch sht {
	case fgs.SHTPlain:
		return utils.MakeEnum(v, "Plain NAS", false)
	case fgs.SHTIntegrityProtected:
		return utils.MakeEnum(v, "Integrity Protected", false)
	case fgs.SHTIntegrityProtectedCiphered:
		return utils.MakeEnum(v, "Integrity Protected and Ciphered", false)
	case fgs.SHTIntegrityProtectedNewContext:
		return utils.MakeEnum(v, "Integrity Protected with New 5G NAS Security Context", false)
	case fgs.SHTIntegrityProtectedCipheredNewContext:
		return utils.MakeEnum(v, "Integrity Protected and Ciphered with New 5G NAS Security Context", false)
	default:
		return utils.MakeEnum(v, "", true)
	}
}
