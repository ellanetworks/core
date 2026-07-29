// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type IdentityRequest struct {
	TypeOfIdentity utils.EnumField `json:"type_of_identity"`
}

func buildIdentityRequest(msg *fgs.IdentityRequest) *IdentityRequest {
	return &IdentityRequest{
		TypeOfIdentity: buildTypeOfIdentityEnum(uint8(msg.IdentityType)),
	}
}
