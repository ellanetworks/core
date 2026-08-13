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

func (g GUTI5G) MappedEPSGUTI() (eps.GUTI, error) {
	id, err := g.MobileIdentity()
	if err != nil {
		return eps.GUTI{}, err
	}

	return MapGUTI5GToEPS(*id.GUTI), nil
}

func MapGUTIEPSTo5G(g eps.GUTI) fgs.GUTI {
	return fgs.GUTI{
		PLMN:        g.PLMN,
		AMFRegionID: uint8(g.MMEGroupID >> 8),
		AMFSetID:    (g.MMEGroupID&0xFF)<<2 | uint16(g.MMECode>>6),
		AMFPointer:  g.MMECode & 0x3F,
		TMSI:        g.TMSI,
	}
}
