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

	// The 5GMM capability is not a cleartext IE (TS 24.501 §4.4.6): a UE with no
	// valid 5G NAS security context sends it in the NAS message container of
	// SECURITY MODE COMPLETE.
	if msg != nil {
		ue.SetGMMCapability(msg.GMMCapability)
	}

	dispatchRegistration(ctx, amfInstance, ue, conn)
}

// dispatchRegistration runs the procedure the 5GS registration type selects. A
// UE performing an inter-system change from S1 mode without N26 registers with
// type "mobility registration update", and the AMF treats the request as an
// initial registration (TS 23.502 §4.11.2.3 step 3, TS 24.501 §5.5.1.3.4).
func dispatchRegistration(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, conn *amf.UeConn) {
	switch conn.RegistrationType5GS {
	case fgs.RegistrationTypeInitial:
		HandleInitialRegistration(ctx, amfInstance, ue)
	case fgs.RegistrationTypeMobilityUpdating:
		if movingFromEPC(conn.RegistrationRequest) {
			HandleInitialRegistration(ctx, amfInstance, ue)
			return
		}

		HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfInstance, ue)
	case fgs.RegistrationTypePeriodicUpdating:
		HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfInstance, ue)
	}
}
