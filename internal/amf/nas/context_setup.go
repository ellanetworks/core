// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/fgs"
)

func contextSetup(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *fgs.RegistrationRequest) {
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
	case fgs.RegistrationTypeInitial:
		HandleInitialRegistration(ctx, amfInstance, ue)
	case fgs.RegistrationTypeMobilityUpdating:
		fallthrough
	case fgs.RegistrationTypePeriodicUpdating:
		HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfInstance, ue)
	}
}
