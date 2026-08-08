// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"errors"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
)

func requestTypeRefusal(t eps.RequestType) (eps.ESMCause, bool) {
	switch t {
	case eps.RequestTypeHandoverOfEmergencyBearerServices:
		return eps.ESMCausePDNConnectionDoesNotExist, true
	case eps.RequestTypeEmergency, eps.RequestTypeRLOS:
		return eps.ESMCauseServiceOptionNotSupported, true
	default:
		return 0, false
	}
}

func transferRejectCause(err error) eps.ESMCause {
	switch {
	case errors.Is(err, models.ErrSessionNotTransferable):
		return eps.ESMCausePDNConnectionDoesNotExist
	case errors.Is(err, models.ErrSessionOnOtherDNN), errors.Is(err, models.ErrSessionNotMovable):
		return eps.ESMCauseInsufficientResources
	default:
		return eps.ESMCauseRequestRejectedUnspecified
	}
}

func attachBearerRejectCause(t eps.RequestType, err error) eps.ESMCause {
	if t == eps.RequestTypeHandover {
		return transferRejectCause(err)
	}

	// The anchor computed the cause, since it holds the pools the answer depends
	// on (TS 24.301 §6.5.1.4.1).
	var pdnType *models.PDNTypeError
	if errors.As(err, &pdnType) {
		return pdnType.Cause
	}

	return eps.ESMCauseRequestRejectedUnspecified
}
