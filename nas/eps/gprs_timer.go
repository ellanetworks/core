// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"fmt"
	"time"
)

// GPRS Timer unit codes (TS 24.008 §10.5.7.3): the high three bits of the octet.
const (
	gprsTimerUnit2Seconds  uint8 = 0b000
	gprsTimerUnit1Minute   uint8 = 0b001
	gprsTimerUnitDecihours uint8 = 0b010
)

// gprsTimerUnits, finest first, so EncodeGPRSTimer picks the smallest unit that
// represents the duration exactly within the 5-bit value range.
var gprsTimerUnits = []struct {
	bits uint8
	step time.Duration
}{
	{gprsTimerUnit2Seconds, 2 * time.Second},
	{gprsTimerUnit1Minute, time.Minute},
	{gprsTimerUnitDecihours, 6 * time.Minute}, // 1 decihour = 1/10 hour
}

// EncodeGPRSTimer encodes d as a one-octet GPRS Timer IE value (TS 24.008
// §10.5.7.3): the high three bits select the unit and the low five bits the
// value (0–31). It returns an error if d cannot be represented exactly, so a
// configured timer that the IE cannot carry fails loudly rather than silently
// rounding.
func EncodeGPRSTimer(d time.Duration) (uint8, error) {
	for _, u := range gprsTimerUnits {
		if d%u.step != 0 {
			continue
		}

		n := d / u.step
		if n >= 0 && n <= 31 {
			return u.bits<<5 | uint8(n), nil
		}
	}

	return 0, fmt.Errorf("eps: cannot encode %v as a GPRS Timer (TS 24.008 §10.5.7.3)", d)
}
