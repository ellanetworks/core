// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"context"
	"time"
)

const (
	cleanupRetryInitial = 10 * time.Second
	cleanupRetryMax     = 5 * time.Minute
)

type cleanupReaper interface {
	ReapDeletedSessions(ctx context.Context) (cleaned []uint64, pending int)
}

type cleanupReport interface {
	UserPlaneSessionCleaned(ctx context.Context, seid uint64)
	pendingCleanup(pending int)
}

func nextCleanupDelay(delay time.Duration, cleaned, pending int) time.Duration {
	if pending == 0 || cleaned > 0 {
		return cleanupRetryInitial
	}

	return min(2*delay, cleanupRetryMax)
}

func runCleanupReaper(ctx context.Context, reaper cleanupReaper, report cleanupReport) {
	delay := cleanupRetryInitial

	timer := time.NewTimer(delay)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		cleaned, pending := reapOnce(ctx, reaper, report)

		delay = nextCleanupDelay(delay, cleaned, pending)
		timer.Reset(delay)
	}
}

func reapOnce(ctx context.Context, reaper cleanupReaper, report cleanupReport) (cleaned, pending int) {
	seids, pending := reaper.ReapDeletedSessions(ctx)

	for _, seid := range seids {
		report.UserPlaneSessionCleaned(ctx, seid)
	}

	report.pendingCleanup(pending)

	return len(seids), pending
}

type upfCleanupReport struct{ u *UPF }

func (r upfCleanupReport) UserPlaneSessionCleaned(ctx context.Context, seid uint64) {
	if r.u.smf != nil {
		r.u.smf.UserPlaneSessionCleaned(ctx, seid)
	}
}

func (upfCleanupReport) pendingCleanup(pending int) {
	sessionsPendingCleanup.Set(float64(pending))
}

func (u *UPF) startCleanupReaper(ctx context.Context) {
	cctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	u.reaperCancel = cancel
	u.reaperDone = done

	go func() {
		defer close(done)

		runCleanupReaper(cctx, u.se, upfCleanupReport{u: u})
	}()
}

func (u *UPF) stopCleanupReaper() {
	if u.reaperCancel == nil {
		return
	}

	u.reaperCancel()
	<-u.reaperDone

	u.reaperCancel = nil
}
