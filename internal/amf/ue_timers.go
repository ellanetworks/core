// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
)

// mobileReachableMargin is added to the periodic registration timer (T3512) to form
// the mobile reachable timer: TS 24.501 §5.3.7 — "by default, the mobile reachable
// timer is 4 minutes greater than the value of timer T3512".
const mobileReachableMargin = 4 * time.Minute

// implicitDeregistrationTime is the delay from mobile-reachable-timer expiry to
// implicit de-registration of an unreachable UE (TS 24.501 §5.3.7).
const implicitDeregistrationTime = 2 * time.Minute

// Idle-mode supervision (TS 24.501 §5.3.7): when the UE goes CM-IDLE the AMF arms
// the mobile reachable timer, which on expiry escalates to implicit
// deregistration. The timers and their generation counter are guarded by the
// registry lock (AMF.mu), since they track connection presence.

// StartMobileReachable arms the mobile reachable timer when the UE moves to
// CM-IDLE. A fresh idle period cancels any prior timer and its in-flight callback.
func (a *AMF) StartMobileReachable(ue *UeContext) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stopIdleTimersLocked(ue)
	gen := ue.idleGen

	ue.mobileReachableTimer.ArmOnce(a.T3512Value+mobileReachableMargin, func() {
		a.onMobileReachableExpiry(ue, gen)
	})
}

// stopIdleTimers ends idle-mode supervision for a caller not holding a.mu (attach paths
// use stopIdleTimersLocked under the lock).
func (a *AMF) stopIdleTimers(ue *UeContext) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stopIdleTimersLocked(ue)
}

// stopIdleTimersLocked cancels both idle timers and bumps idleGen so a callback
// that already fired becomes a no-op. The caller holds AMF.mu.
func (a *AMF) stopIdleTimersLocked(ue *UeContext) {
	ue.idleGen++
	ue.mobileReachableTimer.Stop()
	ue.implicitDeregistrationTimer.Stop()
}

// onMobileReachableExpiry escalates to the implicit deregistration timer once the
// mobile reachable timer fires (TS 24.501). It no-ops if a reconnect bumped
// idleGen after this timer was armed.
func (a *AMF) onMobileReachableExpiry(ue *UeContext, gen uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if ue.idleGen != gen {
		return
	}

	logger.AmfLog.Debug("mobile reachable timer expired", logger.SUPI(ue.supi.String()))

	ue.implicitDeregistrationTimer.ArmOnce(implicitDeregistrationTime, func() {
		a.onImplicitDeregistrationExpiry(ue, gen)
	})
}

// onImplicitDeregistrationExpiry deregisters an unreachable UE (TS 24.501). It
// no-ops if a reconnect bumped idleGen after the implicit timer was armed; the
// deregister runs after releasing AMF.mu (it takes UeContext.Mutex).
func (a *AMF) onImplicitDeregistrationExpiry(ue *UeContext, gen uint64) {
	a.mu.Lock()

	if ue.idleGen != gen {
		a.mu.Unlock()
		return
	}

	a.stopIdleTimersLocked(ue)

	a.mu.Unlock()

	ue.Deregister(context.Background())
}
