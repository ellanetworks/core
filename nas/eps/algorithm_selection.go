// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// SelectNASAlgorithms picks the EPS NAS algorithms allowed by both the UE
// network capability and the operator policy (TS 33.401 §7.2.4.3), in the
// operator's order of preference. It reports false when the UE and the operator
// share no algorithm, so the caller rejects rather than falling back to the null
// algorithm.
//
// It is the one place EPS NAS algorithms are negotiated. Both the MME (for the
// EPS security mode control procedure) and the AMF (for the EPS NAS algorithms
// it hands the UE for use after mobility to EPS, TS 33.501 §6.7.2) call it, so
// a UE that moves between the two systems is offered a consistent choice.
func SelectNASAlgorithms(uecap UENetworkCapability, integrity []nas.IntegrityAlgorithm, ciphering []nas.CipheringAlgorithm) (eea nas.CipheringAlgorithm, eia nas.IntegrityAlgorithm, ok bool) {
	eea, eok := selectAlgorithm(ciphering, uecap.SupportsEEA)
	eia, iok := selectAlgorithm(integrity, uecap.SupportsEIA)

	return eea, eia, eok && iok
}

// selectAlgorithm returns the first operator-preferred algorithm the UE
// advertises support for, reporting false when none is. The null algorithm is
// selected only when the operator lists it and the UE advertises it
// (TS 33.401 §5: EIA0 is not an implicit fallback for a non-emergency UE).
func selectAlgorithm[T ~uint8](preference []T, supported func(uint8) bool) (T, bool) {
	for _, v := range preference {
		if supported(uint8(v)) {
			return v, true
		}
	}

	return 0, false
}
