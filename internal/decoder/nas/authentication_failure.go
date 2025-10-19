package nas

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
)

type AuthenticationFailure struct {
	ExtendedProtocolDiscriminator        uint8  `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType  uint8  `json:"spare_half_octet_and_security_header_type"`
	AuthenticationFailureMessageIdentity string `json:"authentication_failure_message_identity"`
	Cause5GMM                            string `json:"cause"`
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
