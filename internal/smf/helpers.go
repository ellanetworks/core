// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//
// Modified by Ella Networks.

package smf

import (
	"strconv"
	"strings"
)

func bitRateTokbps(bitrate string) uint64 {
	s := strings.Split(bitrate, " ")
	if len(s) != 2 {
		return 0
	}

	digit, err := strconv.Atoi(s[0])
	if err != nil {
		return 0
	}

	switch s[1] {
	case "bps":
		return uint64(digit / 1000)
	case "Kbps":
		return uint64(digit * 1)
	case "Mbps":
		return uint64(digit * 1000)
	case "Gbps":
		return uint64(digit * 1000000)
	case "Tbps":
		return uint64(digit * 1000000000)
	default:
		return 0
	}
}
