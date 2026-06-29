// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
)

func handleSecurityModeReject(ctx context.Context, ue *amf.UeContext, msg *nasMessage.SecurityModeReject) error {
	if state := ue.GetState(); state != amf.SecurityMode {
		return fmt.Errorf("state mismatch: receive Security Mode Reject message in state %s", state)
	}

	defer ue.Deregister(ctx)

	if conn := ue.NasConn(); conn != nil {
		conn.T3560.Stop()
		conn.Procedures.End(procedure.SecurityMode)
	}

	ue.Log.Error("UE rejected the security mode command, abort the ongoing procedure", logger.Cause(nasMessage.Cause5GMMToString(msg.GetCauseValue())), logger.SUPI(ue.SupiValue().String()))

	ue.ClearSecurityContext()

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	ranUe.ReleaseAction = amf.UeContextReleaseUeContext

	err := ranUe.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}

	return nil
}
