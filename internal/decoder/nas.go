package decoder

import (
	"fmt"

	"github.com/omec-project/nas"
)

type SecurityHeader struct {
	ProtocolDiscriminator     uint8  `json:"protocol_discriminator"`
	SecurityHeaderType        string `json:"security_header_type"`
	MessageAuthenticationCode uint32 `json:"message_authentication_code,omitempty"`
	SequenceNumber            uint8  `json:"sequence_number"`
}

type GmmMessage struct {
}

type GsmMessage struct{}

type NASMessage struct {
	SecurityHeader SecurityHeader `json:"security_header"`
	GmmMessage     *GmmMessage    `json:"gmm_message"`
	GsmMessage     *GsmMessage    `json:"gsm_message"`
}

func DecodeNASMessage(raw []byte) (*NASMessage, error) {
	nasMsg := new(NASMessage)

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
		return nil, fmt.Errorf("not yet implemented: cannot decode ciphered NAS message") // TODO: Decrypt and decode
	case nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext:
		return nil, fmt.Errorf("not yet implemented: cannot decode ciphered NAS message") // TODO: Decrypt and decode
	case nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext:
		return nil, fmt.Errorf("not yet implemented: cannot decode ciphered NAS message") // TODO: Decrypt and decode
	default:
		return nil, fmt.Errorf("unsupported security header type: %d", msg.SecurityHeaderType)
	}

	nasMsg.SecurityHeader = buildSecurityHeader(msg)

	return nasMsg, nil
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
