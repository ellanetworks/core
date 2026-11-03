// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"sync"
	"time"
)

const (
	n32cRateRetryAfterS = 60
	n32fRateRetryAfterS = 1
)

type rateLimiter struct {
	mu      sync.Mutex
	window  time.Duration
	windows map[string]*rateWindow
}

type rateWindow struct {
	start time.Time
	count int
}

func (l *rateLimiter) init(window time.Duration) {
	l.window = window
	l.windows = map[string]*rateWindow{}
}

func (l *rateLimiter) allow(key string, now time.Time, limit int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	w, ok := l.windows[key]
	if !ok || now.Sub(w.start) >= l.window {
		for k, old := range l.windows {
			if now.Sub(old.start) >= l.window {
				delete(l.windows, k)
			}
		}

		w = &rateWindow{start: now}
		l.windows[key] = w
	}

	if w.count >= limit {
		return false
	}

	w.count++

	return true
}
