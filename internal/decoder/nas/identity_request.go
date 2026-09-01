// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type IdentityRequest struct {
	TypeOfIdentity utils.EnumField `json:"type_of_identity"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildIdentityRequest(msg *fgs.IdentityRequest) *IdentityRequest {
	out := &IdentityRequest{
		TypeOfIdentity: buildTypeOfIdentityEnum(uint8(msg.IdentityType)),
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
