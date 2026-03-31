package gmm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

// TS 23.502 4.2.2.3
func handleDeregistrationAccept(ctx context.Context, ue *amf.AmfUe) error {
	if ue.T3522 != nil {
		ue.T3522.Stop()
		ue.T3522 = nil // clear the timer
	}

	defer ue.Deregister(ctx)

	ranUe := ue.RanUe()
	if ranUe == nil {
		logger.WithTrace(ctx, logger.AmfLog).Warn("RanUe is nil, cannot send UE Context Release Command", logger.SUPI(ue.Supi.String()))
		return nil
	}

	ranUe.ReleaseAction = amf.UeContextReleaseDueToNwInitiatedDeregistraion

	err := ranUe.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}

	return nil
}
