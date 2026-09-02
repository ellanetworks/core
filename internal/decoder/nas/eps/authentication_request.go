// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type AuthenticationRequest struct {
	NASKeySetIdentifier utils.KeySetIdentifier `json:"nas_key_set_identifier"`
	RAND                string                 `json:"rand"`
	AUTN                string                 `json:"autn"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildAuthenticationRequest(msg *eps.AuthenticationRequest) *AuthenticationRequest {
	out := &AuthenticationRequest{
		NASKeySetIdentifier: utils.NewKeySetIdentifier(msg.NASKeySetIdentifier),
		RAND:                hex.EncodeToString(msg.RAND[:]),
		AUTN:                hex.EncodeToString(msg.AUTN[:]),
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
