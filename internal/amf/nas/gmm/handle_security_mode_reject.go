package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handleSecurityModeReject(ctx context.Context, ue *amfContext.AmfUe, msg *nas.GmmMessage) error {
	logger.AmfLog.Debug("Handle Security Mode Reject", zap.String("supi", ue.Supi))

	if ue.State != amfContext.SecurityMode {
		return fmt.Errorf("state mismatch: receive Security Mode Reject message in state %s", ue.State)
	}

	ue.State = amfContext.Deregistered

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	cause := msg.SecurityModeReject.GetCauseValue()

	ue.Log.Error("UE rejected the security mode command, abort the ongoing procedure", zap.String("Cause", nasMessage.Cause5GMMToString(cause)), zap.String("supi", ue.Supi))

	ue.SecurityContextAvailable = false
	ue.RanUe.ReleaseAction = amfContext.UeContextReleaseUeContext

	err := ue.RanUe.Radio.NGAPSender.SendUEContextReleaseCommand(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}

	return nil
}
