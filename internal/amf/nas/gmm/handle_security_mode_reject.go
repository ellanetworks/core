package gmm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
)

func handleSecurityModeReject(ctx context.Context, ue *amf.AmfUe, msg *nasMessage.SecurityModeReject) error {
	if state := ue.GetState(); state != amf.SecurityMode {
		return fmt.Errorf("state mismatch: receive Security Mode Reject message in state %s", state)
	}

	defer ue.Deregister(ctx)

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	ue.Log.Error("UE rejected the security mode command, abort the ongoing procedure", logger.Cause(nasMessage.Cause5GMMToString(msg.GetCauseValue())), logger.SUPI(ue.Supi.String()))

	ue.SecurityContextAvailable = false
	ue.RanUe().ReleaseAction = amf.UeContextReleaseUeContext

	err := ue.RanUe().SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}

	return nil
}
