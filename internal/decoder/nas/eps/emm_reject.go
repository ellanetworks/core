// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/hex"

	nasie "github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

type AttachReject struct {
	EMMCause     utils.EnumField `json:"emm_cause"`
	ESMContainer *ESMMessage     `json:"esm_container,omitempty"`
	T3402        *uint8          `json:"t3402,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildAttachReject(msg *eps.AttachReject) *AttachReject {
	out := &AttachReject{
		EMMCause:     emmCauseToEnum(msg.Cause),
		ESMContainer: decodeESMContainer(msg.ESMMessageContainer),
		T3402:        optionalTimer(msg.T3402),
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type AuthenticationFailure struct {
	EMMCause utils.EnumField `json:"emm_cause"`
	AUTS     string          `json:"auts,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildAuthenticationFailure(msg *eps.AuthenticationFailure) *AuthenticationFailure {
	out := &AuthenticationFailure{EMMCause: emmCauseToEnum(msg.Cause)}

	if len(msg.AUTS) > 0 {
		out.AUTS = hex.EncodeToString(msg.AUTS)
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type AuthenticationReject struct {
	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildAuthenticationReject(msg *eps.AuthenticationReject) *AuthenticationReject {
	return &AuthenticationReject{UnrecognizedIEs: utils.RawIEs(msg.Unrecognized)}
}

type DetachAccept struct {
	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildDetachAccept(msg *eps.DetachAccept) *DetachAccept {
	return &DetachAccept{UnrecognizedIEs: utils.RawIEs(msg.Unrecognized)}
}

type DetachRequestNetwork struct {
	DetachType utils.EnumField  `json:"detach_type"`
	EMMCause   *utils.EnumField `json:"emm_cause,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildDetachRequestNetwork(msg *eps.DetachRequestNetwork) *DetachRequestNetwork {
	out := &DetachRequestNetwork{DetachType: detachTypeNetworkToEnum(msg.TypeOfDetach)}

	if msg.Cause != nil {
		c := emmCauseToEnum(*msg.Cause)
		out.EMMCause = &c
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type EMMStatus struct {
	EMMCause utils.EnumField `json:"emm_cause"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildEMMStatus(msg *eps.EMMStatus) *EMMStatus {
	out := &EMMStatus{EMMCause: emmCauseToEnum(msg.Cause)}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type SecurityModeReject struct {
	EMMCause utils.EnumField `json:"emm_cause"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildSecurityModeReject(msg *eps.SecurityModeReject) *SecurityModeReject {
	out := &SecurityModeReject{EMMCause: emmCauseToEnum(msg.Cause)}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type ServiceAccept struct {
	EPSBearerContextStatus []nasie.EPSBearerContextStatusItem `json:"eps_bearer_context_status,omitempty"`
	T3448                  *uint8                             `json:"t3448,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildServiceAccept(msg *eps.ServiceAccept) *ServiceAccept {
	out := &ServiceAccept{
		EPSBearerContextStatus: nasie.EPSBearerContextStatus(msg.EPSBearerContextStatus),
		T3448:                  optionalTimer(msg.T3448),
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type ServiceReject struct {
	EMMCause utils.EnumField `json:"emm_cause"`
	T3442    *uint8          `json:"t3442,omitempty"`
	T3346    *uint8          `json:"t3346,omitempty"`
	T3448    *uint8          `json:"t3448,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildServiceReject(msg *eps.ServiceReject) *ServiceReject {
	out := &ServiceReject{
		EMMCause: emmCauseToEnum(msg.Cause),
		T3442:    optionalTimer(msg.T3442),
		T3346:    optionalTimer(msg.T3346),
		T3448:    optionalTimer(msg.T3448),
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type TrackingAreaUpdateComplete struct {
	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildTrackingAreaUpdateComplete(msg *eps.TrackingAreaUpdateComplete) *TrackingAreaUpdateComplete {
	return &TrackingAreaUpdateComplete{UnrecognizedIEs: utils.RawIEs(msg.Unrecognized)}
}

// optionalTimer renders a GPRS timer that the message may omit.
func optionalTimer(t *nas.GPRSTimer2) *uint8 {
	if t == nil {
		return nil
	}

	v := timerOctet(*t)

	return &v
}
