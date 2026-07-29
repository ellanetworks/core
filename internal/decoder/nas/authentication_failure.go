// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type AuthenticationFailure struct {
	Cause5GMM utils.EnumField `json:"cause"`

	AuthenticationFailureParameter *string `json:"authentication_failure_parameter,omitempty"`
}

func buildAuthenticationFailure(msg *fgs.AuthenticationFailure) *AuthenticationFailure {
	out := &AuthenticationFailure{
		Cause5GMM: cause5GMMToEnum(msg.Cause),
	}

	if msg.AUTS != nil {
		s := hex.EncodeToString(msg.AUTS)
		out.AuthenticationFailureParameter = &s
	}

	return out
}
