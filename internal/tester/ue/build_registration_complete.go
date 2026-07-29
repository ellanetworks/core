// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas/fgs"
)

type RegistrationCompleteOpts struct {
	SORTransparentContainer []uint8
}

func BuildRegistrationComplete(opts *RegistrationCompleteOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("RegistrationCompleteOpts is nil")
	}

	return (&fgs.RegistrationComplete{SORTransparentContainer: opts.SORTransparentContainer}).MarshalBinary()
}
