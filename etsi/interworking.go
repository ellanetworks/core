// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package etsi

import (
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

func MapGUTI5GToEPS(g fgs.GUTI) eps.GUTI {
	return eps.GUTI{
		PLMN:       g.PLMN,
		MMEGroupID: uint16(g.AMFRegionID)<<8 | (g.AMFSetID>>2)&0xFF,
		MMECode:    (uint8(g.AMFSetID&0x03) << 6) | (g.AMFPointer & 0x3F),
		TMSI:       g.TMSI,
	}
}
