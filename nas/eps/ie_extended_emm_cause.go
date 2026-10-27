// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "fmt"

// ExtendedEMMCause is the Extended EMM cause value (TS 24.301 §9.9.3.26A).
type ExtendedEMMCause struct {
	EUTRANNotAllowed            bool
	EPSOptimizationNotSupported bool
	NBIoTNotAllowed             bool
}

// ParseExtendedEMMCause decodes an Extended EMM cause value nibble.
func ParseExtendedEMMCause(v uint8) ExtendedEMMCause {
	return ExtendedEMMCause{
		EUTRANNotAllowed:            v&0x01 != 0,
		EPSOptimizationNotSupported: v&0x02 != 0,
		NBIoTNotAllowed:             v&0x04 != 0,
	}
}

// Nibble is the value half-octet this element occupies.
func (c ExtendedEMMCause) Nibble() uint8 {
	return boolBit(c.EUTRANNotAllowed, 0) | boolBit(c.EPSOptimizationNotSupported, 1) | boolBit(c.NBIoTNotAllowed, 2)
}

func (c ExtendedEMMCause) String() string {
	return fmt.Sprintf("EUTRANNotAllowed=%t EPSOptimizationNotSupported=%t NBIoTNotAllowed=%t",
		c.EUTRANNotAllowed, c.EPSOptimizationNotSupported, c.NBIoTNotAllowed)
}
