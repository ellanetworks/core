// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/fgs"
)

// NAS security algorithm identifiers (TS 33.501 §9.4.2.3). NEA3/NIA3 (128-ZUC)
// are defined for completeness but not implemented — Ella does not offer ZUC.
const (
	algCiphering128NEA0 uint8 = 0x00 // NULL
	algCiphering128NEA1 uint8 = 0x01 // 128-SNOW3G
	algCiphering128NEA2 uint8 = 0x02 // 128-AES
	algCiphering128NEA3 uint8 = 0x03 // 128-ZUC

	algIntegrity128NIA0 uint8 = 0x00 // NULL
	algIntegrity128NIA1 uint8 = 0x01 // 128-SNOW3G
	algIntegrity128NIA2 uint8 = 0x02 // 128-AES
	algIntegrity128NIA3 uint8 = 0x03 // 128-ZUC
)

// nasBearer3GPP is the BEARER input to the 5G NAS algorithms: the NAS connection
// identifier for 3GPP access (TS 33.501 §6.9.2.1). fgs.Protect/Unprotect apply it
// internally; it is needed here only to decipher a bare NAS container that carries
// no security header of its own.
const nasBearer3GPP uint8 = 0x01

// IntegrityAlg / CipherAlg map a 5G NAS algorithm identity to the nas
// implementation: null (NIA0/NEA0), SNOW3G (128-NIA1/128-NEA1), or AES
// (128-NIA2/128-NEA2). An unrecognized value falls back to null.
func IntegrityAlg(nia byte) nascommon.Integrity {
	switch nia {
	case algIntegrity128NIA1:
		return nascommon.SNOW3GIntegrity{}
	case algIntegrity128NIA2:
		return nascommon.AESCMACIntegrity{}
	default:
		return nascommon.NullIntegrity{}
	}
}

func CipherAlg(nea byte) nascommon.Cipher {
	switch nea {
	case algCiphering128NEA1:
		return nascommon.SNOW3GCipher{}
	case algCiphering128NEA2:
		return nascommon.AESCTRCipher{}
	default:
		return nascommon.NullCipher{}
	}
}

func cipheringAlgName(alg byte) string {
	switch alg {
	case algCiphering128NEA0:
		return "NEA0"
	case algCiphering128NEA1:
		return "NEA1"
	case algCiphering128NEA2:
		return "NEA2"
	case algCiphering128NEA3:
		return "NEA3"
	default:
		return ""
	}
}

func integrityAlgName(alg byte) string {
	switch alg {
	case algIntegrity128NIA0:
		return "NIA0"
	case algIntegrity128NIA1:
		return "NIA1"
	case algIntegrity128NIA2:
		return "NIA2"
	case algIntegrity128NIA3:
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

	// The NEA/NIA algorithm identity equals the support-bit index in the UE
	// security capability (NEA0/NIA0 = bit 8, NEA1/NIA1 = bit 7, …), so the operator
	// preference value indexes SupportsEA/SupportsIA directly (TS 24.501 §9.11.3.54).
	sc, err := fgs.ParseUESecurityCapability(uecap)
	if err != nil {
		return 0, 0, false
	}

	nia, iok := selectNASAlg(intOrder, sc.SupportsIA)
	nea, eok := selectNASAlg(encOrder, sc.SupportsEA)

	return nea, nia, iok && eok
}
