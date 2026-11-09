// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"github.com/ellanetworks/core/internal/models"
)

const (
	dscpDefault = 0
	dscpAF11    = 10
	dscpAF21    = 18
	dscpAF41    = 34
	dscpAF42    = 36
	dscpAF43    = 38
	dscpCS5     = 40
	dscpCS6     = 48
	dscpEF      = 46

	tosTrafficClassMask = uint16(0xFC)
)

func dscpForQos(fiveQI uint8, arp models.Arp) uint8 {
	switch fiveQI {
	case 1, 65, 66, 82, 83, 84, 85, 86, 87, 88, 89, 90:
		return dscpEF
	case 2, 67, 71, 72, 73, 74, 75, 76:
		return dscpAF41
	case 3:
		return dscpAF42
	case 4:
		return dscpAF43
	case 5:
		return dscpCS5
	case 69:
		return dscpCS6
	}

	switch {
	case arp.PriorityLevel >= 1 && arp.PriorityLevel <= 5:
		return dscpAF21
	case arp.PriorityLevel >= 6 && arp.PriorityLevel <= 10:
		return dscpAF11
	default:
		return dscpDefault
	}
}

func transportLevelMarking(fiveQI uint8, arp models.Arp) uint16 {
	dscp := dscpForQos(fiveQI, arp)
	if dscp == dscpDefault {
		return 0
	}

	return uint16(dscp)<<10 | tosTrafficClassMask
}

func arpValue(arp *models.Arp) models.Arp {
	if arp == nil {
		return models.Arp{}
	}

	return *arp
}
