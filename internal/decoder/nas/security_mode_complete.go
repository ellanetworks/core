package nas

import (
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
)

type SecurityModeComplete struct {
	ExtendedProtocolDiscriminator       uint8   `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8   `json:"spare_half_octet_and_security_header_type"`
	IMEISV                              *string `json:"imeisv,omitempty"`
	NASMessageContainer                 []byte  `json:"nas_message_container,omitempty"`
}

func buildSecurityModeComplete(msg *nasMessage.SecurityModeComplete) *SecurityModeComplete {
	if msg == nil {
		return nil
	}

	securityModeComplete := &SecurityModeComplete{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
	}

	if msg.IMEISV != nil {
		pei := nasConvert.PeiToString(msg.IMEISV.Octet[:])
		securityModeComplete.IMEISV = &pei
	}

	if msg.NASMessageContainer != nil {
		securityModeComplete.NASMessageContainer = msg.GetNASMessageContainerContents()
	}

	return securityModeComplete
}
