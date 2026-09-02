// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type IdentityResponse struct {
	MobileIdentity string `json:"mobile_identity"` // raw value, hex

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildIdentityResponse(msg *eps.IdentityResponse) *IdentityResponse {
	out := &IdentityResponse{MobileIdentity: msg.MobileIdentity.String()}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
