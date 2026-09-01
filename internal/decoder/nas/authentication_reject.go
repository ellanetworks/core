// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type AuthenticationReject struct {
	EAPMessage []byte `json:"eap_message,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildAuthenticationReject(msg *fgs.AuthenticationReject) *AuthenticationReject {
	out := &AuthenticationReject{
		EAPMessage: msg.EAP,
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
