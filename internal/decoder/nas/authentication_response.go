// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type AuthenticationResponseParameter struct {
	ResStar [16]uint8 `json:"res_star"`
}

type AuthenticationResponse struct {
	AuthenticationResponseParameter *AuthenticationResponseParameter `json:"authentication_response_parameter,omitempty"`
	EAPMessage                      *utils.RawOctets                 `json:"eap_message,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildAuthenticationResponse(msg *fgs.AuthenticationResponse) *AuthenticationResponse {
	out := &AuthenticationResponse{
		EAPMessage: utils.NewRawOctets(msg.EAP),
	}

	if msg.RES != nil {
		var p AuthenticationResponseParameter
		copy(p.ResStar[:], msg.RES)
		out.AuthenticationResponseParameter = &p
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
