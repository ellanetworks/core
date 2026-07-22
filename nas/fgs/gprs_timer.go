// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"fmt"
	"time"
)

// GPRS timer unit codes (TS 24.008 §10.5.7.4): the high three bits of the octet.
const (
	gprsTimer2Unit2Seconds  uint8 = 0b000
	gprsTimer2Unit1Minute   uint8 = 0b001
	gprsTimer2UnitDecihours uint8 = 0b010
)

var gprsTimer2Units = []struct {
	bits uint8
	step time.Duration
}{
	{gprsTimer2Unit2Seconds, 2 * time.Second},
	{gprsTimer2Unit1Minute, time.Minute},
	{gprsTimer2UnitDecihours, 6 * time.Minute}, // 1 decihour = 1/10 hour
}

// EncodeGPRSTimer2 encodes d as a one-octet GPRS timer 2 IE value (TS 24.008
// §10.5.7.4): the high three bits select the unit and the low five the value
// (0-31). It picks the smallest unit representing d exactly, and errors when d
// cannot be carried — a configured timer the IE cannot express fails loudly.
func EncodeGPRSTimer2(d time.Duration) (uint8, error) {
	for _, u := range gprsTimer2Units {
		if d%u.step != 0 {
			continue
		}

		n := d / u.step
		if n >= 0 && n <= 31 {
			return u.bits<<5 | uint8(n), nil
		}
	}

	return 0, fmt.Errorf("nas/fgs: cannot encode %v as a GPRS timer 2 (TS 24.008)", d)
}
