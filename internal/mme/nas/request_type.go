// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"errors"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
)

// requestTypeRefusal reports the ESM cause a request type this network does not
// serve draws, and whether it is refused at all (TS 24.301 §6.5.1.6).
//
// Ella Core provides no emergency services, so the two emergency types and RLOS
// are refused rather than served as ordinary connections — establishing one
// would give the UE a connection with none of the emergency handling it asked
// for. "Handover of emergency bearer services" draws #54: §6.5.1.6 e) has the
// MME reject it when it has no emergency PDN connection for the UE, and it never
// has one.
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

// transferRejectCause maps a refused move onto the ESM cause the UE is told.
//
// #54 "PDN connection does not exist" says the network has no information about
// the connection, on which the UE establishes a new one instead of retrying
// (TS 24.301 §6.5.1.6 b). It would be untrue for a session the anchor does hold
// but cannot move as asked — a mismatched data network, a move already in
// flight, a session being released — so those draw #26 "insufficient resources",
// which is retryable and is what the 5GS side reports for the same case.
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

// attachBearerRejectCause is the ESM cause a failed default-bearer setup draws.
// A refused move is reported as such, so the UE knows to establish rather than
// retry; anything else keeps the generic #31.
func attachBearerRejectCause(t eps.RequestType, err error) eps.ESMCause {
	if t == eps.RequestTypeHandover {
		return transferRejectCause(err)
	}

	return eps.ESMCauseRequestRejectedUnspecified
}
