// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

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

func (ue *UeContext) SelectSecurityAlg(intOrder []nas.IntegrityAlgorithm, encOrder []nas.CipheringAlgorithm) (nea nas.CipheringAlgorithm, nia nas.IntegrityAlgorithm, ok bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.ueSecurityCapability == nil {
		return 0, 0, false
	}

	return fgs.SelectNASAlgorithms(*ue.ueSecurityCapability, intOrder, encOrder)
}
