// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

type SessionReleaseReason string

const (
	ReleaseUERequested           SessionReleaseReason = "ue_requested"
	ReleaseDeregistration        SessionReleaseReason = "deregistration"
	ReleaseSubscriptionWithdrawn SessionReleaseReason = "subscription_withdrawn"
	ReleaseReactivationRequested SessionReleaseReason = "reactivation_requested"
	ReleaseStatusMismatch        SessionReleaseReason = "status_mismatch"
	ReleaseDuplicateSessionID    SessionReleaseReason = "duplicate_session_id"
	ReleaseHandoverFailure       SessionReleaseReason = "handover_failure"
	ReleaseUENotResponding       SessionReleaseReason = "ue_not_responding"
	ReleaseProtocolError         SessionReleaseReason = "protocol_error"
	ReleaseEstablishmentFailure  SessionReleaseReason = "establishment_failure"
	ReleaseAccessReleased        SessionReleaseReason = "access_released"
)

var SessionReleaseReasons = []SessionReleaseReason{
	ReleaseUERequested,
	ReleaseDeregistration,
	ReleaseSubscriptionWithdrawn,
	ReleaseReactivationRequested,
	ReleaseStatusMismatch,
	ReleaseDuplicateSessionID,
	ReleaseHandoverFailure,
	ReleaseUENotResponding,
	ReleaseProtocolError,
	ReleaseEstablishmentFailure,
	ReleaseAccessReleased,
}

func (r SessionReleaseReason) String() string { return string(r) }

type RecordClosingCause uint8

const (
	RecordClosingNormalRelease          RecordClosingCause = 0
	RecordClosingAbnormalRelease        RecordClosingCause = 4
	RecordClosingManagementIntervention RecordClosingCause = 20
)

func (r SessionReleaseReason) RecordClosing() RecordClosingCause {
	switch r {
	case ReleaseUERequested, ReleaseDeregistration, ReleaseStatusMismatch, ReleaseDuplicateSessionID:
		return RecordClosingNormalRelease
	case ReleaseSubscriptionWithdrawn, ReleaseReactivationRequested:
		return RecordClosingManagementIntervention
	default:
		return RecordClosingAbnormalRelease
	}
}

type RanNasCauseProtocol string

const (
	RanNasNGAP RanNasCauseProtocol = "ngap"
	RanNas5GMM RanNasCauseProtocol = "5gmm"
	RanNas5GSM RanNasCauseProtocol = "5gsm"
	RanNasS1AP RanNasCauseProtocol = "s1ap"
	RanNasEMM  RanNasCauseProtocol = "emm"
	RanNasESM  RanNasCauseProtocol = "esm"
)

type RanNasCause struct {
	Protocol RanNasCauseProtocol
	Group    uint8
	Value    uint16
}

type SessionReleaseCause struct {
	Reason      SessionReleaseReason
	Diagnostics []RanNasCause
}

func ReleaseCause(reason SessionReleaseReason, diagnostics ...RanNasCause) SessionReleaseCause {
	return SessionReleaseCause{Reason: reason, Diagnostics: diagnostics}
}

func GSMCause(value uint8) RanNasCause {
	return RanNasCause{Protocol: RanNas5GSM, Value: uint16(value)}
}

func GMMCause(value uint8) RanNasCause {
	return RanNasCause{Protocol: RanNas5GMM, Value: uint16(value)}
}

func ESMCause(value uint8) RanNasCause {
	return RanNasCause{Protocol: RanNasESM, Value: uint16(value)}
}

func EMMCause(value uint8) RanNasCause {
	return RanNasCause{Protocol: RanNasEMM, Value: uint16(value)}
}

func NGAPCause(group uint8, value uint16) RanNasCause {
	return RanNasCause{Protocol: RanNasNGAP, Group: group, Value: value}
}

func S1APCause(group uint8, value uint16) RanNasCause {
	return RanNasCause{Protocol: RanNasS1AP, Group: group, Value: value}
}
