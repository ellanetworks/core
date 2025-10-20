package nas

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
)

type RegistrationReject struct {
	ExtendedProtocolDiscriminator       uint8  `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8  `json:"spare_half_octet_and_security_header_type"`
	RegistrationRejectMessageIdentity   string `json:"registration_reject_message_identity"`
	Cause5GMM                           string `json:"cause_5gmm"`
}

func buildRegistrationReject(msg *nasMessage.RegistrationReject) *RegistrationReject {
	if msg == nil {
		return nil
	}
	regRej := &RegistrationReject{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		RegistrationRejectMessageIdentity:   nas.MessageName(msg.RegistrationRejectMessageIdentity.Octet),
		Cause5GMM:                           nasMessage.Cause5GMMToString(msg.Cause5GMM.Octet),
	}

	if msg.T3346Value != nil {
		logger.EllaLog.Warn("T3346Value in RegistrationReject is not implemented")
	}

	if msg.T3502Value != nil {
		logger.EllaLog.Warn("T3502Value in RegistrationReject is not implemented")
	}

	if msg.EAPMessage != nil {
		logger.EllaLog.Warn("EAPMessage in RegistrationReject is not implemented")
	}

	return regRej
}
