// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas/fgs"
)

type AuthenticationFailureOpts struct {
	Cause fgs.GMMCause
	AUTS  []byte
}

// BuildAuthenticationFailure encodes the UE's rejection of an AUTHENTICATION
// REQUEST (TS 24.501 §8.2.4). A synch failure carries the AUTS the UE computed
// so the network can resynchronise its sequence number (TS 33.501 §6.1.3.2.1);
// every other cause carries none.
func BuildAuthenticationFailure(opts *AuthenticationFailureOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("AuthenticationFailureOpts is nil")
	}

	if opts.Cause == fgs.GMMCauseSynchFailure && len(opts.AUTS) == 0 {
		return nil, fmt.Errorf("a synch failure needs the AUTS the network resynchronises from")
	}

	m := &fgs.AuthenticationFailure{
		Cause: opts.Cause,
		AUTS:  opts.AUTS,
	}

	return m.MarshalBinary()
}
