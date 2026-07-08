// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"slices"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// EMMState is the EPS Mobility Management state of a UE (TS 24.301 §5.1.3.2,
// TS 23.401). The ECM state is derived from whether the UE holds an
// S1-connection (ue.active).
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

// RegStep is the sub-phase within the attach procedure, meaningful only in
// EMM-REGISTERED-INITIATED. The 4G attach and 5G registration share the same
// common-procedure skeleton — identification, authentication, security mode
// control (TS 24.301 §5.4 / TS 24.501 §5.4).
type RegStep uint8

const (
	RegStepNone RegStep = iota
	RegStepAuthenticating
	RegStepSecurityMode
	RegStepContextSetup
)

func (s RegStep) String() string {
	switch s {
	case RegStepAuthenticating:
		return "authenticating"
	case RegStepSecurityMode:
		return "security-mode"
	case RegStepContextSetup:
		return "context-setup"
	default:
		return "none"
	}
}

// validEMMTransitions is the allowed EMM state graph (TS 24.301 §5.1.3.2). A
// network-initiated detach ("re-attach not required") is not superseded by a
// colliding attach — the attach is ignored (§5.5.2.3.4 case d) — so
// EMM-DEREGISTERED-INITIATED only completes to EMM-DEREGISTERED.
var validEMMTransitions = map[EMMState][]EMMState{
	EMMDeregistered:            {EMMRegistrationInitiated},
	EMMRegistrationInitiated:   {EMMRegistered, EMMDeregistered},
	EMMRegistered:              {EMMRegistrationInitiated, EMMDeregistrationInitiated, EMMDeregistered},
	EMMDeregistrationInitiated: {EMMDeregistered},
}

// transitionEMMLocked applies a validated EMM state change: an unexpected
// transition resets the UE to EMM-DEREGISTERED as a fail-safe, never advancing
// in a corrupt state. The caller holds ue.mu.
func (ue *UeContext) transitionEMMLocked(target EMMState) {
	from := ue.emmState
	if from == target {
		return
	}

	if slices.Contains(validEMMTransitions[from], target) {
		ue.emmState = target
	} else {
		logger.MmeLog.Error("invalid EMM state transition",
			zap.String("from", from.String()), zap.String("to", target.String()))

		ue.emmState = EMMDeregistered
	}

	// Reset the registration sub-phase to match the new state: entering
	// EMM-REGISTERED-INITIATED starts at the authentication exchange; any other
	// state carries no sub-phase (TS 24.301 §5.4).
	if ue.emmState == EMMRegistrationInitiated {
		ue.regStep = RegStepAuthenticating
	} else {
		ue.regStep = RegStepNone
	}
}

// RegStep returns the sub-phase within the attach procedure (meaningful only in
// EMM-REGISTERED-INITIATED).
func (ue *UeContext) RegStep() RegStep {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.regStep
}

// AdvanceRegStep moves the attach sub-phase forward while the UE is
// registration-initiated; a no-op in any other state.
func (ue *UeContext) AdvanceRegStep(step RegStep) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.emmState == EMMRegistrationInitiated {
		ue.regStep = step
	}
}

// EMMState returns the UE's EMM registration state, read under ue.mu. Reading it
// while EMM-REGISTERED carries the happens-before that lets the caller then read
// the UE's other registered data (the mutex acts as the publication barrier).
func (ue *UeContext) EMMState() EMMState {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.emmState
}

// TransitionTo moves the UE's EMM registration state through the validated
// transition graph under ue.mu (TS 24.301 §5.1.3.2); an unexpected transition
// fails safe to EMM-DEREGISTERED.
func (ue *UeContext) TransitionTo(s EMMState) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.transitionEMMLocked(s)
}
