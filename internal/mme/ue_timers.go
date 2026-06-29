// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// startMobileReachable arms the mobile reachable timer when the UE moves to
// ECM-IDLE (TS 24.301): the MME supervises the UE's periodic tracking
// area updating, and on expiry escalates to the implicit detach timer. A fresh
// idle period restarts the timer, so any prior timer (and its in-flight
// callback) is cancelled first.
func (m *MME) startMobileReachable(ue *UeContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopIdleTimersLocked(ue)
	gen := ue.idleGen

	ue.mobileReachableTimer = time.AfterFunc(m.mobileReachableTime, func() {
		m.onMobileReachableExpiry(ue, gen)
	})
}

// stopIdleTimersLocked cancels both idle timers and bumps idleGen so a callback
// that has already fired becomes a no-op. The caller holds m.mu.
func (m *MME) stopIdleTimersLocked(ue *UeContext) {
	ue.idleGen++

	if ue.mobileReachableTimer != nil {
		ue.mobileReachableTimer.Stop()
		ue.mobileReachableTimer = nil
	}

	if ue.implicitDetachTimer != nil {
		ue.implicitDetachTimer.Stop()
		ue.implicitDetachTimer = nil
	}
}

// onMobileReachableExpiry escalates to the implicit detach timer (TS 24.301).
// The MME has no recurring paging loop to stop on first expiry, so it
// arms the implicit detach timer directly.
func (m *MME) onMobileReachableExpiry(ue *UeContext, gen uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.idleGen != gen {
		return
	}

	ue.mobileReachableTimer = nil

	logger.MmeLog.Debug("mobile reachable timer expired", zap.String("imsi", ue.imsi))

	ue.implicitDetachTimer = time.AfterFunc(m.implicitDetachTime, func() {
		m.onImplicitDetachExpiry(ue, gen)
	})
}

// onImplicitDetachExpiry implicitly detaches an unreachable UE (TS 24.301):
// the EPS bearers and UE context are released locally, with no NAS or
// S1AP signalling since the UE is in ECM-IDLE. The network/DB release runs
// without m.mu held.
func (m *MME) onImplicitDetachExpiry(ue *UeContext, gen uint64) {
	m.mu.Lock()

	if ue.idleGen != gen {
		m.mu.Unlock()
		return
	}

	ue.implicitDetachTimer = nil
	ue.emmState.store(EMMDeregistered)
	imsi := ue.imsi

	m.mu.Unlock()

	logger.MmeLog.Info("implicit detach: UE unreachable, releasing context",
		zap.String("imsi", imsi))

	m.ReleaseAllSessions(ue)
	m.RemoveUe(ue)
}
