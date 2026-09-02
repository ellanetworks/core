// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type RegistrationComplete struct {
	SORTransparentContainer *RawOctets `json:"sor_transparent_container,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildRegistrationComplete(msg *fgs.RegistrationComplete) *RegistrationComplete {
	out := &RegistrationComplete{
		SORTransparentContainer: rawOctets(msg.SORTransparentContainer),
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
