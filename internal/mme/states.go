// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"slices"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// EMMState is the EPS Mobility Management state of a UE (TS 24.301 §5.1.3.2,
// TS 23.401). The ECM state is derived from whether the UE holds an
// S1-connection (ue.s1).
type EMMState uint8

const (
	EMMDeregistered EMMState = iota
	EMMRegistrationInitiated
	EMMRegistered
	EMMDeregistrationInitiated
)

func (s EMMState) String() string {
	switch s {
	case EMMDeregistered:
		return "EMM-DEREGISTERED"
	case EMMRegistrationInitiated:
		return "EMM-REGISTERED-INITIATED"
	case EMMRegistered:
		return "EMM-REGISTERED"
	case EMMDeregistrationInitiated:
		return "EMM-DEREGISTERED-INITIATED"
	default:
		return "EMM-UNKNOWN"
	}
}

// validEMMTransitions is the allowed EMM state graph (TS 24.301 §5.1.3.2). An
// attach supersedes an in-progress detach, so a re-attach may start from
// EMM-DEREGISTERED-INITIATED.
var validEMMTransitions = map[EMMState][]EMMState{
	EMMDeregistered:            {EMMRegistrationInitiated},
	EMMRegistrationInitiated:   {EMMRegistered, EMMDeregistered},
	EMMRegistered:              {EMMRegistrationInitiated, EMMDeregistrationInitiated, EMMDeregistered},
	EMMDeregistrationInitiated: {EMMRegistrationInitiated, EMMDeregistered},
}

// emmStateAtomic holds a UE's EMMState; see the MME concurrency model. Reads are
// lock-free; writes are serialised per UE by the NAS goroutine.
type emmStateAtomic struct{ v atomic.Uint32 }

func (a *emmStateAtomic) load() EMMState   { return EMMState(a.v.Load()) }
func (a *emmStateAtomic) store(s EMMState) { a.v.Store(uint32(s)) }

// transition applies a validated EMM state change: an unexpected transition
// resets the UE to EMM-DEREGISTERED as a fail-safe rather than proceeding in a
// corrupt state (mirrors the 5GMM state machine).
func (a *emmStateAtomic) transition(target EMMState) {
	from := a.load()
	if from == target {
		return
	}

	if slices.Contains(validEMMTransitions[from], target) {
		a.store(target)
		return
	}

	logger.MmeLog.Error("invalid EMM state transition",
		zap.String("from", from.String()), zap.String("to", target.String()))

	a.store(EMMDeregistered)
}
