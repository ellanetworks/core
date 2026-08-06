// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/fgs"
)

// plain is the message msg was decoded from: it becomes the oracle a later
// retransmission is compared against, so once the security mode procedure has
// replayed the complete message in its NAS message container, that is what the
// AMF holds (TS 24.501 §4.4.6, §5.5.1.2.8 case d).
func contextSetup(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *fgs.RegistrationRequest, plain []byte) {
	ctx, span := gmmTracer.Start(ctx, "nas/context_setup")
	defer span.End()

	ue.AdvanceRegStep(amf.RegStepContextSetup)

	conn := ue.Conn()
	if conn == nil {
		logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
		return
	}

	conn.RegistrationRequest = msg
	conn.RegistrationRequestPlain = plain

	// The 5GMM capability is not a cleartext IE (TS 24.501 §4.4.6), so a UE with
	// no valid 5G NAS security context sends it only in the NAS message container
	// of SECURITY MODE COMPLETE. This is the first point at which the complete
	// message is known, for both of §4.4.6's cases. Its S1 mode bit decides
	// whether the UE is told how this network interworks with EPS.
	if msg != nil {
		ue.SetGMMCapability(msg.GMMCapability)
	}

	switch conn.RegistrationType5GS {
	case fgs.RegistrationTypeInitial:
		HandleInitialRegistration(ctx, amfInstance, ue)
	case fgs.RegistrationTypeMobilityUpdating:
		fallthrough
	case fgs.RegistrationTypePeriodicUpdating:
		HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfInstance, ue)
	}
}
