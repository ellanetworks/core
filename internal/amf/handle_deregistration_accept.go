// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

// TS 23.502 4.2.2.3
func handleDeregistrationAccept(ctx context.Context, ue *UeContext) error {
	if conn := ue.NasConn(); conn != nil && conn.T3522 != nil {
		conn.T3522.Stop()
		conn.T3522 = nil
	}

	defer ue.Deregister(ctx)

	ranUe := ue.RanUe()
	if ranUe == nil {
		logger.WithTrace(ctx, logger.AmfLog).Warn("RanUe is nil, cannot send UE Context Release Command", logger.SUPI(ue.supi.String()))
		return nil
	}

	ranUe.ReleaseAction = UeContextReleaseDueToNwInitiatedDeregistraion

	err := ranUe.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}

	return nil
}
