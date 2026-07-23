// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handleRegistrationComplete(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext) nasreply.Disposition {
	if ue.RegStep() != amf.RegStepContextSetup {
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Registration Complete message outside context setup", zap.String("state", string(ue.State())))
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	ue.TransitionTo(amf.Registered)

	conn := ue.Conn()
	if conn == nil {
		logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
		return nasreply.Handled()
	}

	conn.StopNASGuard()

	// UE confirmed receipt of the new GUTI — free the old one (TS 24.501)
	amfInstance.CommitGUTIRealloc(ue)

	// Configuration update command delivers the operator network name (TS 24.501).
	amf.SendConfigurationUpdateCommand(ctx, amfInstance, ue, false)

	forPending := conn.RegistrationRequest.FOR == 1

	udsHasPending := conn.RegistrationRequest.UplinkDataStatus != nil

	hasActiveSessions := ue.HasActivePduSessions()

	shouldRelease := !forPending && !udsHasPending && !hasActiveSessions

	if shouldRelease {
		ueConn := ue.Conn()
		if ueConn == nil {
			logger.From(ctx, logger.AmfLog).Warn("ue is not connected to RAN")
			return nasreply.Handled()
		}

		ueConn.ReleaseAction = amf.UeContextN2NormalRelease

		ueConn.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
	}

	ue.ClearRegistrationRequestData()

	return nasreply.Handled()
}
