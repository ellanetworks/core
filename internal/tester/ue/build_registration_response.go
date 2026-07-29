// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"encoding/base64"
	"fmt"

	"github.com/ellanetworks/core/nas/fgs"
)

type AuthenticationResponseOpts struct {
	AuthenticationResponseParam []uint8
	EapMsg                      string
}

func BuildAuthenticationResponse(opts *AuthenticationResponseOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("AuthenticationResponseOpts is nil")
	}

	m := &fgs.AuthenticationResponse{}

	switch {
	case len(opts.AuthenticationResponseParam) > 0:
		m.RES = opts.AuthenticationResponseParam
	case opts.EapMsg != "":
		rawEapMsg, err := base64.StdEncoding.DecodeString(opts.EapMsg)
		if err != nil {
			return nil, fmt.Errorf("could not decode eap msg: %v", err)
		}

		m.EAP = rawEapMsg
	}

	return m.MarshalBinary()
}
