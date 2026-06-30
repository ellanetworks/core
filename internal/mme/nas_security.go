// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"crypto/sha256"

	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
)

// HashMME returns the 8-octet HashMME for the SECURITY MODE COMMAND — the 64 most
// significant bits of the SHA-256 of the triggering plain Attach/TAU — or nil when
// there is nothing to hash (TS 24.301 §5.4.3.2, TS 33.401 §6.4.2.1).
func HashMME(input []byte) []byte {
	if len(input) == 0 {
		return nil
	}

	sum := sha256.Sum256(input)

	return sum[:8]
}

// SelectAlgorithms picks the EPS NAS algorithms allowed by both the UE network
// capability and the operator policy (TS 33.401), in the operator's order of
// preference. The operator's list uses the RAT-neutral algorithm identities
// (NULL ≡ EEA0/EIA0, SNOW3G ≡ 128-EEA1/128-EIA1, AES ≡ 128-EEA2/128-EIA2). It
// reports false when the UE and operator share no algorithm, so the caller
// rejects the attach without falling back to the null algorithm.
func SelectAlgorithms(ueNetCap []byte, ciphering, integrity []string) (eea, eia byte, ok bool) {
	uecap, err := eps.ParseUENetworkCapability(ueNetCap)
	if err != nil {
		return 0, 0, false
	}

	eea, eok := selectEPSAlgorithm(ciphering, uecap.SupportsEEA)
	eia, iok := selectEPSAlgorithm(integrity, uecap.SupportsEIA)

	return eea, eia, eok && iok
}

// epsAlgorithmValue maps an operator algorithm identity to its EPS algorithm
// number (TS 33.401): NULL=0, SNOW3G=1, AES=2.
func epsAlgorithmValue(name string) (byte, bool) {
	switch name {
	case "NULL":
		return 0, true
	case "SNOW3G":
		return 1, true
	case "AES":
		return 2, true
	default:
		return 0, false
	}
}

// selectEPSAlgorithm returns the first operator-preferred algorithm the UE
// advertises support for, reporting false when none is common. The null
// algorithm is selected only when the operator lists it and the UE advertises it
// (TS 33.401 §5: EIA0 is not an implicit fallback for a non-emergency UE).
func selectEPSAlgorithm(preference []string, supported func(uint8) bool) (byte, bool) {
	for _, name := range preference {
		v, ok := epsAlgorithmValue(name)
		if !ok {
			continue
		}

		if supported(v) {
			return v, true
		}
	}

	return 0, false
}

// geaOctet maps the GERAN ciphering algorithms from the UE's MS network
// capability (TS 24.008: GEA1 at octet 1 bit 8, GEA2-7 at octet 2 bits 7-2) onto
// UE security capability octet 7 (TS 24.301: GEA1 at bit 7 down to GEA7 at bit 1,
// bit 8 spare). It returns 0 when the UE advertised no GERAN ciphering.
func geaOctet(msNetCap []byte) byte {
	if len(msNetCap) == 0 {
		return 0
	}

	gea1 := msNetCap[0] >> 7 & 0x01

	var extended byte
	if len(msNetCap) >= 2 {
		extended = msNetCap[1] >> 1 & 0x3f
	}

	return gea1<<6 | extended
}

// ReplayedUESecCap builds the Replayed UE security capabilities IE that the
// SECURITY MODE COMMAND echoes back so the UE can detect bidding-down (TS 24.301).
// The UE rejects the command with cause #23 if the replay differs from the
// capabilities it sent: EPS algorithms from the UE network capability, UMTS
// algorithms from its octets 5-6, and GERAN algorithms from the MS network
// capability.
func ReplayedUESecCap(ueNetCap, msNetCap []byte) []byte {
	uecap, err := eps.ParseUENetworkCapability(ueNetCap)
	if err != nil {
		return nil
	}

	out := []byte{uecap.EEA, uecap.EIA}

	var uea, uia byte
	if len(uecap.Rest) >= 2 {
		// Octets 5-6 carry the UMTS algorithms (UEA, UIA). Octet 6 bit 8 is UCS2
		// support in the UE network capability but spare in the UE
		// security capability, so it is cleared here.
		uea, uia = uecap.Rest[0], uecap.Rest[1]&0x7f
	}

	// Octet 7 (GERAN) is included only when the UE indicated a Gb-mode algorithm,
	// and then octets 5-6 must also be present, zero-filled if the UE indicated no
	// Iu-mode algorithm.
	switch gea := geaOctet(msNetCap); {
	case gea != 0:
		out = append(out, uea, uia, gea)
	case len(uecap.Rest) >= 2:
		out = append(out, uea, uia)
	}

	return out
}

// IntegrityAlg / CipherAlg map an EPS algorithm identity to the nas
// implementation: null (EIA0/EEA0), SNOW3G (128-EIA1/128-EEA1), or AES
// (128-EIA2/128-EEA2). An unrecognized value falls back to null.
func IntegrityAlg(eia byte) nascommon.Integrity {
	switch eia {
	case 1:
		return nascommon.SNOW3GIntegrity{}
	case 2:
		return nascommon.AESCMACIntegrity{}
	default:
		return nascommon.NullIntegrity{}
	}
}

func CipherAlg(eea byte) nascommon.Cipher {
	switch eea {
	case 1:
		return nascommon.SNOW3GCipher{}
	case 2:
		return nascommon.AESCTRCipher{}
	default:
		return nascommon.NullCipher{}
	}
}
