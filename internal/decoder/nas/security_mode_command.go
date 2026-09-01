// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

type SelectedNASSecurityAlgorithms struct {
	Integrity utils.EnumField `json:"integrity"`
	Ciphering utils.EnumField `json:"ciphering"`
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
	SelectedNASSecurityAlgorithms    SelectedNASSecurityAlgorithms     `json:"selected_nas_security_algorithms"`
	SpareHalfOctetAndNgksi           uint8                             `json:"spare_half_octet_and_ngksi"`
	ReplayedUESecurityCapabilities   UESecurityCapability              `json:"replayed_ue_security_capabilities"`
	IMEISVRequest                    *utils.EnumField                  `json:"imeisv_request,omitempty"`
	SelectedEPSNASSecurityAlgorithms *SelectedEPSNASSecurityAlgorithms `json:"selected_eps_nas_security_algorithms,omitempty"`
	Additional5GSecurityInformation  *Additional5GSecurityInformation  `json:"additional_5g_security_information,omitempty"`
	EAPMessage                       []byte                            `json:"eap_message,omitempty"`
	ABBA                             []uint8                           `json:"abba,omitempty"`

	ReplayedS1UESecurityCapabilities *S1UESecurityCapability `json:"replayed_s1_ue_security_capabilities,omitempty"`
}

func buildSecurityModeCommand(msg *fgs.SecurityModeCommand) *SecurityModeCommand {
	out := &SecurityModeCommand{
		SelectedNASSecurityAlgorithms: SelectedNASSecurityAlgorithms{
			Integrity: getIntegrity(uint8(msg.IntegrityAlgorithm)),
			Ciphering: getCiphering(uint8(msg.CipheringAlgorithm)),
		},
		SpareHalfOctetAndNgksi:         msg.NgKSI.HalfOctet(),
		ReplayedUESecurityCapabilities: *buildUESecurityCapability(msg.ReplayedUESecurityCapability),
		EAPMessage:                     msg.EAP,
		ABBA:                           msg.ABBA,
	}

	if msg.IMEISVRequested != nil {
		v := buildIMEISVRequest(uint8(*msg.IMEISVRequested))
		out.IMEISVRequest = &v
	}

	if algs := msg.SelectedEPSNASSecurityAlgorithms; algs != nil {
		out.SelectedEPSNASSecurityAlgorithms = &SelectedEPSNASSecurityAlgorithms{
			Ciphering: getEPSCiphering(uint8(algs.Ciphering)),
			Integrity: getEPSIntegrity(uint8(algs.Integrity)),
		}
	}

	if info := msg.AdditionalSecurityInformation; info != nil {
		out.Additional5GSecurityInformation = &Additional5GSecurityInformation{
			RINMR: b2u(info.RINMR),
			HDP:   b2u(info.HDP),
		}
	}

	if msg.ReplayedS1UESecurityCapability != nil {
		out.ReplayedS1UESecurityCapabilities = s1UESecurityCapability(msg.ReplayedS1UESecurityCapability)
	}

	return out
}

// SelectedEPSNASSecurityAlgorithms is the decoded Selected EPS NAS security
// algorithms IE: EPS ciphering (EEA) and EPS integrity (EIA) — TS 24.301 §9.9.3.23.
type SelectedEPSNASSecurityAlgorithms struct {
	Ciphering utils.EnumField `json:"ciphering"`
	Integrity utils.EnumField `json:"integrity"`
}

func getEPSCiphering(value uint8) utils.EnumField {
	return utils.NamedEnum(value, eps.CipheringAlgorithmName(nas.CipheringAlgorithm(value)))
}

func getEPSIntegrity(value uint8) utils.EnumField {
	return utils.NamedEnum(value, eps.IntegrityAlgorithmName(nas.IntegrityAlgorithm(value)))
}

func buildIMEISVRequest(v uint8) utils.EnumField {
	return utils.NamedEnum(v, fgs.IMEISVRequest(v).String())
}

func getIntegrity(value uint8) utils.EnumField {
	return utils.NamedEnum(value, fgs.IntegrityAlgorithmName(nas.IntegrityAlgorithm(value)))
}

func getCiphering(value uint8) utils.EnumField {
	return utils.NamedEnum(value, fgs.CipheringAlgorithmName(nas.CipheringAlgorithm(value)))
}

// S1UESecurityCapability is the S1 UE security capability the AMF replays
// (TS 24.501 §9.11.3.48A, which defers to TS 24.301 §9.9.3.36). Hex keeps the
// bytes the UE compares against what it sent (TS 33.501 §6.7.2); the algorithm
// lists are a reading of them, not a substitute.
type S1UESecurityCapability struct {
	Hex   string   `json:"hex"`
	EEA   []string `json:"eea,omitempty"`
	EIA   []string `json:"eia,omitempty"`
	UEA   []string `json:"uea,omitempty"`
	UIA   []string `json:"uia,omitempty"`
	GEA   []string `json:"gea,omitempty"`
	Error string   `json:"error,omitempty"`
}

func algorithmNames(prefix string, set nas.AlgorithmSet) []string {
	identities := set.Identities()
	if len(identities) == 0 {
		return nil
	}

	names := make([]string, 0, len(identities))
	for _, n := range identities {
		names = append(names, fmt.Sprintf("%s%d", prefix, n))
	}

	return names
}

func s1UESecurityCapability(b []byte) *S1UESecurityCapability {
	out := &S1UESecurityCapability{Hex: hex.EncodeToString(b)}

	capability, err := eps.ParseUESecurityCapability(b)
	if err != nil {
		out.Error = err.Error()

		return out
	}

	out.EEA = algorithmNames("EEA", capability.EEA)
	out.EIA = algorithmNames("EIA", capability.EIA)

	if capability.HasUMTS {
		out.UEA = algorithmNames("UEA", capability.UEA)
		out.UIA = algorithmNames("UIA", capability.UIA)
	}

	if capability.HasGERAN {
		out.GEA = algorithmNames("GEA", capability.GEA)
	}

	return out
}
