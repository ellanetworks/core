package nas

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
)

type MobileIdentity struct {
	Identity utils.EnumField[uint8] `json:"identity_type"`
	PLMNID   *PLMNID                `json:"plmn_id,omitempty"`
	SUCI     *string                `json:"suci,omitempty"`
	GUTI     *string                `json:"guti,omitempty"`
	STMSI    *string                `json:"s_tmsi,omitempty"`
	IMEI     *string                `json:"imei,omitempty"`
	IMEISV   *string                `json:"imeisv,omitempty"`
}

type IdentityResponse struct {
	ExtendedProtocolDiscriminator       uint8          `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8          `json:"spare_half_octet_and_security_header_type"`
	MobileIdentity                      MobileIdentity `json:"mobile_identity"`
}

func buildIdentityResponse(msg *nasMessage.IdentityResponse) *IdentityResponse {
	if msg == nil {
		return nil
	}

	return &IdentityResponse{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		MobileIdentity:                      buildMobileIdentity(msg.MobileIdentity),
	}
}

func buildMobileIdentity(mobileIdentity5GS nasType.MobileIdentity) MobileIdentity {
	mobileIdentity5GSContents := mobileIdentity5GS.GetMobileIdentityContents()
	identityTypeUsedForRegistration := nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0])
	switch identityTypeUsedForRegistration {
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		return MobileIdentity{
			Identity: utils.MakeEnum(identityTypeUsedForRegistration, "No Identity", false),
		}
	case nasMessage.MobileIdentity5GSTypeSuci:
		suci, plmnID := nasConvert.SuciToString(mobileIdentity5GSContents)
		plmnIDModel := plmnIDStringToModels(plmnID)
		return MobileIdentity{
			Identity: utils.MakeEnum(identityTypeUsedForRegistration, "SUCI", false),
			SUCI:     &suci,
			PLMNID:   &plmnIDModel,
		}
	case nasMessage.MobileIdentity5GSType5gGuti:
		_, guti := util.GutiToString(mobileIdentity5GSContents)
		return MobileIdentity{
			GUTI:     &guti,
			Identity: utils.MakeEnum(identityTypeUsedForRegistration, "5G-GUTI", false),
		}
	case nasMessage.MobileIdentity5GSTypeImei:
		imei := nasConvert.PeiToString(mobileIdentity5GSContents)
		return MobileIdentity{
			Identity: utils.MakeEnum(identityTypeUsedForRegistration, "IMEI", false),
			IMEI:     &imei,
		}
	case nasMessage.MobileIdentity5GSType5gSTmsi:
		sTmsi := hex.EncodeToString(mobileIdentity5GSContents[1:])
		return MobileIdentity{
			STMSI:    &sTmsi,
			Identity: utils.MakeEnum(identityTypeUsedForRegistration, "5G-S-TMSI", false),
		}
	case nasMessage.MobileIdentity5GSTypeImeisv:
		imeisv := nasConvert.PeiToString(mobileIdentity5GSContents)
		return MobileIdentity{
			Identity: utils.MakeEnum(identityTypeUsedForRegistration, "IMEISV", false),
			IMEISV:   &imeisv,
		}
	default:
		return MobileIdentity{
			Identity: utils.MakeEnum(identityTypeUsedForRegistration, "", true),
		}
	}
}
