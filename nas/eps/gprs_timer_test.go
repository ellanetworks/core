// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"testing"
	"time"
)

func TestEncodeGPRSTimer(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want uint8
	}{
		{54 * time.Minute, 0x49}, // T3412 default: 9 decihours (010 01001)
		{30 * time.Minute, 0x3e}, // 30 minutes (001 11110)
		{10 * time.Second, 0x05}, // 5 × 2 seconds (000 00101)
		{6 * time.Minute, 0x26},  // 6 minutes in 1-minute units (001 00110)
	}

	for _, c := range cases {
		got, err := EncodeGPRSTimer(c.d)
		if err != nil {
			t.Errorf("EncodeGPRSTimer(%v): unexpected error %v", c.d, err)
			continue
		}

		if got != c.want {
			t.Errorf("EncodeGPRSTimer(%v) = %#x, want %#x", c.d, got, c.want)
		}
	}
}

func TestEncodeGPRSTimerUnrepresentable(t *testing.T) {
	// 100 minutes is not a whole number of 2 s / 1 min (≤31) / 6 min, so it has
	// no exact one-octet GPRS Timer encoding.
	if _, err := EncodeGPRSTimer(100 * time.Minute); err == nil {
		t.Fatal("expected an error for an unrepresentable duration")
	}
}
