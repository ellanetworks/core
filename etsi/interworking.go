// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package etsi

import (
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

// MapGUTI5GToEPS maps a 5G-GUTI to the GUTI a UE presents in E-UTRAN
// (TS 23.003 §2.10.2.1.2), which is how it builds the TRACKING AREA UPDATE
// REQUEST after moving to S1 mode. The UE performs this mapping itself, so a
// network that has to recognise the result must perform the same one.
//
// The identities are not the same shape: 5GS names an AMF with a region, a set
// and a pointer, EPS an MME with a group and a code, so the spec repacks the
// bits rather than renaming fields.
func MapGUTI5GToEPS(g fgs.GUTI) eps.GUTI {
	return eps.GUTI{
		PLMN: g.PLMN,
		// The 8-bit AMF Region ID takes the top half of the MME Group ID, and the
		// top 8 of the AMF Set ID's 10 bits take the bottom half.
		MMEGroupID: uint16(g.AMFRegionID)<<8 | (g.AMFSetID>>2)&0xFF,
		// The AMF Set ID's remaining 2 bits sit above the 6-bit AMF Pointer.
		MMECode: (uint8(g.AMFSetID&0x03) << 6) | (g.AMFPointer & 0x3F),
		TMSI:    g.TMSI,
	}
}
