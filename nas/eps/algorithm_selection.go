// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// SelectNASAlgorithms picks the EPS NAS algorithms allowed by both the UE
// network capability and the operator policy (TS 33.401 §7.2.4.3), in the
// operator's order of preference. It reports false when the UE and the operator
// share no algorithm.
func SelectNASAlgorithms(uecap UENetworkCapability, integrity []nas.IntegrityAlgorithm, ciphering []nas.CipheringAlgorithm) (eea nas.CipheringAlgorithm, eia nas.IntegrityAlgorithm, ok bool) {
	eea, eok := nas.SelectAlgorithm(ciphering, uecap.SupportsEEA)
	eia, iok := nas.SelectAlgorithm(integrity, uecap.SupportsEIA)

	return eea, eia, eok && iok
}
