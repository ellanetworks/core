// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type AuthenticationRequest struct {
	SpareHalfOctetAndNgksi      uint8            `json:"spare_half_octet_and_ngksi"`
	ABBA                        []uint8          `json:"abba"`
	AuthenticationParameterAUTN [16]uint8        `json:"authentication_parameter_autn,omitempty"`
	AuthenticationParameterRAND [16]uint8        `json:"authentication_parameter_rand,omitempty"`
	EAPMessage                  *utils.RawOctets `json:"eap_message,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildAuthenticationRequest(msg *fgs.AuthenticationRequest) *AuthenticationRequest {
	out := &AuthenticationRequest{
		SpareHalfOctetAndNgksi: msg.NgKSI.HalfOctet(),
		ABBA:                   msg.ABBA,
		EAPMessage:             utils.NewRawOctets(msg.EAP),
	}

	if msg.RAND != nil {
		out.AuthenticationParameterRAND = *msg.RAND
	}

	if msg.AUTN != nil {
		out.AuthenticationParameterAUTN = *msg.AUTN
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
