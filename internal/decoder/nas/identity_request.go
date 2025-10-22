package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/omec-project/nas/nasMessage"
)

type IdentityRequest struct {
	ExtendedProtocolDiscriminator       uint8                  `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                  `json:"spare_half_octet_and_security_header_type"`
	TypeOfIdentity                      utils.EnumField[uint8] `json:"type_of_identity"`
}

func buildIdentityRequest(msg *nasMessage.IdentityRequest) *IdentityRequest {
	if msg == nil {
		return nil
	}

	return &IdentityRequest{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		TypeOfIdentity:                      buildTypeOfIdentityEnum(msg.SpareHalfOctetAndIdentityType.GetTypeOfIdentity()),
	}
}
