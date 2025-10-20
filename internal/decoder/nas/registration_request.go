package nas

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
	"go.uber.org/zap"
)

type MobileIdentity5GS struct {
	Identity string
	PLMNID   *PLMNID `json:"plmn_id,omitempty"`
	SUCI     *string `json:"suci,omitempty"`
	GUTI     *string `json:"guti,omitempty"`
	STMSI    *string `json:"s_tmsi,omitempty"`
	IMEI     *string `json:"imei,omitempty"`
	IMEISV   *string `json:"imeisv,omitempty"`
}

type RegistrationRequest struct {
	ExtendedProtocolDiscriminator       uint8                 `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                 `json:"spare_half_octet_and_security_header_type"`
	RegistrationRequestMessageIdentity  string                `json:"registration_request_message_identity"`
	NgksiAndRegistrationType5GS         uint8                 `json:"ngksi_and_registration_type_5gs"`
	MobileIdentity5GS                   MobileIdentity5GS     `json:"mobile_identity_5gs"`
	UESecurityCapability                *UESecurityCapability `json:"ue_security_capability,omitempty"`
}

func buildRegistrationRequest(msg *nasMessage.RegistrationRequest) *RegistrationRequest {
	if msg == nil {
		return nil
	}

	registrationRequest := &RegistrationRequest{
		MobileIdentity5GS:                  getMobileIdentity5GS(msg.MobileIdentity5GS),
		ExtendedProtocolDiscriminator:      msg.ExtendedProtocolDiscriminator.Octet,
		NgksiAndRegistrationType5GS:        msg.NgksiAndRegistrationType5GS.Octet,
		RegistrationRequestMessageIdentity: nas.MessageName(msg.RegistrationRequestMessageIdentity.Octet),
	}

	if msg.NoncurrentNativeNASKeySetIdentifier != nil {
		logger.EllaLog.Warn("NoncurrentNativeNASKeySetIdentifier not yet implemented")
	}

	if msg.Capability5GMM != nil {
		logger.EllaLog.Warn("Capability5GMM not yet implemented")
	}

	if msg.UESecurityCapability != nil {
		registrationRequest.UESecurityCapability = buildUESecurityCapability(*msg.UESecurityCapability)
	}

	if msg.RequestedNSSAI != nil {
		logger.EllaLog.Warn("RequestedNSSAI not yet implemented")
	}

	if msg.LastVisitedRegisteredTAI != nil {
		logger.EllaLog.Warn("LastVisitedRegisteredTAI not yet implemented")
	}

	if msg.S1UENetworkCapability != nil {
		logger.EllaLog.Warn("S1UENetworkCapability not yet implemented")
	}

	if msg.UplinkDataStatus != nil {
		logger.EllaLog.Warn("UplinkDataStatus not yet implemented")
	}

	if msg.PDUSessionStatus != nil {
		logger.EllaLog.Warn("PDUSessionStatus not yet implemented")
	}

	if msg.MICOIndication != nil {
		logger.EllaLog.Warn("MICOIndication not yet implemented")
	}

	if msg.UEStatus != nil {
		logger.EllaLog.Warn("UEStatus not yet implemented")
	}

	if msg.AdditionalGUTI != nil {
		logger.EllaLog.Warn("AdditionalGUTI not yet implemented")
	}

	if msg.AllowedPDUSessionStatus != nil {
		logger.EllaLog.Warn("AllowedPDUSessionStatus not yet implemented")
	}

	if msg.UesUsageSetting != nil {
		logger.EllaLog.Warn("UesUsageSetting not yet implemented")
	}

	if msg.RequestedDRXParameters != nil {
		logger.EllaLog.Warn("RequestedDRXParameters not yet implemented")
	}

	if msg.EPSNASMessageContainer != nil {
		logger.EllaLog.Warn("EPSNASMessageContainer not yet implemented")
	}

	if msg.LADNIndication != nil {
		logger.EllaLog.Warn("LADNIndication not yet implemented")
	}

	if msg.PayloadContainer != nil {
		logger.EllaLog.Warn("PayloadContainer not yet implemented")
	}

	if msg.NetworkSlicingIndication != nil {
		logger.EllaLog.Warn("NetworkSlicingIndication not yet implemented")
	}

	if msg.UpdateType5GS != nil {
		logger.EllaLog.Warn("UpdateType5GS not yet implemented")
	}

	if msg.NASMessageContainer != nil {
		logger.EllaLog.Warn("NASMessageContainer not yet implemented")
	}

	return registrationRequest
}

func getMobileIdentity5GS(mobileIdentity5GS nasType.MobileIdentity5GS) MobileIdentity5GS {
	mobileIdentity5GSContents := mobileIdentity5GS.GetMobileIdentity5GSContents()
	identityTypeUsedForRegistration := nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0])
	switch identityTypeUsedForRegistration {
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		return MobileIdentity5GS{
			Identity: "No Identity",
		}
	case nasMessage.MobileIdentity5GSTypeSuci:
		suci, plmnID := nasConvert.SuciToString(mobileIdentity5GSContents)
		plmnIDModel := plmnIDStringToModels(plmnID)
		return MobileIdentity5GS{
			Identity: "SUCI",
			SUCI:     &suci,
			PLMNID:   &plmnIDModel,
		}
	case nasMessage.MobileIdentity5GSType5gGuti:
		_, guti := util.GutiToString(mobileIdentity5GSContents)
		return MobileIdentity5GS{
			GUTI:     &guti,
			Identity: "5G-GUTI",
		}
	case nasMessage.MobileIdentity5GSTypeImei:
		imei := nasConvert.PeiToString(mobileIdentity5GSContents)
		return MobileIdentity5GS{
			Identity: "IMEI",
			IMEI:     &imei,
		}
	case nasMessage.MobileIdentity5GSType5gSTmsi:
		sTmsi := hex.EncodeToString(mobileIdentity5GSContents[1:])
		return MobileIdentity5GS{
			STMSI:    &sTmsi,
			Identity: "5G-S-TMSI",
		}
	case nasMessage.MobileIdentity5GSTypeImeisv:
		imeisv := nasConvert.PeiToString(mobileIdentity5GSContents)
		return MobileIdentity5GS{
			Identity: "IMEISV",
			IMEISV:   &imeisv,
		}
	default:
		logger.EllaLog.Warn("MobileIdentity5GS type not fully implemented", zap.String("identity_type", fmt.Sprintf("%v", identityTypeUsedForRegistration)))
		return MobileIdentity5GS{
			Identity: "Unknown",
		}
	}
}

func buildUESecurityCapability(ueSecurityCapability nasType.UESecurityCapability) *UESecurityCapability {
	ueSecCap := &UESecurityCapability{
		IntegrityAlgorithm: IntegrityAlgorithm{},
		CipheringAlgorithm: CipheringAlgorithm{},
	}

	if ueSecurityCapability.GetIA0_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.NIA0 = true
	}

	if ueSecurityCapability.GetIA1_128_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.NIA1 = true
	}

	if ueSecurityCapability.GetIA2_128_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.NIA2 = true
	}

	if ueSecurityCapability.GetIA3_128_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.NIA3 = true
	}

	if ueSecurityCapability.GetEA0_5G() == 1 {
		ueSecCap.CipheringAlgorithm.NEA0 = true
	}

	if ueSecurityCapability.GetEA1_128_5G() == 1 {
		ueSecCap.CipheringAlgorithm.NEA1 = true
	}

	if ueSecurityCapability.GetEA2_128_5G() == 1 {
		ueSecCap.CipheringAlgorithm.NEA2 = true
	}

	if ueSecurityCapability.GetEA3_128_5G() == 1 {
		ueSecCap.CipheringAlgorithm.NEA3 = true
	}

	return ueSecCap
}

func plmnIDStringToModels(plmnIDStr string) PLMNID {
	var plmnID PLMNID
	plmnID.Mcc = plmnIDStr[:3]
	plmnID.Mnc = plmnIDStr[3:]
	return plmnID
}
