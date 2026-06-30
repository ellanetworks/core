// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

// TS 23.502
func handleDeregistrationAccept(ctx context.Context, ue *amf.UeContext) error {
	if conn := ue.NasConn(); conn != nil {
		conn.T3522.Stop()
	}

	defer ue.Deregister(ctx)

	ranUe := ue.RanUe()
	if ranUe == nil {
		logger.WithTrace(ctx, logger.AmfLog).Warn("amf.RanUe is nil, cannot send UE Context Release Command", logger.SUPI(ue.SupiValue().String()))
		return nil
	}

	ranUe.ReleaseAction = amf.UeContextReleaseDueToNwInitiatedDeregistraion

	err := ranUe.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}

	return nil
}
