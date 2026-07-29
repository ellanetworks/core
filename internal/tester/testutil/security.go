// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutil

import (
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type IntegrityAlgorithms struct {
	Nia0 bool
	Nia1 bool
	Nia2 bool
	Nia3 bool
}

type CipheringAlgorithms struct {
	Nea0 bool
	Nea1 bool
	Nea2 bool
	Nea3 bool
}

type UeSecurityCapability struct {
	Integrity IntegrityAlgorithms
	Ciphering CipheringAlgorithms
}

// GetUESecurityCapability builds the 5G UE security capability information
// element: the 5G-EA octet then the 5G-IA octet, each with algorithm n in bit
// (8-n) (TS 24.501 §9.11.3.54).
func GetUESecurityCapability(secCap *UeSecurityCapability) fgs.UESecurityCapability {
	var cipher, integrity []uint8

	if secCap.Ciphering.Nea0 {
		cipher = append(cipher, 0)
	}

	if secCap.Ciphering.Nea1 {
		cipher = append(cipher, 1)
	}

	if secCap.Ciphering.Nea2 {
		cipher = append(cipher, 2)
	}

	if secCap.Ciphering.Nea3 {
		cipher = append(cipher, 3)
	}

	if secCap.Integrity.Nia0 {
		integrity = append(integrity, 0)
	}

	if secCap.Integrity.Nia1 {
		integrity = append(integrity, 1)
	}

	if secCap.Integrity.Nia2 {
		integrity = append(integrity, 2)
	}

	if secCap.Integrity.Nia3 {
		integrity = append(integrity, 3)
	}

	return fgs.UESecurityCapability{
		EA: nas.Algorithms(cipher...),
		IA: nas.Algorithms(integrity...),
	}
}
