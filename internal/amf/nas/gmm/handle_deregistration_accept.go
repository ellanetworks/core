package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// TS 23.502 4.2.2.3
func handleDeregistrationAccept(ctx context.Context, ue *amfContext.AmfUe) error {
	logger.AmfLog.Debug("Handle Deregistration Accept", zap.String("supi", ue.Supi))

	if ue.T3522 != nil {
		ue.T3522.Stop()
		ue.T3522 = nil // clear the timer
	}

	ue.State = amfContext.Deregistered

	if ue.RanUe == nil {
		logger.AmfLog.Warn("RanUe is nil, cannot send UE Context Release Command", zap.String("supi", ue.Supi))
		return nil
	}

	ue.RanUe.ReleaseAction = amfContext.UeContextReleaseDueToNwInitiatedDeregistraion

	err := ue.RanUe.Radio.NGAPSender.SendUEContextReleaseCommand(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}

	return nil
}
