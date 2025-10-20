package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
)

type RegistrationReject struct {
	ExtendedProtocolDiscriminator       uint8                  `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                  `json:"spare_half_octet_and_security_header_type"`
	RegistrationRejectMessageIdentity   string                 `json:"registration_reject_message_identity"`
	Cause5GMM                           utils.EnumField[uint8] `json:"cause_5gmm"`

	T3346Value *UnsupportedIE `json:"t3346_value,omitempty"`
	T3502Value *UnsupportedIE `json:"t3502_value,omitempty"`
	EAPMessage *UnsupportedIE `json:"eap_message,omitempty"`
}

func buildRegistrationReject(msg *nasMessage.RegistrationReject) *RegistrationReject {
	if msg == nil {
		return nil
	}
	regRej := &RegistrationReject{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		RegistrationRejectMessageIdentity:   nas.MessageName(msg.RegistrationRejectMessageIdentity.Octet),
		Cause5GMM:                           cause5GMMToEnum(msg.Cause5GMM.Octet),
	}

	if msg.T3346Value != nil {
		regRej.T3346Value = makeUnsupportedIE()
	}

	if msg.T3502Value != nil {
		regRej.T3502Value = makeUnsupportedIE()
	}

	if msg.EAPMessage != nil {
		regRej.EAPMessage = makeUnsupportedIE()
	}

	return regRej
}
