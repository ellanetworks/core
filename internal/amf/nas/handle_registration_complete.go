// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handleRegistrationComplete(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext) {
	if ue.RegStep() != amf.RegStepContextSetup {
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Registration Complete message outside context setup", zap.String("state", string(ue.State())))
		return
	}

	ue.TransitionTo(amf.Registered)

	conn := ue.Conn()
	if conn == nil {
		logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
		return
	}

	conn.StopNASGuard()

	// UE confirmed receipt of the new GUTI — free the old one (TS 24.501)
	amfInstance.CommitGUTIRealloc(ue)

	// Configuration update command delivers the operator network name (TS 24.501).
	amf.SendConfigurationUpdateCommand(ctx, amfInstance, ue, false)

	forPending := conn.RegistrationRequest.GetFOR() == nasMessage.FollowOnRequestPending

	udsHasPending := conn.RegistrationRequest.UplinkDataStatus != nil

	hasActiveSessions := ue.HasActivePduSessions()

	shouldRelease := !forPending && !udsHasPending && !hasActiveSessions

	if shouldRelease {
		ueConn := ue.Conn()
		if ueConn == nil {
			logger.From(ctx, logger.AmfLog).Warn("ue is not connected to RAN")
			return
		}

		ueConn.ReleaseAction = amf.UeContextN2NormalRelease

		err := ueConn.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("error sending ue context release command", zap.Error(err))
			return
		}
	}

	ue.ClearRegistrationRequestData()
}
