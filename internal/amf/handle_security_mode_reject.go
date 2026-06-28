// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
)

func handleSecurityModeReject(ctx context.Context, ue *UeContext, msg *nasMessage.SecurityModeReject) error {
	if state := ue.GetState(); state != SecurityMode {
		return fmt.Errorf("state mismatch: receive Security Mode Reject message in state %s", state)
	}

	defer ue.Deregister(ctx)

	if conn := ue.NasConn(); conn != nil {
		if conn.T3560 != nil {
			conn.T3560.Stop()
			conn.T3560 = nil
		}

		conn.Procedures.End(procedure.SecurityMode)
	}

	ue.Log.Error("UE rejected the security mode command, abort the ongoing procedure", logger.Cause(nasMessage.Cause5GMMToString(msg.GetCauseValue())), logger.SUPI(ue.supi.String()))

	ue.SecurityContextAvailable = false

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	ranUe.ReleaseAction = UeContextReleaseUeContext

	err := ranUe.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}

	return nil
}
