// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasType

// GetBitMask number, pos is shift bit
// >= lb
// < up
// TODO　exception check
func GetBitMask(ub uint8, lb uint8) (bitMask uint8) {
	bitMask = ((1<<(ub-lb) - 1) << (lb))
	return bitMask
}
