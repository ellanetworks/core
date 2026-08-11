// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas"

// SelectNASAlgorithms picks the 5G NAS algorithms allowed by both the UE
// security capability and the operator policy (TS 33.501 §6.7.2), in the
// operator's order of preference. It reports false when the UE and the operator
// share no algorithm, so the caller rejects rather than falling back to the null
// algorithm.
func SelectNASAlgorithms(uecap UESecurityCapability, integrity []nas.IntegrityAlgorithm, ciphering []nas.CipheringAlgorithm) (nea nas.CipheringAlgorithm, nia nas.IntegrityAlgorithm, ok bool) {
	nea, eok := nas.SelectAlgorithm(ciphering, uecap.SupportsEA)
	nia, iok := nas.SelectAlgorithm(integrity, uecap.SupportsIA)

	return nea, nia, eok && iok
}
