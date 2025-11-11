package nas

import (
	"github.com/free5gc/nas/nasMessage"
)

type AuthenticationRequest struct {
	ExtendedProtocolDiscriminator       uint8     `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8     `json:"spare_half_octet_and_security_header_type"`
	SpareHalfOctetAndNgksi              uint8     `json:"spare_half_octet_and_ngksi"`
	ABBA                                []uint8   `json:"abba"`
	AuthenticationParameterAUTN         [16]uint8 `json:"authentication_parameter_autn,omitempty"`
	AuthenticationParameterRAND         [16]uint8 `json:"authentication_parameter_rand,omitempty"`
	EAPMessage                          []byte    `json:"eap_message,omitempty"`
}

func buildAuthenticationRequest(msg *nasMessage.AuthenticationRequest) *AuthenticationRequest {
	if msg == nil {
		return nil
	}

	authenticationRequest := &AuthenticationRequest{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		SpareHalfOctetAndNgksi:              msg.SpareHalfOctetAndNgksi.Octet,
		ABBA:                                msg.ABBA.GetABBAContents(),
	}

	if msg.AuthenticationParameterRAND != nil {
		authenticationRequest.AuthenticationParameterRAND = msg.AuthenticationParameterRAND.GetRANDValue()
	}

	if msg.AuthenticationParameterAUTN != nil {
		authenticationRequest.AuthenticationParameterAUTN = msg.AuthenticationParameterAUTN.GetAUTN()
	}

	if msg.EAPMessage != nil {
		authenticationRequest.EAPMessage = msg.EAPMessage.GetEAPMessage()
	}

	return authenticationRequest
}
