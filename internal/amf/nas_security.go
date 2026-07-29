// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import "github.com/ellanetworks/core/nas"

func cipheringAlgName(alg nas.CipheringAlgorithm) string {
	switch alg {
	case nas.CipheringNull:
		return "NEA0"
	case nas.CipheringSNOW3G:
		return "NEA1"
	case nas.CipheringAES:
		return "NEA2"
	case nas.CipheringZUC:
		return "NEA3"
	default:
		return ""
	}
}

func integrityAlgName(alg nas.IntegrityAlgorithm) string {
	switch alg {
	case nas.IntegrityNull:
		return "NIA0"
	case nas.IntegritySNOW3G:
		return "NIA1"
	case nas.IntegrityAES:
		return "NIA2"
	case nas.IntegrityZUC:
		return "NIA3"
	default:
		return ""
	}
}

// selectNASAlg returns the first network-preferred algorithm the UE supports,
// reporting false when none is nas.
func selectNASAlg[T ~uint8](preference []T, supported func(uint8) bool) (T, bool) {
	for _, alg := range preference {
		if supported(uint8(alg)) {
			return alg, true
		}
	}

	return 0, false
}

// SelectSecurityAlg negotiates the NAS ciphering and integrity algorithms against
// the UE's security capability, in the network's preference order (TS 33.501),
// returning ok=false when the UE capability is absent or no common algorithm is
// found for either. It does not mutate the UE — the caller installs the result via
// InstallNASSecurityContext.
func (ue *UeContext) SelectSecurityAlg(intOrder []nas.IntegrityAlgorithm, encOrder []nas.CipheringAlgorithm) (nea nas.CipheringAlgorithm, nia nas.IntegrityAlgorithm, ok bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	sc := ue.ueSecurityCapability
	if sc == nil {
		return 0, 0, false
	}

	// The NEA/NIA algorithm identity equals the support-bit index in the UE
	// security capability (NEA0/NIA0 = bit 8, NEA1/NIA1 = bit 7, …), so the operator
	// preference value indexes SupportsEA/SupportsIA directly (TS 24.501 §9.11.3.54).
	nia, iok := selectNASAlg(intOrder, sc.SupportsIA)
	nea, eok := selectNASAlg(encOrder, sc.SupportsEA)

	return nea, nia, iok && eok
}
