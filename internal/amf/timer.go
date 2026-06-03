// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package amf

import (
	"time"

	"github.com/ellanetworks/core/internal/util/timer"
)

// Timer is the shared retransmission timer (see internal/util/timer).
type Timer = timer.Timer

// NewTimer starts a retransmission timer; see timer.New.
func NewTimer(d time.Duration, maxRetryTimes int32, expiredFunc func(expireTimes int32), cancelFunc func()) *Timer {
	return timer.New(d, maxRetryTimes, expiredFunc, cancelFunc)
}
