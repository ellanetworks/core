package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
	"github.com/omec-project/nas/security"
)

type SelectedNASSecurityAlgorithms struct {
	Integrity utils.EnumField[uint8] `json:"integrity"`
	Ciphering utils.EnumField[uint8] `json:"ciphering"`
}

type IntegrityAlgorithm struct {
	NIA0 bool `json:"nia0"`
	NIA1 bool `json:"nia1"`
	NIA2 bool `json:"nia2"`
	NIA3 bool `json:"nia3"`
}

type CipheringAlgorithm struct {
	NEA0 bool `json:"nea0"`
	NEA1 bool `json:"nea1"`
	NEA2 bool `json:"nea2"`
	NEA3 bool `json:"nea3"`
}

type UESecurityCapability struct {
	IntegrityAlgorithm IntegrityAlgorithm `json:"integrity_algorithm"`
	CipheringAlgorithm CipheringAlgorithm `json:"ciphering_algorithm"`
}

type Additional5GSecurityInformation struct {
	RINMR uint8 `json:"rinmr"`
	HDP   uint8 `json:"hdp"`
}

type SecurityModeCommand struct {
	ExtendedProtocolDiscriminator       uint8                            `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                            `json:"spare_half_octet_and_security_header_type"`
	SelectedNASSecurityAlgorithms       SelectedNASSecurityAlgorithms    `json:"selected_nas_security_algorithms"`
	SpareHalfOctetAndNgksi              uint8                            `json:"spare_half_octet_and_ngksi"`
	ReplayedUESecurityCapabilities      UESecurityCapability             `json:"replayed_ue_security_capabilities"`
	IMEISVRequest                       *utils.EnumField[uint8]          `json:"imeisv_request,omitempty"`
	SelectedEPSNASSecurityAlgorithms    *utils.EnumField[uint8]          `json:"selected_eps_nas_security_algorithms,omitempty"`
	Additional5GSecurityInformation     *Additional5GSecurityInformation `json:"additional_5g_security_information,omitempty"`
	EAPMessage                          []byte                           `json:"eap_message,omitempty"`
	ABBA                                []uint8                          `json:"abba,omitempty"`

	ReplayedS1UESecurityCapabilities *UnsupportedIE `json:"replayed_s1_ue_security_capabilities,omitempty"`
}

func buildSecurityModeCommand(msg *nasMessage.SecurityModeCommand) *SecurityModeCommand {
	if msg == nil {
		return nil
	}

	securityModeCommand := &SecurityModeCommand{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		SelectedNASSecurityAlgorithms:       buildSelectedNASSecurityAlgorithms(msg.SelectedNASSecurityAlgorithms),
		SpareHalfOctetAndNgksi:              msg.SpareHalfOctetAndNgksi.Octet,
		ReplayedUESecurityCapabilities:      *buildReplayedUESecurityCapability(msg.ReplayedUESecurityCapabilities),
	}

	if msg.IMEISVRequest != nil {
		value := buildIMEISVRequest(*msg.IMEISVRequest)
		securityModeCommand.IMEISVRequest = &value
	}

	if msg.SelectedEPSNASSecurityAlgorithms != nil {
		algo := getIntegrity(msg.SelectedEPSNASSecurityAlgorithms.GetTypeOfIntegrityProtectionAlgorithm())
		securityModeCommand.SelectedEPSNASSecurityAlgorithms = &algo
	}

	if msg.Additional5GSecurityInformation != nil {
		securityModeCommand.Additional5GSecurityInformation = &Additional5GSecurityInformation{
			RINMR: msg.Additional5GSecurityInformation.GetRINMR(),
			HDP:   msg.Additional5GSecurityInformation.GetHDP(),
		}
	}

	if msg.EAPMessage != nil {
		securityModeCommand.EAPMessage = msg.EAPMessage.GetEAPMessage()
	}

	if msg.ABBA != nil {
		securityModeCommand.ABBA = msg.ABBA.GetABBAContents()
	}

	if msg.ReplayedS1UESecurityCapabilities != nil {
		securityModeCommand.ReplayedS1UESecurityCapabilities = makeUnsupportedIE()
	}

	return securityModeCommand
}

func buildReplayedUESecurityCapability(ueSecurityCapability nasType.ReplayedUESecurityCapabilities) *UESecurityCapability {
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

func buildIMEISVRequest(msg nasType.IMEISVRequest) utils.EnumField[uint8] {
	switch msg.GetIMEISVRequestValue() {
	case nasMessage.IMEISVNotRequested:
		return utils.MakeEnum(msg.GetIMEISVRequestValue(), "NotRequested", false)
	case nasMessage.IMEISVRequested:
		return utils.MakeEnum(msg.GetIMEISVRequestValue(), "Requested", false)
	default:
		return utils.MakeEnum(msg.GetIMEISVRequestValue(), "", true)
	}
}

func buildSelectedNASSecurityAlgorithms(msg nasType.SelectedNASSecurityAlgorithms) SelectedNASSecurityAlgorithms {
	return SelectedNASSecurityAlgorithms{
		Integrity: getIntegrity(msg.GetTypeOfIntegrityProtectionAlgorithm()),
		Ciphering: getCiphering(msg.GetTypeOfCipheringAlgorithm()),
	}
}

func getIntegrity(value uint8) utils.EnumField[uint8] {
	switch value {
	case security.AlgIntegrity128NIA0:
		return utils.MakeEnum(value, "NIA0", false)
	case security.AlgIntegrity128NIA1:
		return utils.MakeEnum(value, "NIA1", false)
	case security.AlgIntegrity128NIA2:
		return utils.MakeEnum(value, "NIA2", false)
	case security.AlgIntegrity128NIA3:
		return utils.MakeEnum(value, "NIA3", false)
	default:
		return utils.MakeEnum(value, "", true)
	}
}

func getCiphering(value uint8) utils.EnumField[uint8] {
	switch value {
	case security.AlgCiphering128NEA0:
		return utils.MakeEnum(value, "NEA0", false)
	case security.AlgCiphering128NEA1:
		return utils.MakeEnum(value, "NEA1", false)
	case security.AlgCiphering128NEA2:
		return utils.MakeEnum(value, "NEA2", false)
	case security.AlgCiphering128NEA3:
		return utils.MakeEnum(value, "NEA3", false)
	default:
		return utils.MakeEnum(value, "", true)
	}
}
