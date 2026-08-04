// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// bitRateUnits are the suffixes 3GPP TS 29.571 allows in an AMBR string.
var bitRateUnits = map[string]float64{
	"bps":  1,
	"Kbps": 1e3,
	"Mbps": 1e6,
	"Gbps": 1e9,
	"Tbps": 1e12,
}

// ParseBitRate converts an AMBR of the form "1 Gbps" (TS 29.571 §5.4.4.16)
// into bits per second. A malformed value is an error rather than a silent
// zero: an AMBR that quietly becomes 0 would be sent on the wire as a rate the
// UE cannot exceed.
func ParseBitRate(s string) (uint64, error) {
	value, unit, ok := strings.Cut(strings.TrimSpace(s), " ")
	if !ok {
		return 0, fmt.Errorf("bit rate %q has no unit", s)
	}

	multiplier, ok := bitRateUnits[unit]
	if !ok {
		return 0, fmt.Errorf("bit rate %q has unknown unit %q", s, unit)
	}

	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("bit rate %q: %w", s, err)
	}

	if n < 0 || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("bit rate %q is not a usable rate", s)
	}

	scaled := n * multiplier
	if scaled > math.MaxUint64 {
		return 0, fmt.Errorf("bit rate %q overflows", s)
	}

	return uint64(scaled), nil
}
