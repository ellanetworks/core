// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type IdentityRequest struct {
	IdentityType uint8 `json:"identity_type"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildIdentityRequest(msg *eps.IdentityRequest) *IdentityRequest {
	out := &IdentityRequest{IdentityType: uint8(msg.IdentityType)}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
