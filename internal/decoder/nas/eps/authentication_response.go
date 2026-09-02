// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type AuthenticationResponse struct {
	RES string `json:"res"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildAuthenticationResponse(msg *eps.AuthenticationResponse) *AuthenticationResponse {
	out := &AuthenticationResponse{RES: hex.EncodeToString(msg.RES)}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
