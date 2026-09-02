// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type SecurityModeCommand struct {
	NASKeySetIdentifier          uint8                 `json:"nas_key_set_identifier"`
	ReplayedUESecurityCapability *UESecurityCapability `json:"replayed_ue_security_capability,omitempty"`
	HASHMME                      *utils.RawOctets      `json:"hash_mme,omitempty"`
	CipheringAlgorithm           utils.EnumField       `json:"ciphering_algorithm"`
	IntegrityAlgorithm           utils.EnumField       `json:"integrity_algorithm"`
	IMEISVRequested              bool                  `json:"imeisv_requested"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildSecurityModeCommand(msg *eps.SecurityModeCommand) *SecurityModeCommand {
	out := &SecurityModeCommand{
		CipheringAlgorithm: cipheringAlgToEnum(uint8(msg.CipheringAlgorithm)),
		IntegrityAlgorithm: integrityAlgToEnum(uint8(msg.IntegrityAlgorithm)),
		IMEISVRequested:    msg.IMEISVRequested != nil && msg.IMEISVRequested.Requested(),
	}

	out.NASKeySetIdentifier = msg.NASKeySetIdentifier.HalfOctet()
	out.ReplayedUESecurityCapability = ueSecurityCapability(msg.ReplayedUESecurityCapability)
	out.HASHMME = utils.NewRawOctets(msg.HASHMME)
	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
