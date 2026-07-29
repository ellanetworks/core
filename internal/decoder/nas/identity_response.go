// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type MobileIdentity struct {
	Identity utils.EnumField `json:"identity_type"`
	PLMNID   *PLMNID         `json:"plmn_id,omitempty"`
	SUCI     *string         `json:"suci,omitempty"`
	GUTI     *string         `json:"guti,omitempty"`
	STMSI    *string         `json:"s_tmsi,omitempty"`
	IMEI     *string         `json:"imei,omitempty"`
	IMEISV   *string         `json:"imeisv,omitempty"`
}

type IdentityResponse struct {
	MobileIdentity MobileIdentity `json:"mobile_identity"`
}

func buildIdentityResponse(msg *fgs.IdentityResponse) *IdentityResponse {
	return &IdentityResponse{
		MobileIdentity: buildMobileIdentity(msg.MobileIdentity),
	}
}
