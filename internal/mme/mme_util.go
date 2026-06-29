// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"strconv"
	"strings"
)

// bitRateToBps parses an "<n> <unit>" bitrate string (e.g. "1 Gbps") to bits/s.
func BitRateToBps(s string) uint64 {
	parts := strings.Split(s, " ")
	if len(parts) != 2 {
		return 0
	}

	n, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0
	}

	switch parts[1] {
	case "bps":
		return n
	case "Kbps":
		return n * 1_000
	case "Mbps":
		return n * 1_000_000
	case "Gbps":
		return n * 1_000_000_000
	case "Tbps":
		return n * 1_000_000_000_000
	default:
		return 0
	}
}
