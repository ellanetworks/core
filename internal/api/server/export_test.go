// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"testing"
	"time"
)

func SetRoamerWithdrawalTimeoutForTest(t testing.TB, timeout time.Duration) {
	previous := roamerWithdrawalTiming
	roamerWithdrawalTiming = roamerWithdrawal{timeout: timeout, poll: 10 * time.Millisecond, margin: previous.margin}

	t.Cleanup(func() { roamerWithdrawalTiming = previous })
}
