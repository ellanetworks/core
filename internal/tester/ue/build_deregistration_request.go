// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type DeregistrationRequestOpts struct {
	Guti *fgs.MobileIdentity
	Suci *fgs.MobileIdentity
	Ksi  int32
}

func BuildDeregistrationRequest(opts *DeregistrationRequestOpts) ([]byte, error) {
	var mobileIdentity fgs.MobileIdentity

	switch {
	case opts.Guti != nil:
		mobileIdentity = *opts.Guti
	case opts.Suci != nil:
		mobileIdentity = *opts.Suci
	default:
		return nil, fmt.Errorf("either Guti or Suci must be provided")
	}

	m := &fgs.DeregistrationRequestUEOriginating{
		AccessType:             1,
		ReRegistrationRequired: false,
		SwitchOff:              true,
		NgKSI:                  nas.KeySetIdentifier{Value: uint8(opts.Ksi)},
		MobileIdentity:         mobileIdentity,
	}

	return m.MarshalBinary()
}
