// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

type HPLMNOperation string

const (
	HPLMNAuthenticate     HPLMNOperation = "nausf_authenticate"
	HPLMNConfirm          HPLMNOperation = "nausf_confirm"
	HPLMNRegister         HPLMNOperation = "nudm_uecm_registration"
	HPLMNSubscriptionData HPLMNOperation = "nudm_sdm_get"
)

type HPLMNFailure struct {
	Op  HPLMNOperation
	Err error
}

func (e *HPLMNFailure) Error() string {
	return string(e.Op) + ": " + e.Err.Error()
}

func (e *HPLMNFailure) Unwrap() error {
	return e.Err
}

func hplmnFailure(op HPLMNOperation, err error) error {
	if err == nil {
		return nil
	}

	var r *Rejection
	if errors.As(err, &r) {
		return err
	}

	return &HPLMNFailure{Op: op, Err: err}
}

type hplmnVerdict struct {
	cause      fgs.GMMCause
	authReject bool
}

type problemKey struct {
	status int
	cause  wire.Cause
}

var hplmnVerdicts = map[HPLMNOperation]map[problemKey]hplmnVerdict{
	HPLMNAuthenticate: {
		{http.StatusForbidden, wire.CauseServingNetworkNotAuthorized}:      {cause: fgs.GMMCausePLMNNotAllowed},
		{http.StatusForbidden, wire.CauseAuthenticationRejected}:           {authReject: true},
		{http.StatusForbidden, wire.CauseInvalidHNPublicKeyIdentifier}:     {authReject: true},
		{http.StatusForbidden, wire.CauseInvalidSchemeOutput}:              {authReject: true},
		{http.StatusNotFound, wire.CauseUserNotFound}:                      {authReject: true},
		{http.StatusNotImplemented, wire.CauseUnsupportedProtectionScheme}: {authReject: true},
	},
	HPLMNRegister: {
		{http.StatusForbidden, wire.CauseUnknown5GSSubscription}:         {cause: fgs.GMMCauseN1ModeNotAllowed},
		{http.StatusForbidden, wire.CauseAccessNotAllowed}:               {cause: fgs.GMMCauseNoSuitableCellsInTrackingArea},
		{http.StatusForbidden, wire.CauseRATNotAllowed}:                  {cause: fgs.GMMCauseNoSuitableCellsInTrackingArea},
		{http.StatusForbidden, wire.CauseNoPSSubscription}:               {cause: fgs.GMMCauseServicesNotAllowed},
		{http.StatusForbidden, wire.CauseRoamingNotAllowed}:              {cause: fgs.GMMCausePLMNNotAllowed},
		{http.StatusNotFound, wire.CauseContextNotFound}:                 {cause: fgs.GMMCauseUEIdentityCannotBeDerived},
		{http.StatusNotFound, wire.CauseUserNotFound}:                    {cause: fgs.GMMCauseIllegalUE},
		{http.StatusUnprocessableEntity, wire.CauseUnprocessableRequest}: {cause: fgs.GMMCauseProtocolErrorUnspecified},
	},
	HPLMNSubscriptionData: {
		{http.StatusNotFound, wire.CauseUserNotFound}: {cause: fgs.GMMCauseIllegalUE},
		{http.StatusNotFound, wire.CauseDataNotFound}: {cause: fgs.GMMCauseN1ModeNotAllowed},
	},
}

func hplmnRejection(op HPLMNOperation, err error) (*Rejection, bool) {
	var r *Rejection
	if errors.As(err, &r) {
		return r, true
	}

	var f *HPLMNFailure
	if errors.As(err, &f) {
		op = f.Op
	}

	pd, ok := roaming.ProblemOf(err)
	if !ok {
		return nil, false
	}

	v, ok := hplmnVerdicts[op][problemKey{pd.Status, pd.Cause}]
	if !ok {
		return nil, false
	}

	if v.authReject {
		return &Rejection{AuthReject: true, Reason: pd.Error()}, true
	}

	return reject(v.cause, pd.Error()), true
}

func AuthenticationRejection(err error) (*Rejection, bool) {
	switch {
	case errors.Is(err, roaming.ErrUnsupportedAuthType):
		return reject(fgs.GMMCauseN1ModeNotAllowed, err.Error()), true
	case errors.Is(err, ErrHPLMNSUPIMismatch):
		return &Rejection{AuthReject: true, Reason: err.Error()}, true
	}

	return hplmnRejection(HPLMNAuthenticate, err)
}

func ConfirmationRejection(err error) (*Rejection, bool) {
	switch {
	case errors.Is(err, wire.ErrAuthenticationFailed):
		return reject(fgs.GMMCauseIllegalUE, "the home AUSF did not authenticate the UE"), true
	case errors.Is(err, ErrHPLMNSUPIMismatch):
		return &Rejection{AuthReject: true, Reason: err.Error()}, true
	}

	return hplmnRejection(HPLMNConfirm, err)
}

func HPLMNRegistrationRejection(err error) (*Rejection, bool) {
	return hplmnRejection(HPLMNRegister, err)
}

func IsReauthenticationRequired(err error) bool {
	pd, ok := roaming.ProblemOf(err)
	return ok && pd.Status == http.StatusForbidden && pd.Cause == wire.CauseReauthenticationRequired
}

var metricCause = regexp.MustCompile(`^[A-Z0-9_]{1,64}$`)

func (amf *AMF) noteHPLMNFailure(ctx context.Context, v roaming.Visitor5G, op HPLMNOperation, err error, mapping func(error) (*Rejection, bool)) {
	if err == nil {
		return
	}

	if _, permanent := mapping(err); permanent || IsReauthenticationRequired(err) {
		return
	}

	status, cause := "none", "error"

	switch pd, ok := roaming.ProblemOf(err); {
	case ok:
		status, cause = strconv.Itoa(pd.Status), "none"
		if metricCause.MatchString(string(pd.Cause)) {
			cause = string(pd.Cause)
		} else if pd.Cause != "" {
			cause = "other"
		}
	case errors.Is(err, roaming.ErrHPLMNUnauthorized):
		cause = "unauthorized"
	case errors.Is(err, roaming.ErrHPLMNUnreachable):
		cause = "unreachable"
	case errors.Is(err, roaming.ErrHPLMNAnswer):
		cause = "malformed_answer"
	}

	roaming.RecordHPLMNTransient(v.PartnerName, string(op), status, cause)

	logger.From(ctx, logger.AmfLog).Warn("the home network failed without a subscriber verdict; treating it as transient",
		zap.String("roaming_partner", v.PartnerName), zap.String("operation", string(op)), zap.Error(err))
}
