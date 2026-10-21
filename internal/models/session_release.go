// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

type SessionReleaseReason string

const (
	ReleaseUERequested            SessionReleaseReason = "ue_requested"
	ReleaseDeregistration         SessionReleaseReason = "deregistration"
	ReleaseImplicitDeregistration SessionReleaseReason = "implicit_deregistration"
	ReleaseRegistrationFailure    SessionReleaseReason = "registration_failure"
	ReleaseSubscriptionWithdrawn  SessionReleaseReason = "subscription_withdrawn"
	ReleaseSubscriptionChanged    SessionReleaseReason = "subscription_changed"
	ReleasePolicyChanged          SessionReleaseReason = "policy_changed"
	ReleaseStatusMismatch         SessionReleaseReason = "status_mismatch"
	ReleaseDuplicateSessionID     SessionReleaseReason = "duplicate_session_id"
	ReleaseHandoverFailure        SessionReleaseReason = "handover_failure"
	ReleaseNotTransferred         SessionReleaseReason = "not_transferred"
	ReleaseUENotResponding        SessionReleaseReason = "ue_not_responding"
	ReleaseProtocolError          SessionReleaseReason = "protocol_error"
	ReleaseEstablishmentFailure   SessionReleaseReason = "establishment_failure"
	ReleaseAccessReleased         SessionReleaseReason = "access_released"
	ReleaseNetworkFailure         SessionReleaseReason = "network_failure"
)

var SessionReleaseReasons = []SessionReleaseReason{
	ReleaseUERequested,
	ReleaseDeregistration,
	ReleaseImplicitDeregistration,
	ReleaseRegistrationFailure,
	ReleaseSubscriptionWithdrawn,
	ReleaseSubscriptionChanged,
	ReleasePolicyChanged,
	ReleaseStatusMismatch,
	ReleaseDuplicateSessionID,
	ReleaseHandoverFailure,
	ReleaseNotTransferred,
	ReleaseUENotResponding,
	ReleaseProtocolError,
	ReleaseEstablishmentFailure,
	ReleaseAccessReleased,
	ReleaseNetworkFailure,
}

func (r SessionReleaseReason) String() string { return string(r) }

type ChargingTerminationTrigger string

const (
	ChargingTriggerFinal                  ChargingTerminationTrigger = "FINAL"
	ChargingTriggerAbnormalRelease        ChargingTerminationTrigger = "ABNORMAL_RELEASE"
	ChargingTriggerManagementIntervention ChargingTerminationTrigger = "MANAGEMENT_INTERVENTION"
)

func (r SessionReleaseReason) ChargingTrigger() ChargingTerminationTrigger {
	switch r {
	case ReleaseUERequested, ReleaseDeregistration, ReleaseImplicitDeregistration, ReleaseStatusMismatch, ReleaseDuplicateSessionID:
		return ChargingTriggerFinal
	case ReleaseSubscriptionWithdrawn, ReleaseSubscriptionChanged, ReleasePolicyChanged:
		return ChargingTriggerManagementIntervention
	default:
		return ChargingTriggerAbnormalRelease
	}
}

type RecordClosingCause uint8

const (
	RecordClosingNormalRelease          RecordClosingCause = 0
	RecordClosingAbnormalRelease        RecordClosingCause = 4
	RecordClosingManagementIntervention RecordClosingCause = 20
)

func (t ChargingTerminationTrigger) RecordClosing() RecordClosingCause {
	switch t {
	case ChargingTriggerFinal:
		return RecordClosingNormalRelease
	case ChargingTriggerManagementIntervention:
		return RecordClosingManagementIntervention
	default:
		return RecordClosingAbnormalRelease
	}
}

type NgApCause struct {
	Group uint8
	Value uint16
}

type EPSCauseProtocol uint8

const (
	EPSCauseS1AP     EPSCauseProtocol = 1
	EPSCauseEMM      EPSCauseProtocol = 2
	EPSCauseESM      EPSCauseProtocol = 3
	EPSCauseDiameter EPSCauseProtocol = 4
	EPSCauseIKEv2    EPSCauseProtocol = 5
)

type EPSRanNasCause struct {
	Protocol  EPSCauseProtocol
	CauseType uint8
	Value     uint16
}

func (c EPSRanNasCause) Encode() []byte {
	head := byte(c.Protocol)<<4 | c.CauseType&0x0f

	if c.Protocol == EPSCauseDiameter || c.Protocol == EPSCauseIKEv2 {
		return []byte{head, byte(c.Value >> 8), byte(c.Value)}
	}

	return []byte{head, byte(c.Value)}
}

type RanNasRelCause struct {
	NgAp    *NgApCause
	FiveGMm *uint8
	FiveGSm *uint8
	EPS     *EPSRanNasCause
}

type SessionReleaseRequest struct {
	Reason     SessionReleaseReason
	SentCause  *uint8
	RanNas     []RanNasRelCause
	UeLocation *UserLocation
}

func ReleaseFor(reason SessionReleaseReason) SessionReleaseRequest {
	return SessionReleaseRequest{Reason: reason}
}

func (r SessionReleaseRequest) WithRanNas(cause RanNasRelCause) SessionReleaseRequest {
	r.RanNas = append(append([]RanNasRelCause(nil), r.RanNas...), cause)

	return r
}

func (r SessionReleaseRequest) WithSentCause(cause uint8) SessionReleaseRequest {
	r.SentCause = &cause

	return r
}

func (r SessionReleaseRequest) WithLocation(loc UserLocation) SessionReleaseRequest {
	if loc.EutraLocation == nil && loc.NrLocation == nil && loc.N3gaLocation == nil {
		return r
	}

	r.UeLocation = &loc

	return r
}

func ReceivedNGAPCause(group uint8, value uint16) RanNasRelCause {
	return RanNasRelCause{NgAp: &NgApCause{Group: group, Value: value}}
}

func Received5GSMCause(value uint8) RanNasRelCause {
	return RanNasRelCause{FiveGSm: &value}
}

func Received5GMMCause(value uint8) RanNasRelCause {
	return RanNasRelCause{FiveGMm: &value}
}

func ReceivedS1APCause(causeType uint8, value uint16) RanNasRelCause {
	return RanNasRelCause{EPS: &EPSRanNasCause{Protocol: EPSCauseS1AP, CauseType: causeType, Value: value}}
}

func ReceivedESMCause(value uint8) RanNasRelCause {
	return RanNasRelCause{EPS: &EPSRanNasCause{Protocol: EPSCauseESM, Value: uint16(value)}}
}
