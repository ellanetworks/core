package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handleSecurityModeReject(ctx context.Context, ue *amfContext.AmfUe, msg *nasMessage.SecurityModeReject) error {
	if ue.State != amfContext.SecurityMode {
		return fmt.Errorf("state mismatch: receive Security Mode Reject message in state %s", ue.State)
	}

	defer ue.Deregister()

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	ue.Log.Error("UE rejected the security mode command, abort the ongoing procedure", zap.String("Cause", nasMessage.Cause5GMMToString(msg.GetCauseValue())), zap.String("supi", ue.Supi))

	ue.SecurityContextAvailable = false
	ue.RanUe.ReleaseAction = amfContext.UeContextReleaseUeContext

	err := ue.RanUe.Radio.NGAPSender.SendUEContextReleaseCommand(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}

	return nil
}
