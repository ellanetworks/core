// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import "github.com/ellanetworks/core/nas/fgs"

// secCapFromBytes decodes a UE security capability fixture that must be
// well-formed, so wire-shaped fixtures stay readable as expressions.
func secCapFromBytes(b ...byte) *fgs.UESecurityCapability {
	c, err := fgs.ParseUESecurityCapability(b)
	if err != nil {
		panic(err)
	}

	return &c
}
