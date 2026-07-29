// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/ellanetworks/core/nas/fgs"
)

// TS 33.501 Annex A.9.
func AlgorithmKeyDerivation(cipheringAlg uint8, kamf []byte, knasEnc *[16]uint8, IntegrityAlg uint8, knasInt *[16]uint8) error {
	P0 := []byte{nnasEncAlg}
	L0 := ueauth.KDFLen(P0)
	P1 := []byte{cipheringAlg}
	L1 := ueauth.KDFLen(P1)

	kenc, err := ueauth.GetKDFValue(kamf, ueauth.FCForAlgorithmKeyDerivation, P0, L0, P1, L1)
	if err != nil {
		return err
	}

	copy(knasEnc[:], kenc[16:32])

	P0 = []byte{nnasIntAlg}
	L0 = ueauth.KDFLen(P0)
	P1 = []byte{IntegrityAlg}
	L1 = ueauth.KDFLen(P1)

	kint, err := ueauth.GetKDFValue(kamf, ueauth.FCForAlgorithmKeyDerivation, P0, L0, P1, L1)
	if err != nil {
		return err
	}

	copy(knasInt[:], kint[16:32])

	return nil
}

func SelectAlgorithms(c fgs.UESecurityCapability) (uint8, uint8) {
	var (
		integrityAlgorithm uint8
		cipheringAlgorithm uint8
	)

	switch {
	case c.SupportsIA(0):
		integrityAlgorithm = AlgIntegrity128NIA0
	case c.SupportsIA(1):
		integrityAlgorithm = AlgIntegrity128NIA1
	case c.SupportsIA(2):
		integrityAlgorithm = AlgIntegrity128NIA2
	case c.SupportsIA(3):
		integrityAlgorithm = AlgIntegrity128NIA3
	}

	switch {
	case c.SupportsEA(0):
		cipheringAlgorithm = AlgCiphering128NEA0
	case c.SupportsEA(1):
		cipheringAlgorithm = AlgCiphering128NEA1
	case c.SupportsEA(2):
		cipheringAlgorithm = AlgCiphering128NEA2
	case c.SupportsEA(3):
		cipheringAlgorithm = AlgCiphering128NEA3
	}

	return integrityAlgorithm, cipheringAlgorithm
}
