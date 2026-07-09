// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handleSecurityModeReject(ctx context.Context, ue *amf.UeContext, msg *nasMessage.SecurityModeReject) nasreply.Disposition {
	if step := ue.RegStep(); step != amf.RegStepSecurityMode {
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Security Mode Reject message outside the security mode exchange", zap.String("state", string(ue.State())))
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	defer ue.Deregister(ctx)

	if conn := ue.Conn(); conn != nil {
		conn.StopNASGuard()
		conn.Parent().EndKeyChainProc(procedure.SecurityMode)
	}

	logger.From(ctx, logger.AmfLog).Error("UE rejected the security mode command, abort the ongoing procedure", logger.Cause(nasMessage.Cause5GMMToString(msg.GetCauseValue())), logger.SUPI(ue.Supi().String()))

	ue.ClearSecured()

	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.AmfLog).Warn("ue is not connected to RAN")
		return nasreply.Handled()
	}

	ueConn.ReleaseAction = amf.UeContextReleaseUeContext

	ueConn.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)

	return nasreply.Handled()
}
