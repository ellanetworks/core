// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "github.com/ellanetworks/core/nas/fgs"

func secCapFromBytes(b ...byte) *fgs.UESecurityCapability {
	c, err := fgs.ParseUESecurityCapability(b)
	if err != nil {
		panic(err)
	}

	return &c
}
