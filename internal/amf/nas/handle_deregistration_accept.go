// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/free5gc/ngap/ngapType"
)

// TS 23.502
func handleDeregistrationAccept(ctx context.Context, ue *amf.UeContext) nasreply.Disposition {
	if conn := ue.Conn(); conn != nil {
		conn.StopNASGuard()
	}

	defer ue.Deregister(ctx)

	ueConn := ue.Conn()
	if ueConn == nil {
		logger.WithTrace(ctx, logger.AmfLog).Warn("amf.UeConn is nil, cannot send UE Context Release Command", logger.SUPI(ue.Supi().String()))
		return nasreply.Handled()
	}

	ueConn.ReleaseAction = amf.UeContextReleaseDueToNwInitiatedDeregistraion

	ueConn.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)

	return nasreply.Handled()
}
