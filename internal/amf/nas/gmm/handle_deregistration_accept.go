package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// TS 23.502 4.2.2.3
func handleDeregistrationAccept(ctx ctxt.Context, ue *context.AmfUe) error {
	logger.AmfLog.Debug("Handle Deregistration Accept", zap.String("supi", ue.Supi))

	ctx, span := tracer.Start(ctx, "AMF NAS HandleDeregistrationAccept")
	defer span.End()

	if ue.State.Current() != context.DeregistrationInitiated {
		return fmt.Errorf("state mismatch: receive Deregistration Accept message in state %s", ue.State.Current())
	}

	if ue.T3522 != nil {
		ue.T3522.Stop()
		ue.T3522 = nil // clear the timer
	}

	ue.SubscriptionDataValid = false
	ue.State.Set(context.Deregistered)

	if ue.RanUe != nil {
		err := ngap_message.SendUEContextReleaseCommand(ctx, ue.RanUe, context.UeContextReleaseDueToNwInitiatedDeregistraion, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
	}

	return nil
}
