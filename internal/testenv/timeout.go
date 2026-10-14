// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package testenv

import (
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	MultiplierEnv = "ELLA_TEST_TIMEOUT_MULTIPLIER"
	ciMultiplier  = 3
)

var multiplier = sync.OnceValue(func() int64 {
	return parseMultiplier(os.Getenv(MultiplierEnv), os.Getenv("CI") != "")
})

func parseMultiplier(raw string, ci bool) int64 {
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil && n >= 1 {
		return n
	}

	if ci {
		return ciMultiplier
	}

	return 1
}

func Timeout(d time.Duration) time.Duration {
	return d * time.Duration(multiplier())
}
