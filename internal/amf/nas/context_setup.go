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

	if msg != nil {
		if msg.UpdateType5GS != nil && msg.UpdateType5GS.NGRANRCU {
			ue.RadioCapability = nil
			ue.RadioCapabilityForPaging = nil
		}
	}

	switch conn.RegistrationType5GS {
	case fgs.RegistrationTypeInitial:
		HandleInitialRegistration(ctx, amfInstance, ue)
	case fgs.RegistrationTypeMobilityUpdating:
		if movingFromEPC(msg) && !ue.TakeArrivedFromEPSHandover() {
			HandleInitialRegistration(ctx, amfInstance, ue)
			return
		}

		HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfInstance, ue)
	case fgs.RegistrationTypePeriodicUpdating:
		HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfInstance, ue)
	}
}
