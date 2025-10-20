package nas

import (
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
)

type AuthenticationReject struct {
	ExtendedProtocolDiscriminator       uint8  `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8  `json:"spare_half_octet_and_security_header_type"`
	AuthenticationRejectMessageIdentity string `json:"authentication_reject_message_identity"`
	EAPMessage                          []byte `json:"eap_message,omitempty"`
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
		authReject.EAPMessage = msg.EAPMessage.GetEAPMessage()
	}

	return authReject
}
