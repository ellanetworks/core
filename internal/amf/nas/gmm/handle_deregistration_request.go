package gmm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
)

// TS 23.502 4.2.2.3
func handleDeregistrationRequestUEOriginatingDeregistration(ctx context.Context, ue *amf.AmfUe, msg *nasMessage.DeregistrationRequestUEOriginatingDeregistration) error {
	if state := ue.GetState(); state != amf.Registered {
		return fmt.Errorf("state mismatch: receive Deregistration Request (UE Originating Deregistration) message in state %s", state)
	}

	defer ue.Deregister(ctx)

	if ue.RanUe() == nil {
		logger.WithTrace(ctx, logger.AmfLog).Warn("RanUe is nil, cannot send UE Context Release Command", logger.SUPI(ue.Supi.String()))
		return nil
	}

	// if Deregistration type is not switch-off, send Deregistration Accept
	if msg.GetSwitchOff() == 0 {
		err := message.SendDeregistrationAccept(ctx, ue.RanUe())
		if err != nil {
			return fmt.Errorf("error sending deregistration accept: %v", err)
		}

		ue.Log.Info("sent deregistration accept")
	}

	// TS 23.502 4.2.6, 4.12.3
	targetDeregistrationAccessType := msg.GetAccessType()
	if targetDeregistrationAccessType != nasMessage.AccessType3GPP {
		return nil
	}

	ue.RanUe().ReleaseAction = amf.UeContextReleaseUeContext

	err := ue.RanUe().SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}

	return nil
}
