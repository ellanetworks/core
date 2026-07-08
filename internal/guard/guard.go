// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package guard provides the retransmission/supervision timer shared by the 4G
// MME, 5G AMF and SMF: a single timer whose callbacks are invalidated the instant
// it is stopped or re-armed, so a firing that races a Stop/Arm becomes a no-op.
//
// It is built on time.AfterFunc and adds the generation-counter staleness that a
// bare timer lacks (a final firing can race a stop). The retransmit and abort
// actions are caller-supplied closures, so the same guard serves repeating
// retransmissions (Arm — the MME/AMF NAS common-procedure guards, SMF T3591/T3592)
// and one-shot supervision/idle timers (ArmOnce — handover guard, mobile
// reachable, implicit detach).
package guard

import (
	"sync"
	"time"
)

// TimerValue configures a retransmitting supervision timer that feeds Guard.Arm:
// ExpireTime is the per-attempt deadline and MaxRetryTimes the retransmit budget.
// Enable gates the timer — a disabled timer is never armed. Both the AMF and MME
// hold their retransmitting guard configs (NAS-common, ESM, paging) as TimerValues.
type TimerValue struct {
	Enable        bool
	ExpireTime    time.Duration
	MaxRetryTimes int32
}

// Guard is the zero-value-ready timer. Methods are safe for concurrent use;
// callback closures run without the Guard's lock held, so they may acquire other
// locks freely.
type Guard struct {
	mu      sync.Mutex
	t       *time.Timer
	gen     uint64
	expires int32 // firings so far in the current arming; status/diagnostics only
	maxRetx int32 // retry limit of the current arming; status/diagnostics only
}

// ArmWith arms the guard from cfg, a no-op when cfg is disabled — so callers need
// not repeat the Enable check or unpack ExpireTime/MaxRetryTimes.
func (g *Guard) ArmWith(cfg TimerValue, onRetransmit func(attempt int32), onAbort func()) {
	if !cfg.Enable {
		return
	}

	g.Arm(cfg.ExpireTime, cfg.MaxRetryTimes, onRetransmit, onAbort)
}

// Arm cancels any running timer and starts a new one: onRetransmit fires on each
// interval d, up to maxRetransmit times, then onAbort fires once when the limit is
// exhausted. A callback whose firing races a later Arm or Stop does not run. After
// onAbort the Guard is idle.
func (g *Guard) Arm(d time.Duration, maxRetransmit int32, onRetransmit func(attempt int32), onAbort func()) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.stopLocked()
	g.gen++
	gen := g.gen
	g.expires = 0
	g.maxRetx = maxRetransmit

	var fire func()

	fire = func() {
		g.mu.Lock()

		if g.gen != gen {
			g.mu.Unlock()
			return
		}

		g.expires++
		count := g.expires
		over := count > maxRetransmit

		if over {
			g.t = nil
			g.gen++
		}

		g.mu.Unlock()

		if over {
			onAbort()
			return
		}

		onRetransmit(count)

		// Re-arm for the next interval unless a Stop/Arm intervened while
		// onRetransmit ran. Scheduling after the callback (not before) keeps
		// firings non-overlapping.
		g.mu.Lock()

		if g.gen == gen {
			g.t = time.AfterFunc(d, fire)
		}

		g.mu.Unlock()
	}

	g.t = time.AfterFunc(d, fire)
}

// ArmOnce cancels any running timer and starts a one-shot: onFire runs once after
// d. Like Arm, a firing that races a later ArmOnce/Arm/Stop does not run. Use for
// supervision, deadline and idle-mode timers (no retransmission).
func (g *Guard) ArmOnce(d time.Duration, onFire func()) {
	g.Arm(d, 0, func(int32) {}, onFire)
}

// Stop cancels the timer and invalidates any in-flight callback. Idempotent.
func (g *Guard) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.stopLocked()
	g.gen++
}

// Active reports whether a timer is currently armed.
func (g *Guard) Active() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.t != nil
}

// ExpireTimes reports how many times the armed timer has fired so far, or 0 when
// idle. For status/diagnostics only.
func (g *Guard) ExpireTimes() int32 {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.t == nil {
		return 0
	}

	return g.expires
}

// MaxRetryTimes reports the armed timer's retry limit, or 0 when idle. For
// status/diagnostics only.
func (g *Guard) MaxRetryTimes() int32 {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.t == nil {
		return 0
	}

	return g.maxRetx
}

func (g *Guard) stopLocked() {
	if g.t != nil {
		g.t.Stop()
		g.t = nil
	}
}
