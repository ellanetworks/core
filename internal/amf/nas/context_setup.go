// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
)

func contextSetup(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nasMessage.RegistrationRequest) {
	ctx, span := gmmTracer.Start(ctx, "nas/context_setup")
	defer span.End()

	ue.AdvanceRegStep(amf.RegStepContextSetup)

	conn := ue.Conn()
	if conn == nil {
		logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
		return
	}

	conn.RegistrationRequest = msg

	switch conn.RegistrationType5GS {
	case nasMessage.RegistrationType5GSInitialRegistration:
		HandleInitialRegistration(ctx, amfInstance, ue)
	case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
		fallthrough
	case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
		HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfInstance, ue)
	}
}
