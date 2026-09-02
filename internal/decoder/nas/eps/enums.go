// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func emmTypeToEnum(mt eps.MessageType) utils.EnumField {
	return utils.NamedEnum(uint8(mt), mt.Name())
}

func esmTypeToEnum(mt eps.ESMMessageType) utils.EnumField {
	return utils.NamedEnum(uint8(mt), mt.Name())
}

func attachTypeToEnum(v eps.AttachType) utils.EnumField {
	return utils.NamedEnum(uint8(v), v.Name())
}

func attachResultToEnum(v eps.AttachResult) utils.EnumField {
	return utils.NamedEnum(uint8(v), v.Name())
}

func updateTypeToEnum(v eps.EPSUpdateType) utils.EnumField {
	return utils.NamedEnum(uint8(v), v.Name())
}

func updateResultToEnum(v eps.EPSUpdateResult) utils.EnumField {
	return utils.NamedEnum(uint8(v), v.Name())
}

func cipheringAlgToEnum(v uint8) utils.EnumField {
	return utils.NamedEnum(v, eps.CipheringAlgorithmName(nas.CipheringAlgorithm(v)))
}

func integrityAlgToEnum(v uint8) utils.EnumField {
	return utils.NamedEnum(v, eps.IntegrityAlgorithmName(nas.IntegrityAlgorithm(v)))
}
