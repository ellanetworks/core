// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "github.com/ellanetworks/core/nas/fgs"

type AuthenticationResponseParameter struct {
	ResStar [16]uint8 `json:"res_star"`
}

type AuthenticationResponse struct {
	AuthenticationResponseParameter *AuthenticationResponseParameter `json:"authentication_response_parameter,omitempty"`
	EAPMessage                      []byte                           `json:"eap_message,omitempty"`
}

func buildAuthenticationResponse(msg *fgs.AuthenticationResponse) *AuthenticationResponse {
	out := &AuthenticationResponse{
		EAPMessage: msg.EAP,
	}

	if msg.RES != nil {
		var p AuthenticationResponseParameter
		copy(p.ResStar[:], msg.RES)
		out.AuthenticationResponseParameter = &p
	}

	return out
}
