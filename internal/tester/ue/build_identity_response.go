// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"github.com/ellanetworks/core/nas/fgs"
)

type IdentityResponseOpts struct {
	Suci fgs.MobileIdentity
}

func BuildIdentityResponse(opts *IdentityResponseOpts) ([]byte, error) {
	m := &fgs.IdentityResponse{MobileIdentity: opts.Suci}

	return m.MarshalBinary()
}
