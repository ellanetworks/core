// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "github.com/ellanetworks/core/nas/fgs"

type AuthenticationReject struct {
	EAPMessage []byte `json:"eap_message,omitempty"`
}

func buildAuthenticationReject(msg *fgs.AuthenticationReject) *AuthenticationReject {
	return &AuthenticationReject{
		EAPMessage: msg.EAP,
	}
}
