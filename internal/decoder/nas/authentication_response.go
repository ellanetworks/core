package nas

import (
	"github.com/free5gc/nas/nasMessage"
)

type AuthenticationResponseParameter struct {
	ResStar [16]uint8 `json:"res_star"`
}

type AuthenticationResponse struct {
	ExtendedProtocolDiscriminator       uint8                            `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                            `json:"spare_half_octet_and_security_header_type"`
	AuthenticationResponseParameter     *AuthenticationResponseParameter `json:"authentication_response_parameter,omitempty"`
	EAPMessage                          []byte                           `json:"eap_message,omitempty"`
}

func buildAuthenticationResponse(msg *nasMessage.AuthenticationResponse) *AuthenticationResponse {
	if msg == nil {
		return nil
	}

	authResp := &AuthenticationResponse{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
	}

	if msg.AuthenticationResponseParameter != nil {
		authResp.AuthenticationResponseParameter = &AuthenticationResponseParameter{
			ResStar: msg.GetRES(),
		}
	}

	if msg.EAPMessage != nil {
		authResp.EAPMessage = msg.GetEAPMessage()
	}

	return authResp
}
