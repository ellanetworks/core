// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// StartMobileReachable arms the mobile reachable timer when the UE moves to
// ECM-IDLE (TS 24.301): the MME supervises the UE's periodic tracking
// area updating, and on expiry escalates to the implicit detach timer. A fresh
// idle period restarts the timer, so any prior timer (and its in-flight
// callback) is cancelled first.
func (m *MME) StartMobileReachable(ue *UeContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopIdleTimersLocked(ue)
	gen := ue.idleGen

	ue.mobileReachableTimer.ArmOnce(m.mobileReachableTime, func() {
		m.onMobileReachableExpiry(ue, gen)
	})
}

// stopIdleTimersLocked cancels both idle timers and bumps idleGen so a callback
// that has already fired becomes a no-op. The caller holds m.mu.
func (m *MME) stopIdleTimersLocked(ue *UeContext) {
	ue.idleGen++
	ue.mobileReachableTimer.Stop()
	ue.implicitDetachTimer.Stop()
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

	logger.MmeLog.Debug("mobile reachable timer expired", zap.String("imsi", ue.imsiOrEmpty()))

	ue.implicitDetachTimer.ArmOnce(m.implicitDetachTime, func() {
		m.onImplicitDetachExpiry(ue, gen)
	})
}

// onImplicitDetachExpiry implicitly detaches an unreachable UE (TS 24.301): the EPS
// bearers are released locally (no NAS/S1AP signalling — the UE is in ECM-IDLE), but
// the EMM context is retained as a Deregistered husk with its native EPS security
// context and its IMSI/M-TMSI index. This lets a later re-attach with the native GUTI
// reuse the context and skip authentication (resolveAttachContext), keeping the
// native security context (TS 24.301 §4.4.2 / annex C). A fresh attach for the same
// IMSI supersedes the husk (CommitUEIdentity), and subscriber deletion frees it
// (DetachSubscriber). The network/DB release runs without m.mu held.
func (m *MME) onImplicitDetachExpiry(ue *UeContext, gen uint64) {
	m.mu.Lock()

	if ue.idleGen != gen {
		m.mu.Unlock()
		return
	}

	m.stopIdleTimersLocked(ue)
	ue.TransitionTo(EMMDeregistered)
	imsi := ue.imsiOrEmpty()

	m.mu.Unlock()

	logger.MmeLog.Info("implicit detach: UE unreachable, deregistering (native security context retained)",
		zap.String("imsi", imsi))

	// A timer callback has no request context; the teardown must complete regardless,
	// so it runs on context.Background().
	m.ReleaseAllSessions(context.Background(), ue)
}
