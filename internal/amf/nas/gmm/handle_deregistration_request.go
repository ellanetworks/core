package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// TS 23.502 4.2.2.3
func handleDeregistrationRequestUEOriginatingDeregistration(ctx context.Context, amf *amfContext.AMF, ue *amfContext.AmfUe, msg *nasMessage.DeregistrationRequestUEOriginatingDeregistration) error {
	if ue.State != amfContext.Registered {
		return fmt.Errorf("state mismatch: receive Deregistration Request (UE Originating Deregistration) message in state %s", ue.State)
	}

	ue.State = amfContext.Deregistered

	for _, smContext := range ue.SmContextList {
		err := amf.Smf.ReleaseSmContext(ctx, smContext.Ref)
		if err != nil {
			ue.Log.Error("Release SmContext Error", zap.Error(err))
		}
	}

	if ue.RanUe == nil {
		logger.AmfLog.Warn("RanUe is nil, cannot send UE Context Release Command", zap.String("supi", ue.Supi))
		return nil
	}

	// if Deregistration type is not switch-off, send Deregistration Accept
	if msg.GetSwitchOff() == 0 {
		err := message.SendDeregistrationAccept(ctx, ue.RanUe)
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

	ue.RanUe.ReleaseAction = amfContext.UeContextReleaseUeContext

	err := ue.RanUe.Radio.NGAPSender.SendUEContextReleaseCommand(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}

	return nil
}
