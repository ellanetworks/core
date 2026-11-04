// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"errors"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
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

	if datapathCapacityExhausted(err) {
		return eps.ESMCauseInsufficientResources
	}

	if pdnType, ok := errors.AsType[*models.PDNTypeError](err); ok {
		return pdnType.Cause
	}

	if errors.Is(err, models.ErrUnknownAPN) {
		return eps.ESMCauseMissingOrUnknownAPN
	}

	return eps.ESMCauseRequestRejectedUnspecified
}

type datapathCapacity interface {
	error
	DatapathCapacity() bool
}

func datapathCapacityExhausted(err error) bool {
	c, ok := errors.AsType[datapathCapacity](err)

	return ok && c.DatapathCapacity()
}

func capacityBackoff(ueConn *mme.UeConn, t eps.RequestType, err error) *nas.GPRSTimer3 {
	if t == eps.RequestTypeHandover || !datapathCapacityExhausted(err) {
		return nil
	}

	if t == eps.RequestTypeEmergency || t == eps.RequestTypeHandoverOfEmergencyBearerServices || ueConn.MayBeHighPriorityAccess() {
		return nil
	}

	timer, tErr := nas.GPRSTimer3FromDuration(mme.T3396CapacityBackoff)
	if tErr != nil {
		return nil
	}

	return &timer
}
