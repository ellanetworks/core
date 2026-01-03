package nas

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

type MobileIdentity5GS struct {
	Identity utils.EnumField[uint8] `json:"identity_type"`
	PLMNID   *PLMNID                `json:"plmn_id,omitempty"`
	SUCI     *string                `json:"suci,omitempty"`
	GUTI     *string                `json:"guti,omitempty"`
	STMSI    *string                `json:"s_tmsi,omitempty"`
	IMEI     *string                `json:"imei,omitempty"`
	IMEISV   *string                `json:"imeisv,omitempty"`
}

type RegistrationRequest struct {
	ExtendedProtocolDiscriminator       uint8                  `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                  `json:"spare_half_octet_and_security_header_type"`
	NasKeySetIdentifier                 uint8                  `json:"nas_key_set_identifier,omitempty"`
	RegistrationType5GS                 utils.EnumField[uint8] `json:"registration_type_5gs"`
	MobileIdentity5GS                   MobileIdentity5GS      `json:"mobile_identity_5gs"`
	UESecurityCapability                *UESecurityCapability  `json:"ue_security_capability,omitempty"`
	NASMessageContainer                 []byte                 `json:"nas_message_container,omitempty"`

	NoncurrentNativeNASKeySetIdentifier *UnsupportedIE `json:"noncurrent_native_nas_key_set_identifier,omitempty"`
	Capability5GMM                      *UnsupportedIE `json:"capability_5gmm,omitempty"`
	RequestedNSSAI                      *UnsupportedIE `json:"requested_nssai,omitempty"`
	LastVisitedRegisteredTAI            *UnsupportedIE `json:"last_visited_registered_tai,omitempty"`
	S1UENetworkCapability               *UnsupportedIE `json:"s1_ue_network_capability,omitempty"`
	UplinkDataStatus                    *UnsupportedIE `json:"uplink_data_status,omitempty"`
	PDUSessionStatus                    *UnsupportedIE `json:"pdu_session_status,omitempty"`
	MICOIndication                      *UnsupportedIE `json:"mico_indication,omitempty"`
	UEStatus                            *UnsupportedIE `json:"ue_status,omitempty"`
	AdditionalGUTI                      *UnsupportedIE `json:"additional_guti,omitempty"`
	AllowedPDUSessionStatus             *UnsupportedIE `json:"allowed_pdu_session_status,omitempty"`
	UesUsageSetting                     *UnsupportedIE `json:"ues_usage_setting,omitempty"`
	RequestedDRXParameters              *UnsupportedIE `json:"requested_drx_parameters,omitempty"`
	EPSNASMessageContainer              *UnsupportedIE `json:"eps_nas_message_container,omitempty"`
	LADNIndication                      *UnsupportedIE `json:"ladn_indication,omitempty"`
	PayloadContainer                    *UnsupportedIE `json:"payload_container,omitempty"`
	NetworkSlicingIndication            *UnsupportedIE `json:"network_slicing_indication,omitempty"`
	UpdateType5GS                       *UnsupportedIE `json:"update_type_5gs,omitempty"`
}

func buildRegistrationRequest(msg *nasMessage.RegistrationRequest) *RegistrationRequest {
	if msg == nil {
		return nil
	}

	registrationRequest := &RegistrationRequest{
		MobileIdentity5GS:             getMobileIdentity5GS(msg.MobileIdentity5GS),
		ExtendedProtocolDiscriminator: msg.ExtendedProtocolDiscriminator.Octet,
	}

	ksi, regType := buildNgksiAndRegistrationType5GS(msg.NgksiAndRegistrationType5GS)
	registrationRequest.NasKeySetIdentifier = ksi
	registrationRequest.RegistrationType5GS = regType

	if msg.NoncurrentNativeNASKeySetIdentifier != nil {
		registrationRequest.NoncurrentNativeNASKeySetIdentifier = makeUnsupportedIE()
	}

	if msg.Capability5GMM != nil {
		registrationRequest.Capability5GMM = makeUnsupportedIE()
	}

	if msg.UESecurityCapability != nil {
		registrationRequest.UESecurityCapability = buildUESecurityCapability(*msg.UESecurityCapability)
	}

	if msg.RequestedNSSAI != nil {
		registrationRequest.RequestedNSSAI = makeUnsupportedIE()
	}

	if msg.LastVisitedRegisteredTAI != nil {
		registrationRequest.LastVisitedRegisteredTAI = makeUnsupportedIE()
	}

	if msg.S1UENetworkCapability != nil {
		registrationRequest.S1UENetworkCapability = makeUnsupportedIE()
	}

	if msg.UplinkDataStatus != nil {
		registrationRequest.UplinkDataStatus = makeUnsupportedIE()
	}

	if msg.PDUSessionStatus != nil {
		registrationRequest.PDUSessionStatus = makeUnsupportedIE()
	}

	if msg.MICOIndication != nil {
		registrationRequest.MICOIndication = makeUnsupportedIE()
	}

	if msg.UEStatus != nil {
		registrationRequest.UEStatus = makeUnsupportedIE()
	}

	if msg.AdditionalGUTI != nil {
		registrationRequest.AdditionalGUTI = makeUnsupportedIE()
	}

	if msg.AllowedPDUSessionStatus != nil {
		registrationRequest.AllowedPDUSessionStatus = makeUnsupportedIE()
	}

	if msg.UesUsageSetting != nil {
		registrationRequest.UesUsageSetting = makeUnsupportedIE()
	}

	if msg.RequestedDRXParameters != nil {
		registrationRequest.RequestedDRXParameters = makeUnsupportedIE()
	}

	if msg.EPSNASMessageContainer != nil {
		registrationRequest.EPSNASMessageContainer = makeUnsupportedIE()
	}

	if msg.LADNIndication != nil {
		registrationRequest.LADNIndication = makeUnsupportedIE()
	}

	if msg.PayloadContainer != nil {
		registrationRequest.PayloadContainer = makeUnsupportedIE()
	}

	if msg.NetworkSlicingIndication != nil {
		registrationRequest.NetworkSlicingIndication = makeUnsupportedIE()
	}

	if msg.UpdateType5GS != nil {
		registrationRequest.UpdateType5GS = makeUnsupportedIE()
	}

	if msg.NASMessageContainer != nil {
		registrationRequest.NASMessageContainer = msg.GetNASMessageContainerContents()
	}

	return registrationRequest
}

func buildNgksiAndRegistrationType5GS(ngksiAndRegType nasType.NgksiAndRegistrationType5GS) (uint8, utils.EnumField[uint8]) {
	regTypeUint8 := ngksiAndRegType.GetRegistrationType5GS()
	ksi := ngksiAndRegType.GetNasKeySetIdentifiler()

	return ksi, getRegistrationType5GSName(regTypeUint8)
}

func getRegistrationType5GSName(regType5Gs uint8) utils.EnumField[uint8] {
	switch regType5Gs {
	case nasMessage.RegistrationType5GSInitialRegistration:
		return utils.MakeEnum(regType5Gs, "Initial Registration", false)
	case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
		return utils.MakeEnum(regType5Gs, "Mobility Registration Updating", false)
	case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
		return utils.MakeEnum(regType5Gs, "Periodic Registration Updating", false)
	case nasMessage.RegistrationType5GSEmergencyRegistration:
		return utils.MakeEnum(regType5Gs, "Emergency Registration", false)
	case nasMessage.RegistrationType5GSReserved:
		return utils.MakeEnum(regType5Gs, "Reserved", false)
	default:
		return utils.MakeEnum(regType5Gs, "", true)
	}
}

func getMobileIdentity5GS(mobileIdentity5GS nasType.MobileIdentity5GS) MobileIdentity5GS {
	mobileIdentity5GSContents := mobileIdentity5GS.GetMobileIdentity5GSContents()

	identityTypeUsedForRegistration := nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0])
	switch identityTypeUsedForRegistration {
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		return MobileIdentity5GS{
			Identity: utils.MakeEnum(identityTypeUsedForRegistration, "No Identity", false),
		}
	case nasMessage.MobileIdentity5GSTypeSuci:
		suci, plmnID := nasConvert.SuciToString(mobileIdentity5GSContents)
		plmnIDModel := plmnIDStringToModels(plmnID)

		return MobileIdentity5GS{
			Identity: utils.MakeEnum(identityTypeUsedForRegistration, "SUCI", false),
			SUCI:     &suci,
			PLMNID:   &plmnIDModel,
		}
	case nasMessage.MobileIdentity5GSType5gGuti:
		_, guti := nasConvert.GutiToString(mobileIdentity5GSContents)

		return MobileIdentity5GS{
			GUTI:     &guti,
			Identity: utils.MakeEnum(identityTypeUsedForRegistration, "5G-GUTI", false),
		}
	case nasMessage.MobileIdentity5GSTypeImei:
		imei := nasConvert.PeiToString(mobileIdentity5GSContents)

		return MobileIdentity5GS{
			Identity: utils.MakeEnum(identityTypeUsedForRegistration, "IMEI", false),
			IMEI:     &imei,
		}
	case nasMessage.MobileIdentity5GSType5gSTmsi:
		sTmsi := hex.EncodeToString(mobileIdentity5GSContents[1:])

		return MobileIdentity5GS{
			STMSI:    &sTmsi,
			Identity: utils.MakeEnum(identityTypeUsedForRegistration, "5G-S-TMSI", false),
		}
	case nasMessage.MobileIdentity5GSTypeImeisv:
		imeisv := nasConvert.PeiToString(mobileIdentity5GSContents)

		return MobileIdentity5GS{
			Identity: utils.MakeEnum(identityTypeUsedForRegistration, "IMEISV", false),
			IMEISV:   &imeisv,
		}
	default:
		return MobileIdentity5GS{
			Identity: utils.MakeEnum(identityTypeUsedForRegistration, "", true),
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
	return PLMNID{
		Mcc: plmnIDStr[:3],
		Mnc: plmnIDStr[3:],
	}
}
