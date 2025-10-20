package nas

import (
	"github.com/omec-project/nas/nasMessage"
)

type RegistrationComplete struct {
	ExtendedProtocolDiscriminator       uint8   `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8   `json:"spare_half_octet_and_security_header_type"`
	GetSORContent                       []uint8 `json:"sor_transparent_container,omitempty"`
}

func buildRegistrationComplete(msg *nasMessage.RegistrationComplete) *RegistrationComplete {
	if msg == nil {
		return nil
	}

	regComplete := &RegistrationComplete{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
	}

	if msg.SORTransparentContainer != nil {
		regComplete.GetSORContent = msg.SORTransparentContainer.GetSORContent()
	}

	return regComplete
}
