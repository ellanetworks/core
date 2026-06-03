// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

// Package timer provides a retransmission timer that fires a callback on each
// interval up to a retry limit, then a cancel callback once.
package timer

import (
	"sync"
	"sync/atomic"
	"time"
)

// Timer fires expiredFunc on each interval and manages the retry count
// automatically; once the retry count is exceeded it fires cancelFunc and stops.
type Timer struct {
	ticker        *time.Ticker
	expireTimes   int32 // accessed atomically
	maxRetryTimes int32 // accessed atomically
	done          chan bool
	active        int32 // 1 = active, 0 = stopped; accessed atomically
	stopOnce      sync.Once
}

// New starts a timer that calls expiredFunc on each interval d, passing the
// current expiry count. After more than maxRetryTimes expiries it calls
// cancelFunc and stops itself. Stop ends it early.
func New(d time.Duration, maxRetryTimes int32, expiredFunc func(expireTimes int32), cancelFunc func()) *Timer {
	t := &Timer{}
	atomic.StoreInt32(&t.expireTimes, 0)
	atomic.StoreInt32(&t.maxRetryTimes, maxRetryTimes)
	atomic.StoreInt32(&t.active, 1)
	t.done = make(chan bool, 1)
	t.ticker = time.NewTicker(d)

	go func(ticker *time.Ticker) {
		defer ticker.Stop()

		for {
			select {
			case <-t.done:
				return
			case <-ticker.C:
				atomic.AddInt32(&t.expireTimes, 1)

				if t.ExpireTimes() > t.MaxRetryTimes() {
					atomic.StoreInt32(&t.active, 0)
					cancelFunc()

					return
				}

				expiredFunc(t.ExpireTimes())
			}
		}
	}(t.ticker)

	return t
}

// MaxRetryTimes returns the configured retry limit.
func (t *Timer) MaxRetryTimes() int32 {
	return atomic.LoadInt32(&t.maxRetryTimes)
}

// ExpireTimes returns how many times the timer has expired so far.
func (t *Timer) ExpireTimes() int32 {
	return atomic.LoadInt32(&t.expireTimes)
}

// IsActive reports whether the timer has not been stopped.
func (t *Timer) IsActive() bool {
	return atomic.LoadInt32(&t.active) == 1
}

// Stop ends the timer; no further callback fires afterwards. Safe to call
// multiple times.
func (t *Timer) Stop() {
	t.stopOnce.Do(func() {
		atomic.StoreInt32(&t.active, 0)

		t.done <- true

		close(t.done)
	})
}
