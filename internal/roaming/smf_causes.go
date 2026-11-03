// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/nas/fgs"
)

var (
	ErrSessionHomeRouted      = errors.New("the home network allows no local breakout for this DNN")
	ErrSessionNotSubscribed   = errors.New("S-NSSAI and DNN not subscribed by the home network")
	ErrSessionNotLocalDN      = errors.New("not a local data network of the roaming partner")
	ErrSessionSSCMode         = errors.New("the home network allows no SSC mode 1 for this DNN")
	ErrSessionNotIP           = errors.New("the home network allows no IP PDU session type for this DNN")
	ErrSessionHomeUnreachable = errors.New("the home UDM cannot be reached")
	ErrSessionHomeRefused     = errors.New("the home UDM refused the request")
	ErrSessionHomeData        = errors.New("the home UDM returned unusable subscription data")
)

type SessionError struct {
	Cause fgs.GSMCause
	Err   error
}

func (e *SessionError) Error() string {
	return fmt.Sprintf("%v (5GSM cause #%d)", e.Err, uint8(e.Cause))
}

func (e *SessionError) Unwrap() error { return e.Err }

func (e *SessionError) GSMCause() fgs.GSMCause { return e.Cause }

func newSessionError(cause fgs.GSMCause, err error) error {
	return &SessionError{Cause: cause, Err: err}
}

func vsmfSDMError(pd wire.ProblemDetails) error {
	switch {
	case pd.Status == http.StatusNotFound && pd.Cause == wire.CauseUserNotFound:
		return newSessionError(fgs.GSMCauseUserAuthenticationOrAuthorizationFailed, fmt.Errorf("%w: %w", ErrSessionHomeRefused, pd))
	case pd.Status == http.StatusNotFound:
		return newSessionError(fgs.GSMCauseRequestedServiceOptionNotSubscribed, fmt.Errorf("%w: %w", ErrSessionNotSubscribed, pd))
	case pd.Status == http.StatusForbidden:
		return newSessionError(fgs.GSMCauseUserAuthenticationOrAuthorizationFailed, fmt.Errorf("%w: %w", ErrSessionHomeRefused, pd))
	case vsmfTransient(pd.Status):
		return newSessionError(fgs.GSMCauseNetworkFailure, fmt.Errorf("%w: %w", ErrSessionHomeUnreachable, pd))
	default:
		return newSessionError(fgs.GSMCauseRequestRejectedUnspecified, fmt.Errorf("%w: %w", ErrSessionHomeRefused, pd))
	}
}

func vsmfUECMError(pd wire.ProblemDetails) error {
	switch {
	case pd.Status == http.StatusForbidden && pd.Cause == wire.CauseDNNNotAllowed:
		return newSessionError(fgs.GSMCauseMissingOrUnknownDNN, fmt.Errorf("%w: %w", ErrSessionHomeRefused, pd))
	case pd.Status == http.StatusForbidden, pd.Status == http.StatusNotFound:
		return newSessionError(fgs.GSMCauseUserAuthenticationOrAuthorizationFailed, fmt.Errorf("%w: %w", ErrSessionHomeRefused, pd))
	case vsmfTransient(pd.Status):
		return fmt.Errorf("%w: %w", ErrSessionHomeUnreachable, pd)
	default:
		return newSessionError(fgs.GSMCauseRequestRejectedUnspecified, fmt.Errorf("%w: %w", ErrSessionHomeRefused, pd))
	}
}

func vsmfTransient(status int) bool {
	return status >= http.StatusInternalServerError || status == http.StatusTooManyRequests || status == http.StatusRequestTimeout
}

func vsmfUnreachable(err error) error {
	return newSessionError(fgs.GSMCauseNetworkFailure, fmt.Errorf("%w: %w", ErrSessionHomeUnreachable, err))
}
