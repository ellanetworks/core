// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"slices"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type StateType uint8

const (
	Deregistered StateType = iota
	RegistrationInitiated
	Registered
	DeregistrationInitiated
)

// String returns the 5GMM state name. These literals are the exported API/JSON values
// (`gmm_state`) and conformance output, so they must not change. Mirrors the MME's
// EMMState.String() (with the 5G state names).
func (s StateType) String() string {
	switch s {
	case Deregistered:
		return "Deregistered"
	case RegistrationInitiated:
		return "RegistrationInitiated"
	case Registered:
		return "Registered"
	case DeregistrationInitiated:
		return "DeregistrationInitiated"
	default:
		return "Unknown"
	}
}

var validTransitions = map[StateType][]StateType{
	Deregistered:            {RegistrationInitiated},
	RegistrationInitiated:   {Registered, Deregistered},
	Registered:              {RegistrationInitiated, DeregistrationInitiated, Deregistered},
	DeregistrationInitiated: {Deregistered},
}

// RegStep is the phase within the single 5GMM state 5GMM-REGISTERED-INITIATED
// (TS 24.501 §5.1.3.2), meaningful only while the state is RegistrationInitiated.
// Ordering of the authentication, security mode, and context-setup exchanges is
// enforced against it.
type RegStep uint8

const (
	RegStepNone RegStep = iota
	RegStepAuthenticating
	RegStepSecurityMode
	RegStepContextSetup
)

func (ue *UeContext) State() StateType {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.state
}

func (ue *UeContext) TransitionTo(target StateType) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.transitionToLocked(target)
}

// transitionToLocked enforces allowed state transitions and must only be called while ue.Mutex is held.
func (ue *UeContext) transitionToLocked(target StateType) {
	if ue.state == target {
		return
	}

	if slices.Contains(validTransitions[ue.state], target) {
		logger.AmfLog.Debug("state transition",
			zap.String("from", ue.state.String()),
			zap.String("to", target.String()))

		ue.setStateLocked(target)

		return
	}

	logger.AmfLog.Error("invalid state transition",
		zap.String("from", ue.state.String()),
		zap.String("to", target.String()))

	ue.setStateLocked(Deregistered)
}

func (ue *UeContext) setStateLocked(target StateType) {
	ue.state = target

	if target == RegistrationInitiated {
		ue.regStep = RegStepAuthenticating
	} else {
		ue.regStep = RegStepNone
	}
}

// RegStep returns the phase within the registration procedure (meaningful only in
// RegistrationInitiated).
func (ue *UeContext) RegStep() RegStep {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.regStep
}

// AdvanceRegStep moves the registration sub-phase forward while the UE is
// registration-initiated; it is a no-op in any other state.
func (ue *UeContext) AdvanceRegStep(step RegStep) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.state == RegistrationInitiated {
		ue.regStep = step
	}
}
