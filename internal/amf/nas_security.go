// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/ellanetworks/core/nas/fgs"
)

func cipheringAlgName(alg byte) string {
	switch alg {
	case fgs.AlgCiphering128NEA0:
		return "NEA0"
	case fgs.AlgCiphering128NEA1:
		return "NEA1"
	case fgs.AlgCiphering128NEA2:
		return "NEA2"
	case fgs.AlgCiphering128NEA3:
		return "NEA3"
	default:
		return ""
	}
}

func integrityAlgName(alg byte) string {
	switch alg {
	case fgs.AlgIntegrity128NIA0:
		return "NIA0"
	case fgs.AlgIntegrity128NIA1:
		return "NIA1"
	case fgs.AlgIntegrity128NIA2:
		return "NIA2"
	case fgs.AlgIntegrity128NIA3:
		return "NIA3"
	default:
		return ""
	}
}

// selectNASAlg returns the first network-preferred algorithm the UE supports,
// reporting false when none is common.
func selectNASAlg(preference []uint8, supported func(uint8) bool) (byte, bool) {
	for _, alg := range preference {
		if supported(alg) {
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
func (ue *UeContext) SelectSecurityAlg(intOrder, encOrder []uint8) (nea, nia byte, ok bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	uecap := ue.ueSecurityCapability
	if uecap == nil {
		return 0, 0, false
	}

	nia, iok := selectNASAlg(intOrder, func(alg uint8) bool {
		switch alg {
		case fgs.AlgIntegrity128NIA0:
			return uecap.GetIA0_5G() == 1
		case fgs.AlgIntegrity128NIA1:
			return uecap.GetIA1_128_5G() == 1
		case fgs.AlgIntegrity128NIA2:
			return uecap.GetIA2_128_5G() == 1
		case fgs.AlgIntegrity128NIA3:
			return uecap.GetIA3_128_5G() == 1
		}

		return false
	})

	nea, eok := selectNASAlg(encOrder, func(alg uint8) bool {
		switch alg {
		case fgs.AlgCiphering128NEA0:
			return uecap.GetEA0_5G() == 1
		case fgs.AlgCiphering128NEA1:
			return uecap.GetEA1_128_5G() == 1
		case fgs.AlgCiphering128NEA2:
			return uecap.GetEA2_128_5G() == 1
		case fgs.AlgCiphering128NEA3:
			return uecap.GetEA3_128_5G() == 1
		}

		return false
	})

	return nea, nia, iok && eok
}
